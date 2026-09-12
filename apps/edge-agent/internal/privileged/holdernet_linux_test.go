//go:build linux

package privileged

import (
	"strings"
	"testing"
)

// A veth's alias is its ownership marker. Only the exact form the helper writes
// is Guardian's; anything else on the host is somebody else's link.
func TestOnlyTheHelpersOwnAliasMarksAGuardianLink(t *testing.T) {
	alias := holderLinkAlias(testWorkloadID, "eth1", 4242)
	owner, ok := parseHolderLinkAlias(alias)
	if !ok || owner != (holderLink{workloadID: testWorkloadID, zone: "eth1", pid: 4242}) {
		t.Fatalf("round trip = %+v %v", owner, ok)
	}
	for _, foreign := range []string{
		"", "uplink to core", "guardian-decoy", strings.TrimPrefix(alias, "guardian-"),
		"guardian-decoy " + testWorkloadID + " eth1",
		"guardian-decoy " + testWorkloadID + " eth1 4242 extra",
		"guardian-decoy " + testWorkloadID + " eth1 1",
		"guardian-decoy " + testWorkloadID + " eth1 04242",
		"guardian-decoy " + testWorkloadID + " eth1 -4242",
		"guardian-decoy " + testWorkloadID + " ../eth1 4242",
		"guardian-decoy not-a-workload eth1 4242",
		"guardian-decoy " + testWorkloadID + "  eth1 4242",
		alias + " ",
	} {
		if owner, ok := parseHolderLinkAlias(foreign); ok {
			t.Fatalf("%q parsed as %+v", foreign, owner)
		}
	}
}
