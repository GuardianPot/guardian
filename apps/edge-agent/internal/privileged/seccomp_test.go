package privileged

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

var labPlatform = specPlatform{goarch: "amd64", kernelMajor: 6, kernelMinor: 10, kernelKnown: true}

func allowedSyscalls(t *testing.T, granted []string, platform specPlatform) map[string]bool {
	t.Helper()
	profile, err := translateSeccomp(granted, platform)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, entry := range profile.Syscalls {
		if entry.Action != "SCMP_ACT_ALLOW" {
			continue
		}
		for _, name := range entry.Names {
			allowed[name] = true
		}
	}
	return allowed
}

// The vendored profile is the upstream file, unmodified. Changing it means
// replacing it with a newer release and this hash with its hash, together.
func TestTheSeccompProfileIsTheUpstreamFileUnmodified(t *testing.T) {
	sum := sha256.Sum256(upstreamSeccompProfile)
	if got := hex.EncodeToString(sum[:]); got != upstreamSeccompSHA256 {
		t.Fatalf("seccompprofile/default.json hashes to %s, pinned %s", got, upstreamSeccompSHA256)
	}
}

/*
 * A decoy's profile denies by default and keeps the escalation surfaces out.
 *
 * These are upstream's decisions, not Guardian's; the test pins that the
 * translation applied them for a decoy granted only NET_BIND_SERVICE.
 */
func TestADecoysProfileDeniesByDefaultAndKeepsEscalationOut(t *testing.T) {
	profile, err := translateSeccomp([]string{"CAP_NET_BIND_SERVICE"}, labPlatform)
	if err != nil {
		t.Fatal(err)
	}
	if profile.DefaultAction != "SCMP_ACT_ERRNO" {
		t.Fatalf("default action = %s, want deny", profile.DefaultAction)
	}
	if len(profile.Architectures) == 0 || profile.Architectures[0] != "SCMP_ARCH_X86_64" {
		t.Fatalf("architectures = %v", profile.Architectures)
	}
	allowed := allowedSyscalls(t, []string{"CAP_NET_BIND_SERVICE"}, labPlatform)
	for _, denied := range []string{
		"mount", "umount2", "pivot_root", "chroot", "setns", "unshare",
		"init_module", "finit_module", "kexec_load", "bpf", "perf_event_open",
		"add_key", "keyctl", "request_key", "io_uring_setup", "userfaultfd",
		"open_by_handle_at", "reboot", "swapon", "settimeofday",
	} {
		if allowed[denied] {
			t.Fatalf("%s is allowed for a decoy", denied)
		}
	}
	// And it is still a profile a server can run under.
	for _, needed := range []string{"read", "write", "execve", "bind", "listen", "accept4", "exit_group"} {
		if !allowed[needed] {
			t.Fatalf("%s is denied, so no pack could run", needed)
		}
	}
}

// Capability-gated entries follow the grant. Granting CAP_SYS_ADMIN — which a
// workload cannot do — would admit mount; the translation has to honour that
// in both directions or it is not applying the profile.
func TestCapabilityGatedEntriesFollowTheGrant(t *testing.T) {
	if allowedSyscalls(t, []string{"CAP_NET_BIND_SERVICE"}, labPlatform)["mount"] {
		t.Fatal("mount allowed without CAP_SYS_ADMIN")
	}
	if !allowedSyscalls(t, []string{"CAP_SYS_ADMIN"}, labPlatform)["mount"] {
		t.Fatal("the translation ignored a capability-gated entry")
	}
}

func TestArchitectureAndKernelGatedEntries(t *testing.T) {
	if !allowedSyscalls(t, nil, labPlatform)["arch_prctl"] {
		t.Fatal("arch_prctl denied on amd64")
	}
	arm := labPlatform
	arm.goarch = "arm64"
	if allowedSyscalls(t, nil, arm)["arch_prctl"] {
		t.Fatal("an amd64-only entry applied on arm64")
	}
	// ptrace is upstream's minimum-kernel entry. An old or unknown kernel gets
	// the smaller profile, never the larger one.
	if !allowedSyscalls(t, nil, labPlatform)["ptrace"] {
		t.Fatal("the kernel-gated entry did not apply on a new kernel")
	}
	old := labPlatform
	old.kernelMajor, old.kernelMinor = 4, 4
	if allowedSyscalls(t, nil, old)["ptrace"] {
		t.Fatal("the kernel-gated entry applied on a kernel below its minimum")
	}
	unknown := labPlatform
	unknown.kernelKnown = false
	if allowedSyscalls(t, nil, unknown)["ptrace"] {
		t.Fatal("an unknown kernel was treated as new enough")
	}
}

// No decoy runs without a profile, so a platform the profile cannot be
// resolved for is refused rather than run unfiltered.
func TestAnUnmappedArchitectureIsRefused(t *testing.T) {
	platform := labPlatform
	platform.goarch = "mips64"
	if _, err := translateSeccomp(nil, platform); !errors.Is(err, errSeccompPlatform) {
		t.Fatalf("err = %v, want errSeccompPlatform", err)
	}
	workload := validWorkload(t)
	if _, err := buildContainerSpec(workload, cowrieEntrypoint, platform); !errors.Is(err, errSeccompPlatform) {
		t.Fatalf("spec for an unmapped platform = %v", err)
	}
}

func TestKernelVersionsParse(t *testing.T) {
	for release, want := range map[string][2]int{
		"6.10.14-linuxkit": {6, 10}, "5.15.0-105-generic": {5, 15}, "4.8": {4, 8}, "6.1rc2": {6, 1},
	} {
		major, minor, ok := parseKernelVersion(release)
		if !ok || major != want[0] || minor != want[1] {
			t.Fatalf("%s = %d.%d %v", release, major, minor, ok)
		}
	}
	for _, release := range []string{"", "six", "6", "x.y"} {
		if _, _, ok := parseKernelVersion(release); ok {
			t.Fatalf("%q parsed", release)
		}
	}
}
