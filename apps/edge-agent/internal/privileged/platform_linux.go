//go:build linux

package privileged

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// hostPlatform is the architecture and kernel the seccomp translation is
// resolved against. A kernel that cannot be read is reported as unknown, which
// satisfies no minimum-kernel allowance.
func hostPlatform() specPlatform {
	platform := specPlatform{goarch: runtime.GOARCH}
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return platform
	}
	platform.kernelMajor, platform.kernelMinor, platform.kernelKnown = parseKernelVersion(unix.ByteSliceToString(uname.Release[:]))
	return platform
}
