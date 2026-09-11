//go:build linux

package privileged

import (
	"golang.org/x/sys/unix"
)

/*
The address ownership rule.

The netlink mechanism itself lives in `internal/rtnetlink`, shared with the
decoy network holder. What stays here is policy: which addresses are
Guardian's. Every address the helper adds carries the IPv4 label
`<interface>:gdn`, and the address adapter refuses to adopt or remove one that
does not.
*/

const (
	// guardianLabelSuffix marks every address this helper adds. IPv4 address
	// labels are the kernel's own ownership marker: `ip -4 addr show` prints
	// them, so an operator can see which addresses are Guardian's, and the
	// adapter refuses to remove one that is not.
	guardianLabelSuffix = ":gdn"
	// A label is capped at IFNAMSIZ-1 characters by the kernel, and convention
	// requires it to start with the interface name.
	maxLabelledInterface = unix.IFNAMSIZ - 1 - len(guardianLabelSuffix)
)

// guardianLabel returns the ownership label for an interface, and whether one
// fits. A name too long to label is refused rather than silently left unmarked:
// an unmarked address is one this adapter could later delete without knowing
// whether it put it there.
func guardianLabel(interfaceName string) (string, bool) {
	if interfaceName == "" || len(interfaceName) > maxLabelledInterface {
		return "", false
	}
	return interfaceName + guardianLabelSuffix, true
}
