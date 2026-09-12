package privileged

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"syscall"
	"time"

	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	contentapi "github.com/containerd/containerd/api/services/content/v1"
	imagesapi "github.com/containerd/containerd/api/services/images/v1"
	leasesapi "github.com/containerd/containerd/api/services/leases/v1"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	tasksapi "github.com/containerd/containerd/api/services/tasks/v1"
	transferapi "github.com/containerd/containerd/api/services/transfer/v1"
	"github.com/containerd/containerd/api/types"
	tasktypes "github.com/containerd/containerd/api/types/task"
	transfertypes "github.com/containerd/containerd/api/types/transfer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

/*
The containerd API, spoken directly.

This uses the generated API in `containerd/api`, which was already a dependency
for the runtime probe, rather than containerd's client library. The client
library is large, and most of it — image resolution, unpacking, spec helpers
with hundreds of options — is either done server-side by containerd 2.x's
Transfer service or is exactly the "start from a full spec and remember to
clear things" shape `containerspec.go` avoids.

Everything happens in one containerd namespace, `guardian-decoy`. That is the
first isolation property of this file and the easiest to overlook: the helper
cannot list, inspect, stop, or delete a container it did not create, because
every request it sends is scoped to a namespace nothing else on the host uses.

A word on privilege. The helper's capability bounding set is CAP_NET_ADMIN, but
the containerd socket is a root-equivalent API: whoever can send it an
arbitrary spec can run anything. The helper's containment of decoys therefore
rests on what this code can express, which is only the output of
`BuildContainerSpec`. That is recorded in the security review rather than left
for someone to discover.
*/

const (
	containerdNamespace   = "guardian-decoy"
	containerdSnapshotter = "overlayfs"
	containerdRuntimeName = "io.containerd.runc.v2"

	// specTypeURL is the conventional type for an OCI runtime spec. containerd
	// writes the value verbatim into the bundle's config.json.
	specTypeURL = "types.containerd.io/opencontainers/runtime-spec/1/Spec"

	// Manifests and configs are small. A bound keeps a hostile or broken
	// registry from making a root process buffer an arbitrary blob.
	maxImageMetadataBytes = 4 << 20

	labelWorkload    = "guardian.workload"
	labelSpecDigest  = "guardian.spec-digest"
	labelImageDigest = "guardian.image-digest"

	rootfsSnapshotPrefix = "guardian-rootfs-"
)

var (
	errImageDigestMismatch = errors.New("image-digest-mismatch")
	errImagePlatform       = errors.New("image-has-no-matching-platform")
	errImageMetadata       = errors.New("image-metadata-invalid")
)

type containerdClient struct {
	connection *grpc.ClientConn
	images     imagesapi.ImagesClient
	content    contentapi.ContentClient
	snapshots  snapshotsapi.SnapshotsClient
	containers containersapi.ContainersClient
	tasks      tasksapi.TasksClient
	leases     leasesapi.LeasesClient
	transfer   transferapi.TransferClient
}

func dialContainerd(socketPath string) (*containerdClient, error) {
	dialer := &net.Dialer{}
	connection, err := grpc.NewClient(
		"passthrough:///containerd",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableRetry(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxImageMetadataBytes+1<<20)),
	)
	if err != nil {
		return nil, err
	}
	return &containerdClient{
		connection: connection,
		images:     imagesapi.NewImagesClient(connection),
		content:    contentapi.NewContentClient(connection),
		snapshots:  snapshotsapi.NewSnapshotsClient(connection),
		containers: containersapi.NewContainersClient(connection),
		tasks:      tasksapi.NewTasksClient(connection),
		leases:     leasesapi.NewLeasesClient(connection),
		transfer:   transferapi.NewTransferClient(connection),
	}, nil
}

func (c *containerdClient) Close() error { return c.connection.Close() }

// scoped attaches the Guardian namespace. Every call in this file goes through
// it; a call without it would land in containerd's default namespace.
func scoped(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "containerd-namespace", containerdNamespace)
}

func isNotFound(err error) bool { return status.Code(err) == codes.NotFound }

// --- images -----------------------------------------------------------------

// resolvedImage is what the helper needs from an image's own metadata: the
// program to run, and the snapshot its unpacked root filesystem lives under.
type resolvedImage struct {
	entrypoint ImageEntrypoint
	chainID    string
}

// ensureImage makes the workload's image present and returns whether it had to
// be fetched. The image record must point at exactly the digest the workload
// pinned; anything else is refused rather than run.
func (c *containerdClient) ensureImage(ctx context.Context, workload Workload) (bool, *types.Descriptor, error) {
	reference := workload.ImageReference()
	target, err := c.imageTarget(ctx, reference)
	pulled := false
	if isNotFound(err) {
		if err := c.pull(ctx, reference); err != nil {
			return false, nil, err
		}
		pulled = true
		target, err = c.imageTarget(ctx, reference)
	}
	if err != nil {
		return false, nil, err
	}
	if target.GetDigest() != workload.Image.Digest {
		return false, nil, errImageDigestMismatch
	}
	return pulled, target, nil
}

func (c *containerdClient) imageTarget(ctx context.Context, reference string) (*types.Descriptor, error) {
	response, err := c.images.Get(scoped(ctx), &imagesapi.GetImageRequest{Name: reference})
	if err != nil {
		return nil, err
	}
	return response.GetImage().GetTarget(), nil
}

/*
pull asks containerd to fetch and unpack the image itself, through the Transfer
service. The reference always carries a digest, so the registry cannot hand
back different content under the same name: containerd verifies what it
downloads against that digest.

The Any type URLs are the bare message names. containerd looks its transfer
converters up by that name, and `anypb.New`'s `type.googleapis.com/` prefix
would make it fall through to a generic decode that no transfer accepts.
*/
func (c *containerdClient) pull(ctx context.Context, reference string) error {
	platform := &types.Platform{OS: "linux", Architecture: runtime.GOARCH}
	source, err := bareAny(&transfertypes.OCIRegistry{Reference: reference})
	if err != nil {
		return err
	}
	destination, err := bareAny(&transfertypes.ImageStore{
		Name:      reference,
		Platforms: []*types.Platform{platform},
		Unpacks: []*transfertypes.UnpackConfiguration{
			{Platform: platform, Snapshotter: containerdSnapshotter},
		},
	})
	if err != nil {
		return err
	}
	_, err = c.transfer.Transfer(scoped(ctx), &transferapi.TransferRequest{Source: source, Destination: destination})
	return err
}

func bareAny(message proto.Message) (*anypb.Any, error) {
	value, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	return &anypb.Any{TypeUrl: string(message.ProtoReflect().Descriptor().FullName()), Value: value}, nil
}

// OCI and Docker media types for the three documents read below.
const (
	mediaTypeOCIIndex       = "application/vnd.oci.image.index.v1+json"
	mediaTypeDockerList     = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaTypeOCIManifest    = "application/vnd.oci.image.manifest.v1+json"
	mediaTypeDockerManifest = "application/vnd.docker.distribution.manifest.v2+json"
)

type imageIndex struct {
	Manifests []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int64  `json:"size"`
		Platform  *struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

type imageManifest struct {
	Config struct {
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	} `json:"config"`
}

// imageConfig is the part of an image configuration the helper reads. `User`
// is parsed only so that ignoring it is visible here: the workload decides the
// user, never the image.
type imageConfig struct {
	Config struct {
		User       string   `json:"User"`
		Entrypoint []string `json:"Entrypoint"`
		Cmd        []string `json:"Cmd"`
		Env        []string `json:"Env"`
		WorkingDir string   `json:"WorkingDir"`
	} `json:"config"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

// resolveImage walks index → manifest → config for this host's platform.
func (c *containerdClient) resolveImage(ctx context.Context, target *types.Descriptor) (resolvedImage, error) {
	manifestDigest := target.GetDigest()
	switch target.GetMediaType() {
	case mediaTypeOCIIndex, mediaTypeDockerList:
		raw, err := c.readBlob(ctx, target.GetDigest())
		if err != nil {
			return resolvedImage{}, err
		}
		var index imageIndex
		if err := json.Unmarshal(raw, &index); err != nil {
			return resolvedImage{}, errImageMetadata
		}
		manifestDigest = ""
		for _, manifest := range index.Manifests {
			if manifest.Platform != nil && manifest.Platform.OS == "linux" &&
				manifest.Platform.Architecture == runtime.GOARCH {
				manifestDigest = manifest.Digest
				break
			}
		}
		if manifestDigest == "" {
			return resolvedImage{}, errImagePlatform
		}
	case mediaTypeOCIManifest, mediaTypeDockerManifest:
	default:
		return resolvedImage{}, errImageMetadata
	}

	raw, err := c.readBlob(ctx, manifestDigest)
	if err != nil {
		return resolvedImage{}, err
	}
	var manifest imageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Config.Digest == "" {
		return resolvedImage{}, errImageMetadata
	}
	raw, err = c.readBlob(ctx, manifest.Config.Digest)
	if err != nil {
		return resolvedImage{}, err
	}
	var config imageConfig
	if err := json.Unmarshal(raw, &config); err != nil || config.RootFS.Type != "layers" || len(config.RootFS.DiffIDs) == 0 {
		return resolvedImage{}, errImageMetadata
	}
	chainID, err := chainIDOf(config.RootFS.DiffIDs)
	if err != nil {
		return resolvedImage{}, err
	}
	args := append(append([]string{}, config.Config.Entrypoint...), config.Config.Cmd...)
	return resolvedImage{
		entrypoint: ImageEntrypoint{Args: args, Env: config.Config.Env, WorkingDir: config.Config.WorkingDir},
		chainID:    chainID,
	}, nil
}

// readBlob reads one content-addressed blob, bounded, and checks that it
// hashes to the digest it was asked for. containerd's store is content
// addressed already; checking again costs one hash and means this process
// never acts on bytes it has not verified itself.
func (c *containerdClient) readBlob(ctx context.Context, digest string) ([]byte, error) {
	if !imageDigestPattern.MatchString(digest) {
		return nil, errImageMetadata
	}
	stream, err := c.content.Read(scoped(ctx), &contentapi.ReadContentRequest{Digest: digest})
	if err != nil {
		return nil, err
	}
	var data []byte
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		data = append(data, chunk.GetData()...)
		if len(data) > maxImageMetadataBytes {
			return nil, errImageMetadata
		}
	}
	sum := sha256.Sum256(data)
	if "sha256:"+hex.EncodeToString(sum[:]) != digest {
		return nil, errImageDigestMismatch
	}
	return data, nil
}

// chainIDOf is the OCI rule for the identity of a stack of layers: the first
// diff id, then sha256 of "<previous chain id> <next diff id>" for each layer
// above it. It is the name containerd's unpacker commits the top snapshot
// under.
func chainIDOf(diffIDs []string) (string, error) {
	chain := ""
	for index, diffID := range diffIDs {
		if !imageDigestPattern.MatchString(diffID) {
			return "", errImageMetadata
		}
		if index == 0 {
			chain = diffID
			continue
		}
		sum := sha256.Sum256([]byte(chain + " " + diffID))
		chain = "sha256:" + hex.EncodeToString(sum[:])
	}
	return chain, nil
}

// --- containers ---------------------------------------------------------------

func (c *containerdClient) getContainer(ctx context.Context, id string) (*containersapi.Container, error) {
	response, err := c.containers.Get(scoped(ctx), &containersapi.GetContainerRequest{ID: id})
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return response.GetContainer(), nil
}

// containerRecord is what createContainer records. A holder has no image and an
// empty root, so both Image and Parent are empty for one.
type containerRecord struct {
	ID     string
	Image  string
	Parent string
	Spec   []byte
	Labels map[string]string
}

/*
createContainer prepares a snapshot of the image's root filesystem and records
the container against it.

Both happen under a lease, because containerd's garbage collector may remove a
snapshot nothing references yet; the container record is what references it,
and the lease covers the moment between the two.

The snapshot is writable at the snapshotter and the decoy still cannot write to
it. runc has to create mount points an image does not ship — `/proc`,
`/var/log/guardian` — inside the root before it can mount onto them, and only
then remounts the root read-only as the spec requires. A read-only view makes
that first step impossible; the lab found this on the first run, with busybox.
What the decoy's process sees is the read-only remount, and the lab's probe
asserts that from inside.
*/
func (c *containerdClient) createContainer(ctx context.Context, record containerRecord) error {
	leaseCtx, release, err := c.withLease(ctx)
	if err != nil {
		return err
	}
	defer release()

	snapshotKey := rootfsSnapshotPrefix + record.ID
	// A snapshot left by a create that failed half-way would make this View
	// collide. It belongs to this workload by name and nothing else uses it.
	_, _ = c.snapshots.Remove(leaseCtx, &snapshotsapi.RemoveSnapshotRequest{
		Snapshotter: containerdSnapshotter, Key: snapshotKey,
	})
	if _, err := c.snapshots.Prepare(leaseCtx, &snapshotsapi.PrepareSnapshotRequest{
		Snapshotter: containerdSnapshotter, Key: snapshotKey, Parent: record.Parent,
	}); err != nil {
		return err
	}
	_, err = c.containers.Create(leaseCtx, &containersapi.CreateContainerRequest{
		Container: &containersapi.Container{
			ID:          record.ID,
			Image:       record.Image,
			Labels:      record.Labels,
			Runtime:     &containersapi.Container_Runtime{Name: containerdRuntimeName},
			Spec:        &anypb.Any{TypeUrl: specTypeURL, Value: record.Spec},
			Snapshotter: containerdSnapshotter,
			SnapshotKey: snapshotKey,
		},
	})
	if err != nil {
		_, _ = c.snapshots.Remove(leaseCtx, &snapshotsapi.RemoveSnapshotRequest{
			Snapshotter: containerdSnapshotter, Key: snapshotKey,
		})
	}
	return err
}

// deleteContainer removes the record and then its snapshot. Both are
// idempotent: a missing one is already the state asked for.
func (c *containerdClient) deleteContainer(ctx context.Context, container *containersapi.Container) error {
	if _, err := c.containers.Delete(scoped(ctx), &containersapi.DeleteContainerRequest{ID: container.GetID()}); err != nil && !isNotFound(err) {
		return err
	}
	if container.GetSnapshotKey() == "" {
		return nil
	}
	_, err := c.snapshots.Remove(scoped(ctx), &snapshotsapi.RemoveSnapshotRequest{
		Snapshotter: containerdSnapshotter, Key: container.GetSnapshotKey(),
	})
	if err != nil && !isNotFound(err) {
		return err
	}
	return nil
}

func (c *containerdClient) withLease(ctx context.Context) (context.Context, func(), error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return nil, nil, err
	}
	id := "guardian-" + hex.EncodeToString(raw)
	expires := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
	if _, err := c.leases.Create(scoped(ctx), &leasesapi.CreateRequest{
		ID:     id,
		Labels: map[string]string{"containerd.io/gc.expire": expires},
	}); err != nil {
		return nil, nil, err
	}
	leaseCtx := metadata.AppendToOutgoingContext(scoped(ctx), "containerd-lease", id)
	release := func() {
		_, _ = c.leases.Delete(scoped(context.WithoutCancel(ctx)), &leasesapi.DeleteRequest{ID: id})
	}
	return leaseCtx, release, nil
}

// --- tasks --------------------------------------------------------------------

func (c *containerdClient) getTask(ctx context.Context, id string) (*tasktypes.Process, error) {
	response, err := c.tasks.Get(scoped(ctx), &tasksapi.GetRequest{ContainerID: id})
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return response.GetProcess(), nil
}

// startTask mounts the container's snapshot as the task's root and starts it.
// There is no stdio: a decoy's output goes to the files its telemetry adapter
// reads, not to a pipe held open by a root process.
//
// beforeStart runs between create and start, when the task's namespaces exist
// and its program has not run. A decoy uses it to re-check the holder whose
// namespace it just joined; an error deletes the task unstarted.
func (c *containerdClient) startTask(ctx context.Context, container *containersapi.Container, beforeStart func() error) error {
	mounts, err := c.snapshots.Mounts(scoped(ctx), &snapshotsapi.MountsRequest{
		Snapshotter: containerdSnapshotter, Key: container.GetSnapshotKey(),
	})
	if err != nil {
		return err
	}
	if _, err := c.tasks.Create(scoped(ctx), &tasksapi.CreateTaskRequest{
		ContainerID: container.GetID(),
		Rootfs:      mounts.GetMounts(),
	}); err != nil {
		return err
	}
	if beforeStart != nil {
		if err := beforeStart(); err != nil {
			_ = c.deleteTask(context.WithoutCancel(ctx), container.GetID())
			return err
		}
	}
	if _, err := c.tasks.Start(scoped(ctx), &tasksapi.StartRequest{ContainerID: container.GetID()}); err != nil {
		_ = c.deleteTask(ctx, container.GetID())
		return err
	}
	return nil
}

/*
stopTask ends a decoy: SIGTERM, a bounded wait, then SIGKILL, then the task
record is removed.

The grace period is short on purpose. A decoy is bait with no state worth a
clean shutdown, and an operator who asked for it to stop — perhaps because it
is misbehaving — is not served by waiting.
*/
func (c *containerdClient) stopTask(ctx context.Context, id string, grace time.Duration) error {
	process, err := c.getTask(ctx, id)
	if err != nil || process == nil {
		return err
	}
	if process.GetStatus() == tasktypes.Status_RUNNING || process.GetStatus() == tasktypes.Status_PAUSED {
		if err := c.signal(ctx, id, syscall.SIGTERM); err != nil {
			return err
		}
		if !c.waitExited(ctx, id, grace) {
			if err := c.signal(ctx, id, syscall.SIGKILL); err != nil {
				return err
			}
			if !c.waitExited(ctx, id, grace) {
				return fmt.Errorf("task %s did not exit after SIGKILL", id)
			}
		}
	}
	return c.deleteTask(ctx, id)
}

func (c *containerdClient) signal(ctx context.Context, id string, signal syscall.Signal) error {
	_, err := c.tasks.Kill(scoped(ctx), &tasksapi.KillRequest{ContainerID: id, Signal: uint32(signal), All: true})
	if err != nil && !isNotFound(err) && !strings.Contains(status.Convert(err).Message(), "process already finished") {
		return err
	}
	return nil
}

func (c *containerdClient) waitExited(ctx context.Context, id string, bound time.Duration) bool {
	waitCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	_, err := c.tasks.Wait(scoped(waitCtx), &tasksapi.WaitRequest{ContainerID: id})
	return err == nil || isNotFound(err)
}

func (c *containerdClient) deleteTask(ctx context.Context, id string) error {
	_, err := c.tasks.Delete(scoped(ctx), &tasksapi.DeleteTaskRequest{ContainerID: id})
	if err != nil && !isNotFound(err) {
		return err
	}
	return nil
}
