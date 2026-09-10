//go:build linux

package privileged

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"sort"
	"strconv"

	"golang.org/x/sys/unix"
)

/*
The default-deny egress policy, encoded for NETLINK_NETFILTER.

`AC-SEC-003` is the reason this file exists: a decoy must not be able to reach
production or the internet. Cowrie is medium-interaction software running code
an attacker typed, so "the decoy cannot start an outbound connection" is the
control that makes deploying it defensible at all.

Like the address adapter, this speaks netlink rather than running `nft`, for
the same reason: no process-execution primitive inside a daemon holding
CAP_NET_ADMIN. nftables is a much larger protocol than address management, so
only the pieces this one ruleset needs are encoded here.

Two conventions of nf_tables that are easy to get wrong, and are worth stating
because a silent mistake in either produces a ruleset that matches nothing
while reporting success:

  - Attribute integers are big-endian, unlike the rest of netlink.
  - Register contents are not. An address loaded from the packet is raw network
    bytes, but `ct state` is a host-order u32, so its mask is host-order too.
*/

const (
	// nftPolicyVersion is part of the rule marker. Changing the ruleset without
	// changing this would let a host keep an older policy and be reported as
	// converged.
	nftPolicyVersion = "gdn1"

	nftTableName   = "guardian_decoy"
	nftEgressChain = "guardian_egress"
	nftForwardHook = "guardian_forward"
	nftOutputHook  = "guardian_output"

	// Verdicts. NF_DROP and NF_ACCEPT are not exported by x/sys/unix.
	nfDrop   = 0
	nfAccept = 1

	// ct state is a bitmask in a host-order register:
	// established is bit 1 and related is bit 2.
	ctStateEstablished = 0x02
	ctStateRelated     = 0x04

	// Offset of `saddr` within `struct iphdr`.
	ipv4SourceOffset = 12
	ipv4AddressBytes = 4

	// The `filter` priority both iptables and nft use for these hooks.
	nftFilterPriority = 0
)

// nftRuleset is the policy as a set of rules, built before anything is sent so
// that the same description can be compared against the host and then applied.
type nftRuleset struct {
	digest string
	rules  []nftRule
}

type nftRule struct {
	chain       string
	userData    string
	expressions [][]byte
}

/*
buildEgressPolicy describes the whole of `NFTABLES_PROFILE_DEFAULT_DENY_EGRESS`.

The policy is keyed on the decoy address ranges the helper was started with,
not on an interface or a namespace. That choice matters:

  - Those ranges are root-controlled startup arguments, never RPC input.
  - They are the *same* compiled set that gates `EnsureAddress`, so the set of
    addresses Guardian can place and the set it denies egress for cannot drift
    apart. A decoy address that exists is an address this policy covers.
  - It needs no knowledge of how the decoy's namespace or veth is named, so it
    cannot silently stop matching when `P2-W3` chooses those names.

Base chains carry policy `accept` on purpose. An nftables chain policy applies
to every packet reaching that hook, so a `drop` policy in a Guardian table would
drop the host's own forwarded traffic. Denial is expressed in the rules, scoped
to decoy sources, and a drop in any table still wins — so a deny-shaped policy
composes safely with whatever else is on the host.
*/
func buildEgressPolicy(ranges []netip.Prefix) nftRuleset {
	sorted := make([]netip.Prefix, len(ranges))
	copy(sorted, ranges)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })

	digest := policyDigest(sorted)
	ruleset := nftRuleset{digest: digest}

	// The shared decision, reached only by traffic from a decoy address.
	ruleset.add(nftEgressChain, digest, [][]byte{
		exprCtState(),
		exprBitwiseHostOrder(ctStateEstablished | ctStateRelated),
		exprCmp(unix.NFT_CMP_NEQ, make([]byte, 4)),
		exprVerdict(nfAccept, ""),
	})
	ruleset.add(nftEgressChain, digest, [][]byte{exprVerdict(nfDrop, "")})

	// One rule per range per hook. A set would be tidier and is deliberately
	// avoided: set element encoding is a large amount of additional protocol
	// for a list that is normally one or two entries long.
	for _, hook := range []string{nftForwardHook, nftOutputHook} {
		for _, prefix := range sorted {
			ruleset.add(hook, digest, sourceMatch(prefix))
		}
	}
	return ruleset
}

func (r *nftRuleset) add(chain, digest string, expressions [][]byte) {
	index := 0
	for _, existing := range r.rules {
		if existing.chain == chain {
			index++
		}
	}
	r.rules = append(r.rules, nftRule{
		chain:       chain,
		userData:    nftPolicyVersion + ":" + digest + ":" + strconv.Itoa(index),
		expressions: expressions,
	})
}

// sourceMatch is `ip saddr <prefix> jump guardian_egress`.
func sourceMatch(prefix netip.Prefix) [][]byte {
	network := prefix.Masked().Addr().As4()
	expressions := [][]byte{exprPayloadIPv4Source()}
	if prefix.Bits() < 32 {
		mask := prefixMask(prefix.Bits())
		expressions = append(expressions, exprBitwiseRaw(mask[:], make([]byte, ipv4AddressBytes)))
	}
	expressions = append(expressions,
		exprCmp(unix.NFT_CMP_EQ, network[:]),
		exprVerdict(unix.NFT_JUMP, nftEgressChain),
	)
	return expressions
}

func prefixMask(bits int) [4]byte {
	var mask [4]byte
	binary.BigEndian.PutUint32(mask[:], ^uint32(0)<<uint(32-bits))
	return mask
}

func policyDigest(ranges []netip.Prefix) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(nftPolicyVersion))
	// The profile name is folded in so that a second profile, if one is ever
	// added, cannot be mistaken for this one on a host that already converged.
	_, _ = digest.Write([]byte("\x00default-deny-egress"))
	for _, prefix := range ranges {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(prefix.String()))
	}
	return hex.EncodeToString(digest.Sum(nil)[:8])
}

// applyEgressPolicy replaces Guardian's table with the described ruleset in one
// transaction.
//
// The table is created, deleted, and created again inside the batch. Creating
// first makes the delete safe when there is no table yet; deleting is what
// removes rules a previous version left behind. Because the batch is atomic,
// there is no moment at which the table exists without its rules.
func applyEgressPolicy(connection *netlinkConn, ruleset nftRuleset) error {
	messages := []batchMessage{batchBoundary(unix.NFNL_MSG_BATCH_BEGIN)}
	appendMessage := func(command uint16, flags uint16, payload []byte) {
		messages = append(messages, batchMessage{
			messageType: nftMessageType(command),
			flags:       unix.NLM_F_REQUEST | unix.NLM_F_ACK | flags,
			payload:     payload,
		})
	}

	appendMessage(unix.NFT_MSG_NEWTABLE, unix.NLM_F_CREATE, tablePayload())
	appendMessage(unix.NFT_MSG_DELTABLE, 0, tablePayload())
	appendMessage(unix.NFT_MSG_NEWTABLE, unix.NLM_F_CREATE, tablePayload())

	appendMessage(unix.NFT_MSG_NEWCHAIN, unix.NLM_F_CREATE, chainPayload(nftEgressChain, nil))
	appendMessage(unix.NFT_MSG_NEWCHAIN, unix.NLM_F_CREATE,
		chainPayload(nftForwardHook, &nftHook{number: unix.NF_INET_FORWARD}))
	appendMessage(unix.NFT_MSG_NEWCHAIN, unix.NLM_F_CREATE,
		chainPayload(nftOutputHook, &nftHook{number: unix.NF_INET_LOCAL_OUT}))

	for _, rule := range ruleset.rules {
		appendMessage(unix.NFT_MSG_NEWRULE, unix.NLM_F_CREATE|unix.NLM_F_APPEND, rulePayload(rule))
	}
	messages = append(messages, batchBoundary(unix.NFNL_MSG_BATCH_END))
	return connection.executeBatch(messages)
}

/*
observedEgressPolicy reads back what is actually installed.

Every rule carries a marker naming the policy version, the digest of the ranges
it was built from, and its position. Comparing markers catches the three ways a
host can be wrong: no table, a table whose rules were flushed, and a table built
from a different set of decoy ranges.

The expressions themselves are not parsed back. What proves they behave is the
lab test, which sends a packet.
*/
func observedEgressPolicy(connection *netlinkConn) (map[string][]string, error) {
	payload := nfgenmsg(unix.NFPROTO_IPV4)
	payload = appendNftString(payload, unix.NFTA_RULE_TABLE, nftTableName)
	messages, err := connection.execute(nftMessageType(unix.NFT_MSG_GETRULE),
		unix.NLM_F_REQUEST|unix.NLM_F_DUMP, payload)
	if err != nil {
		// No table is a legitimate observation: the policy is simply not
		// installed, which is what an unconverged host looks like.
		if errors.Is(err, unix.ENOENT) {
			return map[string][]string{}, nil
		}
		return nil, err
	}
	observed := map[string][]string{}
	for _, message := range messages {
		if len(message.data) < nfgenmsgBytes {
			continue
		}
		attributes, err := parseNetlinkAttributes(message.data[nfgenmsgBytes:])
		if err != nil {
			return nil, err
		}
		var chain, userData string
		for _, attribute := range attributes {
			switch attribute.attributeType &^ unix.NLA_F_NESTED {
			case unix.NFTA_RULE_CHAIN:
				chain = nftStringValue(attribute.value)
			case unix.NFTA_RULE_USERDATA:
				userData = nftStringValue(attribute.value)
			}
		}
		if chain != "" {
			observed[chain] = append(observed[chain], userData)
		}
	}
	return observed, nil
}

// matches reports whether the host already carries exactly this ruleset.
func (r nftRuleset) matches(observed map[string][]string) bool {
	expected := map[string][]string{}
	for _, rule := range r.rules {
		expected[rule.chain] = append(expected[rule.chain], rule.userData)
	}
	if len(expected) != len(observed) {
		return false
	}
	for chain, wanted := range expected {
		found, ok := observed[chain]
		if !ok || len(found) != len(wanted) {
			return false
		}
		for index := range wanted {
			if found[index] != wanted[index] {
				return false
			}
		}
	}
	return true
}

// --- message and attribute encoding -----------------------------------------

// nfgenmsg is the fixed header every netfilter message carries:
// family, version, and a big-endian resource id.
const nfgenmsgBytes = 4

func nfgenmsg(family uint8) []byte {
	return []byte{family, unix.NFNETLINK_V0, 0, 0}
}

func nftMessageType(command uint16) uint16 {
	return uint16(unix.NFNL_SUBSYS_NFTABLES)<<8 | command
}

// batchBoundary opens or closes a transaction. Its resource id names the
// nftables subsystem, which is how the kernel knows whose batch this is.
func batchBoundary(kind uint16) batchMessage {
	payload := []byte{unix.AF_UNSPEC, unix.NFNETLINK_V0, 0, 0}
	binary.BigEndian.PutUint16(payload[2:4], uint16(unix.NFNL_SUBSYS_NFTABLES))
	return batchMessage{messageType: kind, flags: unix.NLM_F_REQUEST, payload: payload}
}

func tablePayload() []byte {
	return appendNftString(nfgenmsg(unix.NFPROTO_IPV4), unix.NFTA_TABLE_NAME, nftTableName)
}

type nftHook struct{ number uint32 }

func chainPayload(name string, hook *nftHook) []byte {
	payload := nfgenmsg(unix.NFPROTO_IPV4)
	payload = appendNftString(payload, unix.NFTA_CHAIN_TABLE, nftTableName)
	payload = appendNftString(payload, unix.NFTA_CHAIN_NAME, name)
	if hook == nil {
		return payload
	}
	inner := appendNftUint32(nil, unix.NFTA_HOOK_HOOKNUM, hook.number)
	inner = appendNftUint32(inner, unix.NFTA_HOOK_PRIORITY, uint32(nftFilterPriority))
	payload = appendNftNested(payload, unix.NFTA_CHAIN_HOOK, inner)
	// Accept, not drop: see buildEgressPolicy.
	payload = appendNftUint32(payload, unix.NFTA_CHAIN_POLICY, nfAccept)
	payload = appendNftString(payload, unix.NFTA_CHAIN_TYPE, "filter")
	return payload
}

func rulePayload(rule nftRule) []byte {
	payload := nfgenmsg(unix.NFPROTO_IPV4)
	payload = appendNftString(payload, unix.NFTA_RULE_TABLE, nftTableName)
	payload = appendNftString(payload, unix.NFTA_RULE_CHAIN, rule.chain)
	var list []byte
	for _, expression := range rule.expressions {
		list = appendNftNested(list, unix.NFTA_LIST_ELEM, expression)
	}
	payload = appendNftNested(payload, unix.NFTA_RULE_EXPRESSIONS, list)
	payload = appendAttribute(payload, unix.NFTA_RULE_USERDATA, append([]byte(rule.userData), 0))
	return payload
}

func nftExpression(name string, data []byte) []byte {
	out := appendNftString(nil, unix.NFTA_EXPR_NAME, name)
	return appendNftNested(out, unix.NFTA_EXPR_DATA, data)
}

// exprPayloadIPv4Source loads the packet's source address into register 1.
func exprPayloadIPv4Source() []byte {
	inner := appendNftUint32(nil, unix.NFTA_PAYLOAD_DREG, unix.NFT_REG_1)
	inner = appendNftUint32(inner, unix.NFTA_PAYLOAD_BASE, unix.NFT_PAYLOAD_NETWORK_HEADER)
	inner = appendNftUint32(inner, unix.NFTA_PAYLOAD_OFFSET, ipv4SourceOffset)
	inner = appendNftUint32(inner, unix.NFTA_PAYLOAD_LEN, ipv4AddressBytes)
	return nftExpression("payload", inner)
}

// exprCtState loads the connection-tracking state into register 1.
func exprCtState() []byte {
	inner := appendNftUint32(nil, unix.NFTA_CT_DREG, unix.NFT_REG_1)
	inner = appendNftUint32(inner, unix.NFTA_CT_KEY, unix.NFT_CT_STATE)
	return nftExpression("ct", inner)
}

// exprBitwiseHostOrder masks a register holding a host-order u32, which is what
// `ct state` produces.
func exprBitwiseHostOrder(mask uint32) []byte {
	value := make([]byte, 4)
	binary.NativeEndian.PutUint32(value, mask)
	return exprBitwiseRaw(value, make([]byte, 4))
}

// exprBitwiseRaw masks a register holding raw packet bytes, such as an address.
func exprBitwiseRaw(mask, xor []byte) []byte {
	inner := appendNftUint32(nil, unix.NFTA_BITWISE_SREG, unix.NFT_REG_1)
	inner = appendNftUint32(inner, unix.NFTA_BITWISE_DREG, unix.NFT_REG_1)
	inner = appendNftUint32(inner, unix.NFTA_BITWISE_LEN, uint32(len(mask)))
	inner = appendNftNested(inner, unix.NFTA_BITWISE_MASK, appendAttribute(nil, unix.NFTA_DATA_VALUE, mask))
	inner = appendNftNested(inner, unix.NFTA_BITWISE_XOR, appendAttribute(nil, unix.NFTA_DATA_VALUE, xor))
	return nftExpression("bitwise", inner)
}

func exprCmp(operation uint32, value []byte) []byte {
	inner := appendNftUint32(nil, unix.NFTA_CMP_SREG, unix.NFT_REG_1)
	inner = appendNftUint32(inner, unix.NFTA_CMP_OP, operation)
	inner = appendNftNested(inner, unix.NFTA_CMP_DATA, appendAttribute(nil, unix.NFTA_DATA_VALUE, value))
	return nftExpression("cmp", inner)
}

// exprVerdict ends rule evaluation. `chain` is set only for a jump.
func exprVerdict(code int32, chain string) []byte {
	verdict := appendNftUint32(nil, unix.NFTA_VERDICT_CODE, uint32(code))
	if chain != "" {
		verdict = appendNftString(verdict, unix.NFTA_VERDICT_CHAIN, chain)
	}
	data := appendNftNested(nil, unix.NFTA_DATA_VERDICT, verdict)
	inner := appendNftUint32(nil, unix.NFTA_IMMEDIATE_DREG, unix.NFT_REG_VERDICT)
	inner = appendNftNested(inner, unix.NFTA_IMMEDIATE_DATA, data)
	return nftExpression("immediate", inner)
}

// appendNftUint32 writes a big-endian attribute. Unlike the rest of netlink,
// nf_tables reads its integer attributes in network byte order.
func appendNftUint32(payload []byte, attributeType uint16, value uint32) []byte {
	encoded := make([]byte, 4)
	binary.BigEndian.PutUint32(encoded, value)
	return appendAttribute(payload, attributeType, encoded)
}

// appendNftString writes a NUL-terminated string attribute.
func appendNftString(payload []byte, attributeType uint16, value string) []byte {
	return appendAttribute(payload, attributeType, append([]byte(value), 0))
}

func appendNftNested(payload []byte, attributeType uint16, inner []byte) []byte {
	return appendAttribute(payload, attributeType|unix.NLA_F_NESTED, inner)
}

func nftStringValue(value []byte) string {
	for index, character := range value {
		if character == 0 {
			return string(value[:index])
		}
	}
	return string(value)
}
