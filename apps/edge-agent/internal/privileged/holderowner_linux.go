//go:build linux

package privileged

import (
	"os"
	"syscall"
)

func verifyRootOwned(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return ErrHolderBinaryUntrusted
	}
	return nil
}
