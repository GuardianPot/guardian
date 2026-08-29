//go:build integration && linux

package privileged

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	versionv1 "github.com/containerd/containerd/api/services/version/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type fixtureVersionServer struct {
	versionv1.UnimplementedVersionServer
}

func (fixtureVersionServer) Version(context.Context, *emptypb.Empty) (*versionv1.VersionResponse, error) {
	return &versionv1.VersionResponse{Version: "fixture", Revision: "fixture"}, nil
}

func TestFixedContainerdVersionProbeFailureAndRecovery(t *testing.T) {
	if os.Getenv("GUARDIAN_RUNTIME_PROBE_FIXTURE") != "1" {
		t.Skip("isolated runtime probe fixture is not enabled")
	}
	if _, err := os.Lstat(containerdSocketPath); !os.IsNotExist(err) {
		t.Fatalf("refusing to replace existing runtime socket: %v", err)
	}
	if err := os.MkdirAll("/run/containerd", 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(containerdSocketPath)
		_ = os.Remove("/run/containerd")
	})
	start := func() (*grpc.Server, net.Listener) {
		listener, err := net.Listen("unix", containerdSocketPath)
		if err != nil {
			t.Fatal(err)
		}
		server := grpc.NewServer()
		versionv1.RegisterVersionServer(server, fixtureVersionServer{})
		go func() { _ = server.Serve(listener) }()
		return server, listener
	}
	server, listener := start()
	prober := NewContainerdRuntimeProber()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if reachable, reason := prober.ProbeRuntime(ctx); !reachable || reason != "reachable" {
		t.Fatalf("initial probe = (%v, %q)", reachable, reason)
	}
	server.Stop()
	_ = listener.Close()
	_ = os.Remove(containerdSocketPath)
	if reachable, reason := prober.ProbeRuntime(ctx); reachable || reason != "probe-failed" {
		t.Fatalf("stopped probe = (%v, %q)", reachable, reason)
	}
	server, listener = start()
	defer server.Stop()
	defer listener.Close()
	if reachable, reason := prober.ProbeRuntime(ctx); !reachable || reason != "reachable" {
		t.Fatalf("recovered probe = (%v, %q)", reachable, reason)
	}
}
