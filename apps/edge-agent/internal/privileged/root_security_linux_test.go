//go:build linux

package privileged

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"google.golang.org/grpc"
)

const (
	securityChildMode = "GUARDIAN_SECURITY_CHILD_MODE"
	securitySocket    = "GUARDIAN_SECURITY_SOCKET"
	guardianTestUID   = 12001
	guardianTestGID   = 12001
	decoyTestUID      = 12002
	decoyTestGID      = 12002
)

func TestRootPeerAndSocketSecurity(t *testing.T) {
	if mode := os.Getenv(securityChildMode); mode != "" {
		runSecurityChild(t, mode, os.Getenv(securitySocket))
		return
	}
	if os.Getenv("GUARDIAN_ROOT_SECURITY_LAB") != "1" {
		t.Skip("explicit isolated root-security lab is required")
	}
	if os.Geteuid() != 0 {
		t.Skip("root-owned socket lab runs in the isolated security container")
	}

	root, err := os.MkdirTemp("/tmp", "guardian-privileged-security-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := copyTestExecutable(t, root)

	allowedRuntime := filepath.Join(root, "allowed-runtime")
	if err := os.Mkdir(allowedRuntime, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(allowedRuntime, 0, guardianTestGID); err != nil {
		t.Fatal(err)
	}
	allowedSocket := filepath.Join(allowedRuntime, "helper.sock")
	allowedAudit := &memoryAudit{}
	allowed := startRootSecurityServer(t, allowedSocket, 0o750, 0o660, guardianTestGID, allowedAudit)
	runCredentialProcess(t, executable, allowedSocket, "grpc-allowed", guardianTestUID, guardianTestGID)
	runCredentialProcess(t, executable, allowedSocket, "socket-denied", decoyTestUID, decoyTestGID)
	allowed.stop(t)

	openRuntime := filepath.Join(root, "authentication-runtime")
	if err := os.Mkdir(openRuntime, 0o755); err != nil {
		t.Fatal(err)
	}
	openSocket := filepath.Join(openRuntime, "helper.sock")
	rejectedAudit := &memoryAudit{}
	rejected := startRootSecurityServerWithOptions(t, SocketOptions{
		Path: openSocket, OwnerUID: 0, GroupGID: 0, DirectoryMode: 0o755, SocketMode: 0o666,
	}, guardianTestUID, guardianTestGID, rejectedAudit)
	runCredentialProcess(t, executable, openSocket, "grpc-denied", decoyTestUID, guardianTestGID)
	runCredentialProcess(t, executable, openSocket, "grpc-denied", guardianTestUID, decoyTestGID)
	rejected.stop(t)

	events := rejectedAudit.snapshot()
	var wrongUID, wrongGID bool
	for _, event := range events {
		wrongUID = wrongUID || event.ReasonCode == "unexpected-peer-uid"
		wrongGID = wrongGID || event.ReasonCode == "unexpected-peer-gid"
	}
	if !wrongUID || !wrongGID {
		t.Fatalf("missing wrong UID/GID audit events: %+v", events)
	}
}

func runSecurityChild(t *testing.T, mode, socketPath string) {
	t.Helper()
	switch mode {
	case "socket-denied":
		connection, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			t.Fatal("decoy identity reached the privileged socket")
		}
	case "grpc-allowed", "grpc-denied":
		connection, err := securityClientConnection(socketPath)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err = privilegedv1.NewPrivilegedHelperServiceClient(connection).GetStatus(ctx, &privilegedv1.GetStatusRequest{})
		if mode == "grpc-allowed" && err != nil {
			t.Fatalf("authorized peer rejected: %v", err)
		}
		if mode == "grpc-denied" && err == nil {
			t.Fatal("unauthorized peer RPC succeeded")
		}
	default:
		t.Fatalf("unknown security child mode %q", mode)
	}
}

func securityClientConnection(socketPath string) (*grpc.ClientConn, error) {
	dialer := &net.Dialer{}
	return grpc.NewClient(
		"passthrough:///guardian-security-helper",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		}),
		grpc.WithTransportCredentials(NewClientTransportCredentials()),
	)
}

func copyTestExecutable(t *testing.T, directory string) string {
	t.Helper()
	sourcePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	destinationPath := filepath.Join(directory, "guardian-security-test")
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		t.Fatal(err)
	}
	if err := destination.Close(); err != nil {
		t.Fatal(err)
	}
	return destinationPath
}

func runCredentialProcess(t *testing.T, executable, socketPath, mode string, uid, gid uint32) {
	t.Helper()
	process, err := os.StartProcess(executable, []string{executable, "-test.run=^TestRootPeerAndSocketSecurity$"}, &os.ProcAttr{
		Env: append(os.Environ(),
			securityChildMode+"="+mode,
			securitySocket+"="+socketPath,
			"GUARDIAN_TEST_UID="+strconv.FormatUint(uint64(uid), 10),
		),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys: &syscall.SysProcAttr{Credential: &syscall.Credential{
			Uid: uid, Gid: gid, NoSetGroups: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := process.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Success() {
		t.Fatalf("security child %s (%d:%d) failed: %s", mode, uid, gid, state)
	}
}

type rootSecurityServer struct {
	server   *Server
	listener net.Listener
	done     chan error
}

func startRootSecurityServer(t *testing.T, path string, directoryMode, socketMode os.FileMode, groupID uint32, audit AuditRecorder) *rootSecurityServer {
	t.Helper()
	return startRootSecurityServerWithOptions(t, SocketOptions{
		Path: path, OwnerUID: 0, GroupGID: groupID, DirectoryMode: directoryMode, SocketMode: socketMode,
	}, guardianTestUID, guardianTestGID, audit)
}

func startRootSecurityServerWithOptions(t *testing.T, socketOptions SocketOptions, allowedUID, allowedGID uint32, audit AuditRecorder) *rootSecurityServer {
	t.Helper()
	listener, err := ListenUnixSocket(socketOptions)
	if err != nil {
		t.Fatal(err)
	}
	allowlist, err := CompileAllowlist(AllowlistInput{})
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	server, err := NewServer(ServerConfig{
		PeerPolicy: PeerPolicy{AllowedUID: allowedUID, AllowedGID: allowedGID, Verifier: ProcProcessVerifier{}},
		Allowlist:  allowlist,
		Adapter:    UnsupportedAdapter{},
		Audit:      audit,
	})
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	return &rootSecurityServer{server: server, listener: listener, done: done}
}

func (s *rootSecurityServer) stop(t *testing.T) {
	t.Helper()
	s.server.Stop()
	_ = s.listener.Close()
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("root security server did not stop")
	}
}
