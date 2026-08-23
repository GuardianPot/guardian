//go:build linux

package privileged

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

type verifierFunc func(PeerCredentials) (io.Closer, error)

func (f verifierFunc) Verify(peer PeerCredentials) (io.Closer, error) { return f(peer) }

type emptyCloser struct{}

func (emptyCloser) Close() error { return nil }

func TestExtractUnixPeerCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peer.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	accepted := make(chan PeerCredentials, 1)
	errorsSeen := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			errorsSeen <- err
			return
		}
		defer connection.Close()
		peer, err := extractPeerCredentials(connection)
		if err != nil {
			errorsSeen <- err
			return
		}
		accepted <- peer
	}()
	connection, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	select {
	case err := <-errorsSeen:
		t.Fatal(err)
	case peer := <-accepted:
		if peer.PID != int32(os.Getpid()) || peer.UID != uint32(os.Geteuid()) || peer.GID != uint32(os.Getegid()) {
			t.Fatalf("peer credentials = %+v", peer)
		}
	}
}

func TestPeerPolicyRejectsWrongPIDUIDAndGID(t *testing.T) {
	var verified bool
	policy := PeerPolicy{
		AllowedUID: 10001,
		AllowedGID: 10002,
		Verifier: verifierFunc(func(peer PeerCredentials) (io.Closer, error) {
			verified = true
			if peer.PID != 1234 {
				return nil, errors.New("unexpected-peer-pid")
			}
			return emptyCloser{}, nil
		}),
	}
	for _, peer := range []PeerCredentials{
		{PID: 1, UID: 10001, GID: 10002},
		{PID: 1234, UID: 10003, GID: 10002},
		{PID: 1234, UID: 10001, GID: 10003},
	} {
		if handle, err := policy.authorize(peer); err == nil || handle != nil {
			t.Fatalf("unauthorized peer accepted: %+v", peer)
		}
	}
	if verified {
		t.Fatal("process verifier ran before UID/GID/PID screening")
	}
	handle, err := policy.authorize(PeerCredentials{PID: 1234, UID: 10001, GID: 10002})
	if err != nil {
		t.Fatal(err)
	}
	if !verified {
		t.Fatal("process verifier did not run")
	}
	_ = handle.Close()
}
