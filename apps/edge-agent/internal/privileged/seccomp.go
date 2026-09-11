package privileged

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

/*
The decoy's seccomp profile: the default allowlist Docker and containerd ship,
used as published.

`seccompprofile/default.json` is `github.com/moby/profiles/seccomp` v0.2.3,
byte for byte — the file Docker Engine 29.8.0 vendors — under its Apache-2.0
licence, and its hash is pinned in a test so it cannot be edited here without
that test failing. Guardian does not maintain a syscall policy of its own. It
maintains the translation below, which is the part that has to be right about
Guardian's own grants.

The profile is written in Docker's format, not the OCI one: an entry can apply
only on some architectures, only when certain capabilities are granted, or only
on kernels from some version on. `translateSeccomp` resolves those conditions
against what this decoy is actually granted and the host it is on, the same way
Docker does, and emits a plain OCI profile. The default action is deny.

The effect for a decoy granted only NET_BIND_SERVICE: mount and namespace
syscalls, kernel module and kexec loading, `bpf`, keyrings, `io_uring`,
`userfaultfd`, and clock and swap changes are all refused before they reach the
kernel. The capability-gated entries stay out because the capabilities are not
granted, so granting one later widens seccomp and capabilities together, which
is how the profile is designed to be read.
*/

//go:embed seccompprofile/default.json
var upstreamSeccompProfile []byte

// upstreamSeccompSHA256 is the published file's hash. Updating the profile is
// replacing the file with a newer upstream release and this value with its hash,
// in one reviewed change.
const upstreamSeccompSHA256 = "536529b665dd0972c37bfb569f5d4ac8a53592e7b00752bc39ff063ca9864c74"

var errSeccompPlatform = errors.New("no seccomp architecture mapping for this platform")

type upstreamProfile struct {
	DefaultAction   string `json:"defaultAction"`
	DefaultErrnoRet *uint  `json:"defaultErrnoRet"`
	ArchMap         []struct {
		Architecture     string   `json:"architecture"`
		SubArchitectures []string `json:"subArchitectures"`
	} `json:"archMap"`
	Syscalls []struct {
		Names    []string         `json:"names"`
		Action   string           `json:"action"`
		ErrnoRet *uint            `json:"errnoRet"`
		Args     []specSyscallArg `json:"args"`
		Comment  string           `json:"comment"`
		Includes upstreamFilter   `json:"includes"`
		Excludes upstreamFilter   `json:"excludes"`
	} `json:"syscalls"`
}

type upstreamFilter struct {
	Caps      []string `json:"caps"`
	Arches    []string `json:"arches"`
	MinKernel string   `json:"minKernel"`
}

// specPlatform is what the translation depends on besides the grant. It is a
// parameter so tests can pin it; production reads it from the host.
type specPlatform struct {
	goarch      string
	kernelMajor int
	kernelMinor int
	kernelKnown bool
}

// seccompArchitectures maps Go's architecture name to libseccomp's. Only the
// architectures a Guardian Edge ships for are listed; anything else is refused
// rather than run without a profile.
var seccompArchitectures = map[string]string{
	"amd64": "SCMP_ARCH_X86_64",
	"arm64": "SCMP_ARCH_AARCH64",
}

func translateSeccomp(granted []string, platform specPlatform) (*specSeccomp, error) {
	decoder := json.NewDecoder(bytes.NewReader(upstreamSeccompProfile))
	// A field this code does not understand is a condition it would silently
	// ignore. Refuse, so a profile update that changes the format is noticed
	// when it is made rather than by a decoy that is suddenly permissive.
	decoder.DisallowUnknownFields()
	var profile upstreamProfile
	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("parse upstream seccomp profile: %w", err)
	}

	native, ok := seccompArchitectures[platform.goarch]
	if !ok {
		return nil, errSeccompPlatform
	}
	translated := &specSeccomp{DefaultAction: profile.DefaultAction, DefaultErrnoRet: profile.DefaultErrnoRet}
	for _, entry := range profile.ArchMap {
		if entry.Architecture == native {
			translated.Architectures = append([]string{entry.Architecture}, entry.SubArchitectures...)
		}
	}
	if len(translated.Architectures) == 0 {
		return nil, errSeccompPlatform
	}

	has := map[string]bool{}
	for _, capability := range granted {
		has[capability] = true
	}
	for _, entry := range profile.Syscalls {
		if !entryApplies(entry.Includes, entry.Excludes, has, platform) {
			continue
		}
		translated.Syscalls = append(translated.Syscalls, specSyscall{
			Names:    append([]string(nil), entry.Names...),
			Action:   entry.Action,
			ErrnoRet: entry.ErrnoRet,
			Args:     append([]specSyscallArg(nil), entry.Args...),
		})
	}
	return translated, nil
}

// entryApplies is Docker's rule: an exclusion by capability or architecture
// removes the entry; an inclusion requires every listed capability, one of the
// listed architectures, and at least the listed kernel.
func entryApplies(includes, excludes upstreamFilter, has map[string]bool, platform specPlatform) bool {
	for _, capability := range excludes.Caps {
		if has[capability] {
			return false
		}
	}
	for _, arch := range excludes.Arches {
		if arch == platform.goarch {
			return false
		}
	}
	if len(includes.Arches) > 0 && !contains(includes.Arches, platform.goarch) {
		return false
	}
	for _, capability := range includes.Caps {
		if !has[capability] {
			return false
		}
	}
	if includes.MinKernel != "" {
		major, minor, ok := parseKernelVersion(includes.MinKernel)
		// An unknown host kernel satisfies no minimum. The entries gated this
		// way are allowances, so not knowing means allowing less.
		if !ok || !platform.kernelKnown ||
			platform.kernelMajor < major || (platform.kernelMajor == major && platform.kernelMinor < minor) {
			return false
		}
	}
	return true
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// parseKernelVersion reads the leading "major.minor" of a release string such
// as "6.10.14-linuxkit".
func parseKernelVersion(release string) (int, int, bool) {
	parts := strings.SplitN(release, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minorDigits := parts[1]
	for index, character := range minorDigits {
		if character < '0' || character > '9' {
			minorDigits = minorDigits[:index]
			break
		}
	}
	minor, err := strconv.Atoi(minorDigits)
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}
