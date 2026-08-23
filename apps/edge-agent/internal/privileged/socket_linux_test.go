//go:build linux

package privileged

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestListenUnixSocketEnforcesOwnershipModeAndSafeCleanup(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "runtime")
	if err := os.Mkdir(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "helper.sock")
	options := SocketOptions{
		Path: path, OwnerUID: uint32(os.Geteuid()), GroupGID: uint32(os.Getegid()),
		DirectoryMode: 0o750, SocketMode: 0o660,
	}
	listener, err := ListenUnixSocket(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnixSocket(options); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o660 || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket mode = %v", info.Mode())
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("socket remained after close: %v", err)
	}
}

func TestListenUnixSocketRejectsSymlinkAndWrongDirectoryMode(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "runtime")
	if err := os.Mkdir(directory, 0o770); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o770); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "helper.sock")
	options := SocketOptions{
		Path: path, OwnerUID: uint32(os.Geteuid()), GroupGID: uint32(os.Getegid()),
		DirectoryMode: 0o750, SocketMode: 0o660,
	}
	if _, err := ListenUnixSocket(options); err == nil {
		t.Fatal("group-writable runtime directory was accepted")
	}
	if err := os.Chmod(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := ListenUnixSocket(options); err == nil {
		t.Fatal("symlink socket path was accepted")
	}
}

func TestListenUnixSocketReplacesOnlyExpectedStaleSocket(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "runtime")
	if err := os.Mkdir(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "helper.sock")
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o660); err != nil {
		t.Fatal(err)
	}
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	options := SocketOptions{
		Path: path, OwnerUID: uint32(os.Geteuid()), GroupGID: uint32(os.Getegid()),
		DirectoryMode: 0o750, SocketMode: 0o660,
	}
	listener, err := ListenUnixSocket(options)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
}
