//go:build linux

package privileged

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/credentials"
)

type PeerCredentials struct {
	PID int32
	UID uint32
	GID uint32
}

type ProcessVerifier interface {
	Verify(PeerCredentials) (io.Closer, error)
}

type PeerPolicy struct {
	AllowedUID uint32
	AllowedGID uint32
	Verifier   ProcessVerifier
}

func (p PeerPolicy) authorize(peer PeerCredentials) (io.Closer, error) {
	if peer.PID <= 1 {
		return nil, errors.New("unexpected-peer-pid")
	}
	if peer.UID != p.AllowedUID {
		return nil, errors.New("unexpected-peer-uid")
	}
	if peer.GID != p.AllowedGID {
		return nil, errors.New("unexpected-peer-gid")
	}
	if p.Verifier == nil {
		return nil, errors.New("missing-process-verifier")
	}
	return p.Verifier.Verify(peer)
}

// ProcProcessVerifier pins the kernel-reported PID with pidfd_open and confirms
// every Linux UID/GID slot. The dedicated guardian-edge identity is the process
// boundary; no peer-supplied PID, path, or executable claim is trusted.
type ProcProcessVerifier struct{}

func (ProcProcessVerifier) Verify(peer PeerCredentials) (io.Closer, error) {
	pidfd, err := unix.PidfdOpen(int(peer.PID), 0)
	if err != nil {
		return nil, errors.New("peer-pid-unavailable")
	}
	handle := os.NewFile(uintptr(pidfd), "guardian-peer-pidfd")
	verified := false
	defer func() {
		if !verified {
			_ = handle.Close()
		}
	}()

	procRoot := filepath.Join("/proc", strconv.FormatInt(int64(peer.PID), 10))
	uid, gid, err := readProcCredentials(filepath.Join(procRoot, "status"))
	if err != nil || uid != peer.UID || gid != peer.GID {
		return nil, errors.New("peer-process-credentials-mismatch")
	}
	verified = true
	return handle, nil
}

func readProcCredentials(path string) (uint32, uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var uidValues, gidValues []string
	scanner := bufio.NewScanner(io.LimitReader(file, 64<<10))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 5 {
			continue
		}
		switch fields[0] {
		case "Uid:":
			uidValues = fields[1:]
		case "Gid:":
			gidValues = fields[1:]
		}
	}
	if err := scanner.Err(); err != nil || len(uidValues) != 4 || len(gidValues) != 4 {
		return 0, 0, errors.New("invalid-peer-proc-status")
	}
	uid, err := equalCredentialValues(uidValues)
	if err != nil {
		return 0, 0, errors.New("peer-uid-slots-differ")
	}
	gid, err := equalCredentialValues(gidValues)
	if err != nil {
		return 0, 0, errors.New("peer-gid-slots-differ")
	}
	return uid, gid, nil
}

func equalCredentialValues(values []string) (uint32, error) {
	first, err := strconv.ParseUint(values[0], 10, 32)
	if err != nil {
		return 0, err
	}
	for _, value := range values[1:] {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil || parsed != first {
			return 0, errors.New("credential-values-differ")
		}
	}
	return uint32(first), nil
}

// PeerAuthInfo is attached to every authenticated server RPC context.
type PeerAuthInfo struct {
	credentials.CommonAuthInfo
	Credentials PeerCredentials
}

func (PeerAuthInfo) AuthType() string { return "unix-peercred" }

type serverTransportCredentials struct {
	policy   PeerPolicy
	recorder AuditRecorder
}

func NewServerTransportCredentials(policy PeerPolicy, recorder AuditRecorder) credentials.TransportCredentials {
	return &serverTransportCredentials{policy: policy, recorder: recorder}
}

func (c *serverTransportCredentials) ClientHandshake(context.Context, string, net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errors.New("server-only-unix-peercred-credentials")
}

func (c *serverTransportCredentials) ServerHandshake(raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	peer, err := extractPeerCredentials(raw)
	if err != nil {
		c.recordPeer(peer, "rejected", "peer-credentials-unavailable")
		return nil, nil, err
	}
	handle, err := c.policy.authorize(peer)
	if err != nil {
		c.recordPeer(peer, "rejected", err.Error())
		return nil, nil, fmt.Errorf("peer authorization failed: %w", err)
	}
	if handle == nil {
		c.recordPeer(peer, "rejected", "peer-process-handle-unavailable")
		return nil, nil, errors.New("peer process handle unavailable")
	}
	c.recordPeer(peer, "accepted", "peer-authorized")
	return &heldConn{Conn: raw, handle: handle}, PeerAuthInfo{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.NoSecurity},
		Credentials:    peer,
	}, nil
}

func (c *serverTransportCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "unix-peercred", SecurityVersion: "1.0"}
}

func (c *serverTransportCredentials) Clone() credentials.TransportCredentials {
	return &serverTransportCredentials{policy: c.policy, recorder: c.recorder}
}

func (*serverTransportCredentials) OverrideServerName(string) error { return nil }

func (c *serverTransportCredentials) recordPeer(peer PeerCredentials, outcome, reason string) {
	if c.recorder == nil {
		return
	}
	c.recorder.Record(context.Background(), AuditEvent{
		Event:      "peer-authentication",
		Outcome:    outcome,
		ReasonCode: reason,
		PeerPID:    peer.PID,
		PeerUID:    peer.UID,
		PeerGID:    peer.GID,
	})
}

type clientTransportCredentials struct{}

func NewClientTransportCredentials() credentials.TransportCredentials {
	return clientTransportCredentials{}
}

func (clientTransportCredentials) ClientHandshake(ctx context.Context, _ string, raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}
	if _, ok := raw.(*net.UnixConn); !ok {
		return nil, nil, errors.New("privileged helper transport must be a Unix socket")
	}
	return raw, PeerAuthInfo{CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.NoSecurity}}, nil
}

func (clientTransportCredentials) ServerHandshake(net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errors.New("client-only-unix-peercred-credentials")
}

func (clientTransportCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "unix-peercred", SecurityVersion: "1.0"}
}

func (clientTransportCredentials) Clone() credentials.TransportCredentials {
	return clientTransportCredentials{}
}

func (clientTransportCredentials) OverrideServerName(string) error { return nil }

func extractPeerCredentials(raw net.Conn) (PeerCredentials, error) {
	if _, ok := raw.LocalAddr().(*net.UnixAddr); !ok {
		return PeerCredentials{}, errors.New("non-unix-peer-connection")
	}
	for range 4 {
		syscallConn, ok := raw.(syscall.Conn)
		if ok {
			return credentialsFromSyscallConn(syscallConn)
		}
		unwrapper, ok := raw.(interface{ Unwrap() net.Conn })
		if !ok || unwrapper.Unwrap() == nil {
			break
		}
		raw = unwrapper.Unwrap()
	}
	return PeerCredentials{}, errors.New("unix-peer-syscall-connection-unavailable")
}

func credentialsFromSyscallConn(connection syscall.Conn) (PeerCredentials, error) {
	rawConn, err := connection.SyscallConn()
	if err != nil {
		return PeerCredentials{}, errors.New("unix-peer-syscall-connection-unavailable")
	}
	var credentialsValue *unix.Ucred
	var socketErr error
	if err := rawConn.Control(func(fd uintptr) {
		credentialsValue, socketErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return PeerCredentials{}, errors.New("read-unix-peer-credentials")
	}
	if socketErr != nil || credentialsValue == nil {
		return PeerCredentials{}, errors.New("read-unix-peer-credentials")
	}
	return PeerCredentials{
		PID: credentialsValue.Pid,
		UID: credentialsValue.Uid,
		GID: credentialsValue.Gid,
	}, nil
}

type heldConn struct {
	net.Conn
	handle io.Closer
	once   sync.Once
	err    error
}

func (c *heldConn) Close() error {
	c.once.Do(func() {
		c.err = errors.Join(c.Conn.Close(), c.handle.Close())
	})
	return c.err
}
