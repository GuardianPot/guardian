package presence

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"os"
	"strings"
	"testing"
)

// Runs only where a real neighbour cache exists. Proves the probe distinguishes
// a live host from a free address on an actual segment, as an unprivileged user.
func TestLiveProbeDistinguishesAGatewayFromAFreeAddress(t *testing.T) {
	if os.Getenv("GUARDIAN_LIVE_PROBE") != "1" {
		t.Skip("set GUARDIAN_LIVE_PROBE=1 on a Linux host to run")
	}
	gw := defaultGatewayForTest(t)
	probe := NewNeighbourProbe()
	ctx := context.Background()

	inUse, checked, err := probe.InUse(ctx, Address{DecoyID: "gw", InterfaceName: "eth0", Prefix: gw.String() + "/32"})
	if err != nil || !checked || !inUse {
		t.Fatalf("gateway %s: inUse=%v checked=%v err=%v, want a checked conflict", gw, inUse, checked, err)
	}

	free := gw.As4()
	free[2], free[3] = 99, 234
	candidate := netip.AddrFrom4(free)
	inUse, checked, err = probe.InUse(ctx, Address{DecoyID: "free", InterfaceName: "eth0", Prefix: candidate.String() + "/32"})
	if err != nil || !checked || inUse {
		t.Fatalf("free %s: inUse=%v checked=%v err=%v, want checked and free", candidate, inUse, checked, err)
	}
	t.Logf("uid=%d: %s in use, %s free", os.Getuid(), gw, candidate)
}

func defaultGatewayForTest(t *testing.T) netip.Addr {
	t.Helper()
	file, err := os.Open("/proc/net/route")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) < 3 || f[1] != "00000000" {
			continue
		}
		raw, err := hex.DecodeString(f[2])
		if err != nil || len(raw) != 4 {
			continue
		}
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], binary.LittleEndian.Uint32(raw))
		return netip.AddrFrom4(b)
	}
	t.Skip("no default route")
	return netip.Addr{}
}
