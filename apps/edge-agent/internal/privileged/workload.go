package privileged

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

/*
What a workload id resolves to, and where that comes from.

`ReconcileContainer` carries a workload id and a desired state. It carries no
image, command, mount, or capability, and that is not an oversight: `P2-W15`,
`P2-W4`, and the pack index each say in their own words that no API field may
ever supply a runtime detail. The Edge Agent asks for a workload by name; it
cannot describe one.

So the description has to come from somewhere the Edge cannot write, and the
only such place is the host itself. A workload definition is a root-owned file
installed beside the helper, named by the workload id the operator also passed
to `--allow-workload`. Two independent things must therefore be true before any
container can exist: root installed a definition, and root allowlisted its id.

This keeps the invariant the contract was built around. An attacker who fully
controls the Edge Agent process can ask for workload A instead of workload B,
and can ask for it to be running or stopped. They cannot invent a workload, name
an image, add a capability, or mount a path, because there is no field in which
to say any of those things.

The file format is deliberately not in `schemas/`. It is read only by this
helper on one host and is not a wire contract; if the Product Owner would rather
it be a published schema alongside the decoy manifest, that is a one-file
addition and a review.
*/

const (
	// WorkloadSchema is checked before any other field is read. A definition
	// this build does not implement is refused rather than partially honoured.
	WorkloadSchema = "guardian.workload.v1"

	// MaxWorkloadBytes bounds a root-owned file that is still parsed by a
	// privileged process.
	MaxWorkloadBytes = 16 << 10

	// DefaultWorkloadDirectory is where the helper looks. Root-owned, and
	// outside anything the Edge Agent identity can write.
	DefaultWorkloadDirectory = "/etc/guardian-edge/workloads"

	maxWorkloadPorts = 8
)

var (
	ErrWorkloadSchema    = errors.New("workload definition schema is not implemented by this build")
	ErrWorkloadInvalid   = errors.New("workload definition is invalid")
	ErrWorkloadNotFound  = errors.New("workload definition is not installed")
	ErrWorkloadTooLarge  = errors.New("workload definition exceeds the size bound")
	errWorkloadUnsafeRef = errors.New("workload image reference is not digest-pinned")
)

// imageDigestPattern is the only accepted digest form. A tag is not a digest:
// a tag can be moved to point at different content after an operator approved
// it, which is the substitution `AC-SEC-004` exists to prevent even before
// signature enforcement arrives in Phase 5.
var (
	imageDigestPattern    = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	imageRepositoryRegexp = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(\.[a-z0-9]+([._-][a-z0-9]+)*)*(:[0-9]{1,5})?(/[a-z0-9]+([._-][a-z0-9]+)*)+$`)
	packNamePattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)
	packVersionPattern    = regexp.MustCompile(`^[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}$`)
)

// Workload is the complete runtime description of one decoy on this host.
type Workload struct {
	Schema      string            `json:"schema"`
	WorkloadID  string            `json:"workload_id"`
	Pack        string            `json:"pack"`
	PackVersion string            `json:"pack_version"`
	Image       WorkloadImage     `json:"image"`
	Ports       []WorkloadPort    `json:"ports"`
	Privileges  WorkloadPrivilege `json:"privileges"`
	Resources   WorkloadResources `json:"resources"`
	// User is the uid/gid the decoy process runs as inside the container.
	// Required and non-zero: a decoy is attacker-facing software and has no
	// reason to be root even in its own namespace.
	User WorkloadUser `json:"user"`
}

type WorkloadImage struct {
	// Repository names where the image comes from. Guardian never resolves a
	// tag: Digest is what identifies the content.
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

type WorkloadPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type WorkloadPrivilege struct {
	// Capabilities is bounded by the same closed set the decoy manifest
	// schema pins. `GrantableCapabilities` in the deception domain is the
	// Control Plane's copy of this rule; this is the one that decides.
	Capabilities []string `json:"capabilities"`
}

type WorkloadResources struct {
	CPUMillicores int `json:"cpu_millicores"`
	MemoryMiB     int `json:"memory_mib"`
	PIDs          int `json:"pids"`
}

type WorkloadUser struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

// grantableCapabilities is the closed set a workload may request. It matches
// `schemas/decoy/v1/decoy-manifest.schema.json` exactly, and a canonical
// contract check pins that schema's enum, so widening this set alone is not
// enough to widen what a decoy gets.
var grantableCapabilities = map[string]struct{}{
	"NET_BIND_SERVICE": {},
}

// LoadWorkload reads and validates one root-installed definition.
//
// The id is not joined into a path until it has been validated against the same
// pattern the allowlist uses, so a caller cannot reach outside the directory
// with one.
func LoadWorkload(directory, workloadID string) (Workload, error) {
	if !resourcePattern.MatchString(workloadID) {
		return Workload{}, fmt.Errorf("%w: workload id", ErrWorkloadInvalid)
	}
	path := directory + "/" + workloadID + ".json"
	// Lstat, not Stat: a definition that is not a regular file is refused
	// rather than followed. A symlink here would let something outside the
	// root-owned directory decide what a privileged process runs.
	linkInfo, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Workload{}, ErrWorkloadNotFound
		}
		return Workload{}, err
	}
	if !linkInfo.Mode().IsRegular() {
		return Workload{}, fmt.Errorf("%w: not a regular file", ErrWorkloadInvalid)
	}
	file, err := os.Open(path)
	if err != nil {
		return Workload{}, err
	}
	defer func() { _ = file.Close() }()
	// The name could have been swapped for a symlink between the Lstat and the
	// open. What was opened must be the file that was checked.
	openedInfo, err := file.Stat()
	if err != nil {
		return Workload{}, err
	}
	if !os.SameFile(linkInfo, openedInfo) {
		return Workload{}, fmt.Errorf("%w: changed while being read", ErrWorkloadInvalid)
	}
	if openedInfo.Size() > MaxWorkloadBytes {
		return Workload{}, ErrWorkloadTooLarge
	}
	raw := make([]byte, MaxWorkloadBytes+1)
	read, err := io.ReadFull(file, raw)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return Workload{}, err
	}
	return ParseWorkload(raw[:read], workloadID)
}

// ParseWorkload validates a definition's bytes. Separated from reading so the
// rules can be tested without a filesystem.
func ParseWorkload(raw []byte, workloadID string) (Workload, error) {
	if len(raw) > MaxWorkloadBytes {
		return Workload{}, ErrWorkloadTooLarge
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	// An unknown field is a definition written for a different build. Ignoring
	// it would silently drop a restriction its author believed they had set.
	decoder.DisallowUnknownFields()
	var workload Workload
	if err := decoder.Decode(&workload); err != nil {
		return Workload{}, fmt.Errorf("%w: %s", ErrWorkloadInvalid, "malformed json")
	}
	if decoder.More() {
		return Workload{}, fmt.Errorf("%w: trailing content", ErrWorkloadInvalid)
	}
	// Version before anything else, so a future format cannot be half-read.
	if workload.Schema != WorkloadSchema {
		return Workload{}, ErrWorkloadSchema
	}
	if workload.WorkloadID != workloadID {
		return Workload{}, fmt.Errorf("%w: workload id does not match its file", ErrWorkloadInvalid)
	}
	if err := workload.validate(); err != nil {
		return Workload{}, err
	}
	return workload, nil
}

func (w Workload) validate() error {
	if !resourcePattern.MatchString(w.WorkloadID) {
		return fmt.Errorf("%w: workload id", ErrWorkloadInvalid)
	}
	if !packNamePattern.MatchString(w.Pack) || !packVersionPattern.MatchString(w.PackVersion) {
		return fmt.Errorf("%w: pack identity", ErrWorkloadInvalid)
	}
	if !imageDigestPattern.MatchString(w.Image.Digest) {
		return fmt.Errorf("%w: %s", ErrWorkloadInvalid, errWorkloadUnsafeRef)
	}
	if !imageRepositoryRegexp.MatchString(w.Image.Repository) || len(w.Image.Repository) > 255 {
		return fmt.Errorf("%w: image repository", ErrWorkloadInvalid)
	}
	if len(w.Ports) == 0 || len(w.Ports) > maxWorkloadPorts {
		return fmt.Errorf("%w: ports", ErrWorkloadInvalid)
	}
	for _, port := range w.Ports {
		if port.Port < 1 || port.Port > 65535 {
			return fmt.Errorf("%w: port number", ErrWorkloadInvalid)
		}
		if port.Protocol != "tcp" && port.Protocol != "udp" {
			return fmt.Errorf("%w: port protocol", ErrWorkloadInvalid)
		}
	}
	for _, capability := range w.Privileges.Capabilities {
		if _, grantable := grantableCapabilities[capability]; !grantable {
			return fmt.Errorf("%w: capability %q is not grantable", ErrWorkloadInvalid, capability)
		}
	}
	if len(w.Privileges.Capabilities) > len(grantableCapabilities) {
		return fmt.Errorf("%w: duplicate capability", ErrWorkloadInvalid)
	}
	// Resources are required, not optional. An unbounded decoy is a
	// denial-of-service surface on the host that is meant to be watching.
	if w.Resources.CPUMillicores < 10 || w.Resources.CPUMillicores > 8000 {
		return fmt.Errorf("%w: cpu_millicores", ErrWorkloadInvalid)
	}
	if w.Resources.MemoryMiB < 16 || w.Resources.MemoryMiB > 8192 {
		return fmt.Errorf("%w: memory_mib", ErrWorkloadInvalid)
	}
	if w.Resources.PIDs < 8 || w.Resources.PIDs > 4096 {
		return fmt.Errorf("%w: pids", ErrWorkloadInvalid)
	}
	// Never uid 0. A decoy that is root inside its own namespace is one kernel
	// bug away from being root outside it, and nothing a decoy does needs it.
	if w.User.UID < 1 || w.User.UID > 65533 || w.User.GID < 1 || w.User.GID > 65533 {
		return fmt.Errorf("%w: user must be a non-root uid/gid", ErrWorkloadInvalid)
	}
	return nil
}

// ImageReference is the digest-pinned name containerd is asked to resolve.
// There is no tag form, because Guardian never asks for one.
func (w Workload) ImageReference() string {
	return w.Image.Repository + "@" + w.Image.Digest
}
