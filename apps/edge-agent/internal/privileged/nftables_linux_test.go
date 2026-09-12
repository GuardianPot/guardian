//go:build linux

package privileged

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/netip"
	"testing"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/rtnetlink"
	"golang.org/x/sys/unix"
)

func prefixes(values ...string) []netip.Prefix {
	parsed := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		parsed = append(parsed, netip.MustParsePrefix(value))
	}
	return parsed
}

func installed(ruleset nftRuleset) map[string][]string {
	observed := map[string][]string{}
	for _, rule := range ruleset.rules {
		observed[rule.chain] = append(observed[rule.chain], rule.userData)
	}
	return observed
}

var zoneInterfaces = []string{"eth1", "br-decoy"}

func rulesIn(ruleset nftRuleset, chain string) []nftRule {
	var rules []nftRule
	for _, rule := range ruleset.rules {
		if rule.chain == chain {
			rules = append(rules, rule)
		}
	}
	return rules
}

/*
 * The shape of the policy, stated once so a change to it has to be deliberate.
 *
 * Two rules decide a decoy's traffic (accept a reply, drop everything else),
 * one rule per decoy range per hook and one per decoy veth route to that
 * decision. Both hooks matter: `forward` is a decoy in its own namespace,
 * `output` is anything the host itself sends from a decoy address. Traffic
 * forwarded in from a zone interface goes to a second decision that returns it
 * only for a decoy range.
 */
func TestThePolicyCoversEveryDecoyRangeOnBothHooksAndEveryZone(t *testing.T) {
	ruleset := buildEgressPolicy(prefixes("10.20.0.0/24", "10.30.0.0/16"), zoneInterfaces)
	for chain, want := range map[string]int{
		nftEgressChain: 2,
		nftZoneChain:   3, // one return per range, then drop
		nftForwardHook: 5, // two ranges, the decoy veths, two zones
		nftOutputHook:  2,
	} {
		if got := len(rulesIn(ruleset, chain)); got != want {
			t.Fatalf("%s has %d rules, want %d", chain, got, want)
		}
	}
	// Both decisions end in an unconditional drop. If anything were appended
	// after it, the deny would stop being the default.
	for _, chain := range []string{nftEgressChain, nftZoneChain} {
		rules := rulesIn(ruleset, chain)
		last := rules[len(rules)-1]
		if len(last.expressions) != 1 || !bytes.Equal(last.expressions[0], exprVerdict(nfDrop, "")) {
			t.Fatalf("%s does not end in an unconditional drop", chain)
		}
	}
	// A zone rule matches the exact name; the veth rule matches the prefix every
	// decoy veth carries and nothing longer than it.
	forward := rulesIn(ruleset, nftForwardHook)
	if !bytes.Equal(forward[2].expressions[1], exprCmp(unix.NFT_CMP_EQ, []byte("gdn"))) ||
		!bytes.Equal(forward[2].expressions[2], exprVerdict(unix.NFT_JUMP, nftEgressChain)) {
		t.Fatal("the decoy veth rule does not send a veth's traffic to the egress decision")
	}
	if !bytes.Equal(forward[3].expressions[1], exprCmp(unix.NFT_CMP_EQ, []byte("br-decoy\x00"))) ||
		!bytes.Equal(forward[3].expressions[2], exprVerdict(unix.NFT_JUMP, nftZoneChain)) {
		t.Fatal("a zone rule is not an exact-name jump to the zone decision")
	}
}

// Every rule carries a distinct marker, which is what makes a missing rule
// detectable rather than merely a different count.
func TestEveryRuleCarriesADistinctMarker(t *testing.T) {
	ruleset := buildEgressPolicy(prefixes("10.20.0.0/24", "10.30.0.0/16"), zoneInterfaces)
	seen := map[string]struct{}{}
	for _, rule := range ruleset.rules {
		key := rule.chain + "/" + rule.userData
		if _, repeated := seen[key]; repeated {
			t.Fatalf("marker %q appears twice in one chain", key)
		}
		seen[key] = struct{}{}
	}
}

/*
 * The comparison has to fail for every way a host can be wrong.
 *
 * "Already applied" is the answer that stops the helper from writing, so a
 * comparison that is too generous leaves a decoy uncontained while reporting
 * that it is contained.
 */
func TestAnythingOtherThanTheExactPolicyIsNotApplied(t *testing.T) {
	ruleset := buildEgressPolicy(prefixes("10.20.0.0/24"), zoneInterfaces)
	if !ruleset.matches(installed(ruleset)) {
		t.Fatal("the policy does not match itself")
	}
	for name, mutate := range map[string]func(map[string][]string){
		"nothing installed at all": func(observed map[string][]string) {
			for chain := range observed {
				delete(observed, chain)
			}
		},
		"the decision chain was flushed": func(observed map[string][]string) {
			delete(observed, nftEgressChain)
		},
		"one hook lost its rule": func(observed map[string][]string) {
			observed[nftForwardHook] = nil
		},
		"the zone decision was flushed": func(observed map[string][]string) {
			delete(observed, nftZoneChain)
		},
		"a rule was replaced with another version": func(observed map[string][]string) {
			observed[nftEgressChain][0] = "gdn0:deadbeefdeadbeef:0"
		},
		"a rule was appended after the drop": func(observed map[string][]string) {
			observed[nftEgressChain] = append(observed[nftEgressChain], "gdn1:extra:2")
		},
		"an unexpected chain appeared": func(observed map[string][]string) {
			observed["someone_elses_chain"] = []string{"x"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			observed := installed(ruleset)
			mutate(observed)
			if ruleset.matches(observed) {
				t.Fatal("a host in this state was reported as already converged")
			}
		})
	}
}

/*
 * The marker changes when the covered ranges change, and only then.
 *
 * A host that converged on one set of decoy ranges and is now configured with
 * another must rewrite. Ordering is not a change: the allowlist is a set, and
 * two spellings of the same set must not cause a rewrite on every pass.
 */
func TestTheMarkerFollowsTheRangesAndInterfacesAndNotTheirOrder(t *testing.T) {
	one := buildEgressPolicy(prefixes("10.20.0.0/24", "10.30.0.0/16"), []string{"eth1", "br-decoy"})
	reordered := buildEgressPolicy(prefixes("10.30.0.0/16", "10.20.0.0/24"), []string{"br-decoy", "eth1"})
	if one.digest != reordered.digest || !one.matches(installed(reordered)) {
		t.Fatal("the same ranges and interfaces in another order produced a different policy")
	}
	for _, different := range [][]netip.Prefix{
		prefixes("10.20.0.0/24"),
		prefixes("10.20.0.0/24", "10.30.0.0/24"),
		prefixes("10.20.0.0/25", "10.30.0.0/16"),
	} {
		other := buildEgressPolicy(different, zoneInterfaces)
		if other.digest == one.digest {
			t.Fatalf("%v produced the same marker as a different range set", different)
		}
	}
	for _, interfaces := range [][]string{nil, {"eth1"}, {"eth1", "br-decoy", "eth2"}} {
		if buildEgressPolicy(prefixes("10.20.0.0/24", "10.30.0.0/16"), interfaces).digest == one.digest {
			t.Fatalf("%v produced the same marker as a different interface set", interfaces)
		}
	}
}

func TestPrefixMasksAreTheSubnetsOwn(t *testing.T) {
	for bits, want := range map[int][4]byte{
		8:  {0xff, 0, 0, 0},
		16: {0xff, 0xff, 0, 0},
		24: {0xff, 0xff, 0xff, 0},
		25: {0xff, 0xff, 0xff, 0x80},
		32: {0xff, 0xff, 0xff, 0xff},
	} {
		if got := prefixMask(bits); got != want {
			t.Fatalf("prefixMask(%d) = %v, want %v", bits, got, want)
		}
	}
	// A /32 needs no mask at all, so the rule for one is shorter.
	if len(sourceMatch(netip.MustParsePrefix("10.20.0.40/32"))) !=
		len(sourceMatch(netip.MustParsePrefix("10.20.0.0/24")))-1 {
		t.Fatal("a /32 rule carries a redundant mask")
	}
}

/*
 * nf_tables reads its integer attributes in network byte order, unlike the rest
 * of netlink. Getting this backwards produces a ruleset the kernel accepts and
 * that matches nothing, which is the failure mode this whole package is built
 * to avoid.
 */
func TestNftablesIntegerAttributesAreBigEndian(t *testing.T) {
	encoded := appendNftUint32(nil, unix.NFTA_PAYLOAD_OFFSET, ipv4SourceOffset)
	attributes, err := rtnetlink.ParseAttributes(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(attributes) != 1 || len(attributes[0].Value) != 4 {
		t.Fatalf("attributes = %+v", attributes)
	}
	if got := binary.BigEndian.Uint32(attributes[0].Value); got != ipv4SourceOffset {
		t.Fatalf("value decodes big-endian as %d, want %d", got, ipv4SourceOffset)
	}
	// The attribute type itself stays in the host's order, like every other
	// netlink attribute header.
	if attributes[0].Type != unix.NFTA_PAYLOAD_OFFSET {
		t.Fatalf("attribute type = %d", attributes[0].Type)
	}

	// A nested attribute is flagged, and the flag is not part of the type.
	nested := appendNftNested(nil, unix.NFTA_CHAIN_HOOK, encoded)
	outer, err := rtnetlink.ParseAttributes(nested)
	if err != nil {
		t.Fatal(err)
	}
	if outer[0].Type&unix.NLA_F_NESTED == 0 {
		t.Fatal("a nested attribute was not flagged as nested")
	}
	if outer[0].Type&^unix.NLA_F_NESTED != unix.NFTA_CHAIN_HOOK {
		t.Fatal("the nested flag corrupted the attribute type")
	}
}

// `ct state` is a host-order register, so its mask is host-order too. The
// address masks around it are raw packet bytes and are not.
func TestConnectionStateMaskIsHostOrderAndAddressMasksAreNot(t *testing.T) {
	expected := make([]byte, 4)
	binary.NativeEndian.PutUint32(expected, ctStateEstablished|ctStateRelated)
	if !bytes.Contains(exprBitwiseHostOrder(ctStateEstablished|ctStateRelated), expected) {
		t.Fatal("the connection-state mask is not in the register's byte order")
	}
	mask := prefixMask(24)
	if !bytes.Contains(exprBitwiseRaw(mask[:], make([]byte, 4)), []byte{0xff, 0xff, 0xff, 0x00}) {
		t.Fatal("an address mask was reordered")
	}
}

func TestStringAttributesStopAtTheTerminator(t *testing.T) {
	if got := nftStringValue([]byte("guardian_egress\x00\x00")); got != "guardian_egress" {
		t.Fatalf("value = %q", got)
	}
	if got := nftStringValue([]byte("no-terminator")); got != "no-terminator" {
		t.Fatalf("value = %q", got)
	}
	if got := nftStringValue(nil); got != "" {
		t.Fatalf("value = %q", got)
	}
}

/*
 * An empty policy must never be reported as applied.
 *
 * A helper started without decoy ranges would build a ruleset that matches
 * nothing. Telling an operator their decoys are contained when nothing is
 * containing them is worse than telling them the capability is missing.
 */
func TestAPolicyWithNothingToProtectIsUnsupported(t *testing.T) {
	capability := probeNftablesCapability(nil)
	if capability.State != privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED ||
		capability.ReasonCode != "no-decoy-ranges-configured" {
		t.Fatalf("capability = %+v", capability)
	}
	adapter := hostAdapter{nftables: capability}
	result, err := adapter.ApplyNftablesPolicy(context.Background(), NftablesOperation{
		NamespaceName: "guardian-decoy-a",
		Profile:       privilegedv1.NftablesProfile_NFTABLES_PROFILE_DEFAULT_DENY_EGRESS,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED ||
		result.ReasonCode != "no-decoy-ranges-configured" {
		t.Fatalf("result = %+v", result)
	}
}

// The contract defines one profile. An unspecified or invented one is refused
// rather than quietly treated as the default-deny it may not be.
func TestOnlyTheDefinedProfileIsAccepted(t *testing.T) {
	adapter := hostAdapter{
		nftables: AdapterCapability{
			State:      privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE,
			ReasonCode: "nftables-egress-adapter",
		},
		decoyRanges: prefixes("10.20.0.0/24"),
	}
	for _, profile := range []privilegedv1.NftablesProfile{
		privilegedv1.NftablesProfile_NFTABLES_PROFILE_UNSPECIFIED,
		privilegedv1.NftablesProfile(99),
	} {
		_, err := adapter.ApplyNftablesPolicy(context.Background(), NftablesOperation{
			NamespaceName: "guardian-decoy-a", Profile: profile,
		})
		refusal, ok := asViolation(err)
		if !ok || refusal.ReasonCode != "invalid-nftables-profile" {
			t.Fatalf("profile %v = %v, want a typed refusal", profile, err)
		}
	}
}

// Whatever the probe finds, the reason is a closed token and never a claim the
// capability exists when it does not.
func TestTheNftablesCapabilityIsReportedHonestly(t *testing.T) {
	capability := probeNftablesCapability(prefixes("10.20.0.0/24"))
	if !reasonCodePattern.MatchString(capability.ReasonCode) {
		t.Fatalf("reason %q is not a closed token", capability.ReasonCode)
	}
	switch capability.State {
	case privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE:
		if !holdsCapNetAdmin() {
			t.Fatal("egress policy was claimed without CAP_NET_ADMIN")
		}
	case privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED:
	default:
		t.Fatalf("state = %v", capability.State)
	}
}
