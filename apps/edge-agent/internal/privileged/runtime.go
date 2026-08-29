package privileged

import (
	"context"
	"errors"
	"net"
	"time"

	versionv1 "github.com/containerd/containerd/api/services/version/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	containerdSocketPath = "/run/containerd/containerd.sock"
	runtimeProbeTimeout  = 2 * time.Second
)

type RuntimeProber interface {
	ProbeRuntime(context.Context) (reachable bool, reason string)
}

type containerdRuntimeProber struct{}

func NewContainerdRuntimeProber() RuntimeProber { return containerdRuntimeProber{} }

func (containerdRuntimeProber) ProbeRuntime(ctx context.Context) (bool, string) {
	probeCtx, cancel := context.WithTimeout(ctx, runtimeProbeTimeout)
	defer cancel()
	dialer := &net.Dialer{}
	connection, err := grpc.NewClient(
		"passthrough:///containerd",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", containerdSocketPath)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableRetry(),
	)
	if err != nil {
		return false, "probe-failed"
	}
	defer connection.Close()
	if _, err := versionv1.NewVersionClient(connection).Version(probeCtx, &emptypb.Empty{}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
			return false, "probe-timeout"
		}
		return false, "probe-failed"
	}
	return true, "reachable"
}
