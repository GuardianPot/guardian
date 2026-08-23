package privileged

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

type acceptResult struct {
	connection net.Conn
	err        error
}

func TestLimitedListenerBoundsAcceptedConnections(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "limited.sock")
	base, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	listener := newLimitedListener(base, 1)
	t.Cleanup(func() { _ = listener.Close() })

	results := make(chan acceptResult, 2)
	go func() {
		for range 2 {
			connection, acceptErr := listener.Accept()
			results <- acceptResult{connection: connection, err: acceptErr}
			if acceptErr != nil {
				return
			}
		}
	}()

	firstClient, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = firstClient.Close() })
	firstServer := receiveAccept(t, results)

	secondClient, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = secondClient.Close() })
	select {
	case unexpected := <-results:
		if unexpected.connection != nil {
			_ = unexpected.connection.Close()
		}
		t.Fatal("listener accepted a connection above its active limit")
	case <-time.After(50 * time.Millisecond):
	}

	if err := firstServer.Close(); err != nil {
		t.Fatal(err)
	}
	secondServer := receiveAccept(t, results)
	if err := secondServer.Close(); err != nil {
		t.Fatal(err)
	}
}

func receiveAccept(t *testing.T, results <-chan acceptResult) net.Conn {
	t.Helper()
	select {
	case result := <-results:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.connection
	case <-time.After(time.Second):
		t.Fatal("listener did not accept a connection")
		return nil
	}
}
