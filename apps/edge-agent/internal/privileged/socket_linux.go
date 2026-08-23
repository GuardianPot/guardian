//go:build linux

package privileged

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

type SocketOptions struct {
	Path          string
	OwnerUID      uint32
	GroupGID      uint32
	DirectoryMode os.FileMode
	SocketMode    os.FileMode
}

// VerifyUnixSocket checks the directory and socket metadata without opening or
// mutating either object. The Edge client uses it before every new connection.
func VerifyUnixSocket(options SocketOptions) error {
	if !filepath.IsAbs(options.Path) || filepath.Clean(options.Path) != options.Path {
		return errors.New("socket path must be absolute and clean")
	}
	if err := verifyFilesystemObject(filepath.Dir(options.Path), options.OwnerUID, options.GroupGID, options.DirectoryMode, true); err != nil {
		return fmt.Errorf("verify socket directory: %w", err)
	}
	if err := verifyFilesystemObject(options.Path, options.OwnerUID, options.GroupGID, options.SocketMode, false); err != nil {
		return fmt.Errorf("verify socket: %w", err)
	}
	return nil
}

// ListenUnixSocket validates a root-controlled directory, safely replaces only
// a stale socket owned by the expected identity, and verifies final ownership.
func ListenUnixSocket(options SocketOptions) (net.Listener, error) {
	if !filepath.IsAbs(options.Path) || filepath.Clean(options.Path) != options.Path {
		return nil, errors.New("socket path must be absolute and clean")
	}
	if options.DirectoryMode.Perm() != options.DirectoryMode || options.SocketMode.Perm() != options.SocketMode {
		return nil, errors.New("socket modes must contain permission bits only")
	}
	parent := filepath.Dir(options.Path)
	if err := verifyFilesystemObject(parent, options.OwnerUID, options.GroupGID, options.DirectoryMode, true); err != nil {
		return nil, fmt.Errorf("verify socket directory: %w", err)
	}
	if info, err := os.Lstat(options.Path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("existing socket path is not a socket")
		}
		if err := verifyOwnership(info, options.OwnerUID, options.GroupGID); err != nil {
			return nil, errors.New("existing socket ownership mismatch")
		}
		if err := os.Remove(options.Path); err != nil {
			return nil, fmt.Errorf("remove stale socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect socket path: %w", err)
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: options.Path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on privileged helper socket: %w", err)
	}
	listener.SetUnlinkOnClose(false)
	cleanup := func(cause error) (net.Listener, error) {
		_ = listener.Close()
		_ = os.Remove(options.Path)
		return nil, cause
	}
	if err := os.Chown(options.Path, int(options.OwnerUID), int(options.GroupGID)); err != nil {
		return cleanup(fmt.Errorf("set socket ownership: %w", err))
	}
	if err := os.Chmod(options.Path, options.SocketMode); err != nil {
		return cleanup(fmt.Errorf("set socket permissions: %w", err))
	}
	if err := verifyFilesystemObject(options.Path, options.OwnerUID, options.GroupGID, options.SocketMode, false); err != nil {
		return cleanup(fmt.Errorf("verify created socket: %w", err))
	}
	createdInfo, err := os.Lstat(options.Path)
	if err != nil {
		return cleanup(fmt.Errorf("record created socket: %w", err))
	}
	return &ownedUnixListener{UnixListener: listener, path: options.Path, createdInfo: createdInfo}, nil
}

func verifyFilesystemObject(path string, uid, gid uint32, mode os.FileMode, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symbolic links are forbidden")
	}
	if directory && !info.IsDir() {
		return errors.New("expected directory")
	}
	if !directory && info.Mode()&os.ModeSocket == 0 {
		return errors.New("expected Unix socket")
	}
	if info.Mode().Perm() != mode.Perm() {
		return errors.New("permission mode mismatch")
	}
	return verifyOwnership(info, uid, gid)
}

func verifyOwnership(info os.FileInfo, uid, gid uint32) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uid || stat.Gid != gid {
		return errors.New("owner or group mismatch")
	}
	return nil
}

type ownedUnixListener struct {
	*net.UnixListener
	path        string
	createdInfo os.FileInfo
	once        sync.Once
	err         error
}

func (l *ownedUnixListener) Close() error {
	l.once.Do(func() {
		l.err = l.UnixListener.Close()
		current, err := os.Lstat(l.path)
		if err == nil && os.SameFile(l.createdInfo, current) {
			l.err = errors.Join(l.err, os.Remove(l.path))
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			l.err = errors.Join(l.err, err)
		}
	})
	return l.err
}
