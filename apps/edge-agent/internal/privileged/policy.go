package privileged

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

const (
	APIVersion          = "guardian.privileged.v1"
	DefaultSocketPath   = "/run/guardian-edge-privd/guardian-edge-privd.sock"
	MaxMessageBytes     = 16 << 10
	MaxConcurrentRPCs   = 32
	MaxConnections      = 16
	maxAllowlistEntries = 256
)

var (
	requestIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{7,63}$`)
	interfacePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,14}$`)
	resourcePattern  = regexp.MustCompile(`^guardian-[a-z0-9][a-z0-9-]{0,47}$`)
)

// RequestViolation maps validation and authorization failures to a stable,
// payload-free gRPC status.
type RequestViolation struct {
	Code       codes.Code
	ReasonCode string
}

func (e *RequestViolation) Error() string { return e.ReasonCode }

// Allowlist is compiled from root-controlled startup arguments. Empty sets
// deny the corresponding operation arguments.
type Allowlist struct {
	interfaces     map[string]struct{}
	namespaces     map[string]struct{}
	workloads      map[string]struct{}
	addressRanges  []netip.Prefix
	allowedMethods map[string]struct{}
}

// AllowlistInput contains typed values supplied by the root-owned service
// definition, never by an RPC caller.
type AllowlistInput struct {
	Interfaces    []string
	Namespaces    []string
	Workloads     []string
	AddressRanges []string
}

// CompileAllowlist validates and copies the root-controlled policy.
func CompileAllowlist(input AllowlistInput) (Allowlist, error) {
	if len(input.Interfaces) > maxAllowlistEntries ||
		len(input.Namespaces) > maxAllowlistEntries ||
		len(input.Workloads) > maxAllowlistEntries ||
		len(input.AddressRanges) > maxAllowlistEntries {
		return Allowlist{}, fmt.Errorf("allowlist entry limit exceeded")
	}
	policy := Allowlist{
		interfaces: make(map[string]struct{}, len(input.Interfaces)),
		namespaces: make(map[string]struct{}, len(input.Namespaces)),
		workloads:  make(map[string]struct{}, len(input.Workloads)),
		allowedMethods: map[string]struct{}{
			privilegedv1.PrivilegedHelperService_GetStatus_FullMethodName:              {},
			privilegedv1.PrivilegedHelperService_EnsureAddress_FullMethodName:          {},
			privilegedv1.PrivilegedHelperService_ApplyNftablesPolicy_FullMethodName:    {},
			privilegedv1.PrivilegedHelperService_ReconcileContainer_FullMethodName:     {},
			privilegedv1.PrivilegedHelperService_EnsureNetworkNamespace_FullMethodName: {},
		},
	}
	for _, name := range input.Interfaces {
		if !validInterfaceName(name) {
			return Allowlist{}, fmt.Errorf("invalid allowlisted interface")
		}
		policy.interfaces[name] = struct{}{}
	}
	for _, name := range input.Namespaces {
		if !resourcePattern.MatchString(name) {
			return Allowlist{}, fmt.Errorf("invalid allowlisted namespace")
		}
		policy.namespaces[name] = struct{}{}
	}
	for _, name := range input.Workloads {
		if !resourcePattern.MatchString(name) {
			return Allowlist{}, fmt.Errorf("invalid allowlisted workload")
		}
		policy.workloads[name] = struct{}{}
	}
	for _, value := range input.AddressRanges {
		prefix, err := netip.ParsePrefix(value)
		if err != nil || prefix != prefix.Masked() || !validRoutableAddress(prefix.Addr()) {
			return Allowlist{}, fmt.Errorf("invalid allowlisted address range")
		}
		policy.addressRanges = append(policy.addressRanges, prefix)
	}
	return policy, nil
}

func (p Allowlist) allowsMethod(method string) bool {
	_, ok := p.allowedMethods[method]
	return ok
}

func (p Allowlist) validateAddress(request *privilegedv1.EnsureAddressRequest) error {
	if err := validateMessage(request, request.GetRequestId()); err != nil {
		return err
	}
	if !validInterfaceName(request.GetInterfaceName()) {
		return violation(codes.InvalidArgument, "invalid-interface-name")
	}
	if _, ok := p.interfaces[request.GetInterfaceName()]; !ok {
		return violation(codes.PermissionDenied, "interface-not-allowlisted")
	}
	prefix, err := netip.ParsePrefix(request.GetAddressPrefix())
	if err != nil || !validRoutableAddress(prefix.Addr()) {
		return violation(codes.InvalidArgument, "invalid-address-prefix")
	}
	if !p.allowsPrefix(prefix) {
		return violation(codes.PermissionDenied, "address-prefix-not-allowlisted")
	}
	switch request.GetDesiredState() {
	case privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
		privilegedv1.PresenceState_PRESENCE_STATE_ABSENT:
		return nil
	default:
		return violation(codes.InvalidArgument, "invalid-presence-state")
	}
}

func (p Allowlist) validateNftables(request *privilegedv1.ApplyNftablesPolicyRequest) error {
	if err := validateMessage(request, request.GetRequestId()); err != nil {
		return err
	}
	if !resourcePattern.MatchString(request.GetNamespaceName()) {
		return violation(codes.InvalidArgument, "invalid-namespace-name")
	}
	if _, ok := p.namespaces[request.GetNamespaceName()]; !ok {
		return violation(codes.PermissionDenied, "namespace-not-allowlisted")
	}
	if request.GetProfile() != privilegedv1.NftablesProfile_NFTABLES_PROFILE_DEFAULT_DENY_EGRESS {
		return violation(codes.InvalidArgument, "invalid-nftables-profile")
	}
	return nil
}

func (p Allowlist) validateContainer(request *privilegedv1.ReconcileContainerRequest) error {
	if err := validateMessage(request, request.GetRequestId()); err != nil {
		return err
	}
	if !resourcePattern.MatchString(request.GetWorkloadId()) {
		return violation(codes.InvalidArgument, "invalid-workload-id")
	}
	if _, ok := p.workloads[request.GetWorkloadId()]; !ok {
		return violation(codes.PermissionDenied, "workload-not-allowlisted")
	}
	switch request.GetDesiredState() {
	case privilegedv1.ContainerState_CONTAINER_STATE_RUNNING,
		privilegedv1.ContainerState_CONTAINER_STATE_STOPPED,
		privilegedv1.ContainerState_CONTAINER_STATE_ABSENT:
		return nil
	default:
		return violation(codes.InvalidArgument, "invalid-container-state")
	}
}

func (p Allowlist) validateNamespace(request *privilegedv1.EnsureNetworkNamespaceRequest) error {
	if err := validateMessage(request, request.GetRequestId()); err != nil {
		return err
	}
	if !resourcePattern.MatchString(request.GetNamespaceName()) {
		return violation(codes.InvalidArgument, "invalid-namespace-name")
	}
	if _, ok := p.namespaces[request.GetNamespaceName()]; !ok {
		return violation(codes.PermissionDenied, "namespace-not-allowlisted")
	}
	switch request.GetDesiredState() {
	case privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
		privilegedv1.PresenceState_PRESENCE_STATE_ABSENT:
		return nil
	default:
		return violation(codes.InvalidArgument, "invalid-presence-state")
	}
}

func validateMessage(message proto.Message, requestID string) error {
	if message == nil {
		return violation(codes.InvalidArgument, "missing-request")
	}
	if len(message.ProtoReflect().GetUnknown()) != 0 {
		return violation(codes.InvalidArgument, "unknown-protobuf-field")
	}
	if !requestIDPattern.MatchString(requestID) {
		return violation(codes.InvalidArgument, "invalid-request-id")
	}
	return nil
}

func (p Allowlist) allowsPrefix(request netip.Prefix) bool {
	for _, allowed := range p.addressRanges {
		if allowed.Addr().BitLen() == request.Addr().BitLen() &&
			request.Bits() >= allowed.Bits() && allowed.Contains(request.Addr()) {
			return true
		}
	}
	return false
}

func validInterfaceName(value string) bool {
	return value != "." && value != ".." && interfacePattern.MatchString(value) && !strings.Contains(value, "..")
}

func validRoutableAddress(address netip.Addr) bool {
	return address.IsValid() && !address.Is4In6() &&
		!address.IsUnspecified() && !address.IsLoopback() &&
		!address.IsMulticast() && !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast()
}

func violation(code codes.Code, reason string) error {
	return &RequestViolation{Code: code, ReasonCode: reason}
}

func asViolation(err error) (*RequestViolation, bool) {
	var target *RequestViolation
	ok := errors.As(err, &target)
	return target, ok
}
