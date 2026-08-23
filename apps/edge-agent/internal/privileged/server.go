package privileged

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	operationTimeout = 5 * time.Second
	idempotencyLimit = 1024
	idempotencyTTL   = 15 * time.Minute
)

var (
	reasonCodePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	auditRPCMethodPattern = regexp.MustCompile(`^/[A-Za-z0-9._]{1,128}/[A-Za-z0-9_]{1,64}$`)
)

type ServerConfig struct {
	PeerPolicy PeerPolicy
	Allowlist  Allowlist
	Adapter    Adapter
	Audit      AuditRecorder
}

// Server exposes only the generated privileged v1 service over an externally
// supplied Unix listener.
type Server struct {
	privilegedv1.UnimplementedPrivilegedHelperServiceServer
	grpcServer *grpc.Server
	allowlist  Allowlist
	adapter    Adapter
	audit      AuditRecorder
	registry   *idempotencyRegistry
}

func NewServer(config ServerConfig) (*Server, error) {
	if config.Adapter == nil {
		return nil, errors.New("privileged adapter is required")
	}
	if config.Audit == nil {
		return nil, errors.New("audit recorder is required")
	}
	server := &Server{
		allowlist: config.Allowlist,
		adapter:   config.Adapter,
		audit:     config.Audit,
		registry:  newIdempotencyRegistry(idempotencyLimit, idempotencyTTL),
	}
	statsHandler := &auditStatsHandler{recorder: config.Audit}
	server.grpcServer = grpc.NewServer(
		grpc.Creds(NewServerTransportCredentials(config.PeerPolicy, config.Audit)),
		grpc.MaxRecvMsgSize(MaxMessageBytes),
		grpc.MaxSendMsgSize(MaxMessageBytes),
		grpc.MaxConcurrentStreams(MaxConcurrentRPCs),
		grpc.MaxHeaderListSize(8<<10),
		grpc.ConnectionTimeout(3*time.Second),
		grpc.StatsHandler(statsHandler),
		grpc.ChainUnaryInterceptor(server.authorizeAndAudit),
		grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
			return status.Error(codes.Unimplemented, "unknown-method")
		}),
	)
	privilegedv1.RegisterPrivilegedHelperServiceServer(server.grpcServer, server)
	return server, nil
}

func (s *Server) Serve(listener net.Listener) error {
	return s.grpcServer.Serve(newLimitedListener(listener, MaxConnections))
}

func (s *Server) GracefulStop() { s.grpcServer.GracefulStop() }

func (s *Server) Stop() { s.grpcServer.Stop() }

func (s *Server) GetStatus(ctx context.Context, request *privilegedv1.GetStatusRequest) (*privilegedv1.GetStatusResponse, error) {
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid-status-request")
	}
	capabilityStates := s.adapter.Capabilities()
	operations := []privilegedv1.PrivilegedOperation{
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_ADDRESS,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NFTABLES_POLICY,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_CONTAINER_LIFECYCLE,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NETWORK_NAMESPACE,
	}
	capabilities := make([]*privilegedv1.Capability, 0, len(operations))
	for _, operation := range operations {
		state := capabilityStates[operation]
		reason := "adapter-available"
		if state == privilegedv1.CapabilityState_CAPABILITY_STATE_UNSPECIFIED {
			state = privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED
		}
		if state == privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED {
			reason = unsupportedReason
		}
		capabilities = append(capabilities, &privilegedv1.Capability{
			Operation:  operation,
			State:      state,
			ReasonCode: reason,
		})
	}
	return &privilegedv1.GetStatusResponse{ApiVersion: APIVersion, Capabilities: capabilities}, nil
}

func (s *Server) EnsureAddress(ctx context.Context, request *privilegedv1.EnsureAddressRequest) (*privilegedv1.EnsureAddressResponse, error) {
	if err := s.allowlist.validateAddress(request); err != nil {
		return nil, rpcError(err)
	}
	result, err := s.runIdempotent(ctx, request.GetRequestId(), privilegedv1.PrivilegedHelperService_EnsureAddress_FullMethodName, request, func(ctx context.Context) (AdapterResult, error) {
		return s.adapter.EnsureAddress(ctx, AddressOperation{
			InterfaceName: request.GetInterfaceName(),
			AddressPrefix: request.GetAddressPrefix(),
			DesiredState:  request.GetDesiredState(),
		})
	})
	if err != nil {
		return nil, err
	}
	return &privilegedv1.EnsureAddressResponse{Result: operationResult(request.GetRequestId(), result)}, nil
}

func (s *Server) ApplyNftablesPolicy(ctx context.Context, request *privilegedv1.ApplyNftablesPolicyRequest) (*privilegedv1.ApplyNftablesPolicyResponse, error) {
	if err := s.allowlist.validateNftables(request); err != nil {
		return nil, rpcError(err)
	}
	result, err := s.runIdempotent(ctx, request.GetRequestId(), privilegedv1.PrivilegedHelperService_ApplyNftablesPolicy_FullMethodName, request, func(ctx context.Context) (AdapterResult, error) {
		return s.adapter.ApplyNftablesPolicy(ctx, NftablesOperation{
			NamespaceName: request.GetNamespaceName(),
			Profile:       request.GetProfile(),
		})
	})
	if err != nil {
		return nil, err
	}
	return &privilegedv1.ApplyNftablesPolicyResponse{Result: operationResult(request.GetRequestId(), result)}, nil
}

func (s *Server) ReconcileContainer(ctx context.Context, request *privilegedv1.ReconcileContainerRequest) (*privilegedv1.ReconcileContainerResponse, error) {
	if err := s.allowlist.validateContainer(request); err != nil {
		return nil, rpcError(err)
	}
	result, err := s.runIdempotent(ctx, request.GetRequestId(), privilegedv1.PrivilegedHelperService_ReconcileContainer_FullMethodName, request, func(ctx context.Context) (AdapterResult, error) {
		return s.adapter.ReconcileContainer(ctx, ContainerOperation{
			WorkloadID:   request.GetWorkloadId(),
			DesiredState: request.GetDesiredState(),
		})
	})
	if err != nil {
		return nil, err
	}
	return &privilegedv1.ReconcileContainerResponse{Result: operationResult(request.GetRequestId(), result)}, nil
}

func (s *Server) EnsureNetworkNamespace(ctx context.Context, request *privilegedv1.EnsureNetworkNamespaceRequest) (*privilegedv1.EnsureNetworkNamespaceResponse, error) {
	if err := s.allowlist.validateNamespace(request); err != nil {
		return nil, rpcError(err)
	}
	result, err := s.runIdempotent(ctx, request.GetRequestId(), privilegedv1.PrivilegedHelperService_EnsureNetworkNamespace_FullMethodName, request, func(ctx context.Context) (AdapterResult, error) {
		return s.adapter.EnsureNetworkNamespace(ctx, NamespaceOperation{
			NamespaceName: request.GetNamespaceName(),
			DesiredState:  request.GetDesiredState(),
		})
	})
	if err != nil {
		return nil, err
	}
	return &privilegedv1.EnsureNetworkNamespaceResponse{Result: operationResult(request.GetRequestId(), result)}, nil
}

func (s *Server) runIdempotent(
	ctx context.Context,
	requestID string,
	method string,
	request proto.Message,
	operation func(context.Context) (AdapterResult, error),
) (AdapterResult, error) {
	fingerprint, err := requestFingerprint(method, request)
	if err != nil {
		return AdapterResult{}, status.Error(codes.InvalidArgument, "request-fingerprint-failed")
	}
	result, err := s.registry.do(ctx, requestID, fingerprint, operation)
	if err != nil {
		return AdapterResult{}, rpcError(err)
	}
	if !validAdapterResult(result) {
		return AdapterResult{}, status.Error(codes.Internal, "invalid-adapter-result")
	}
	return result, nil
}

func validAdapterResult(result AdapterResult) bool {
	switch result.Outcome {
	case privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED,
		privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED,
		privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED:
		return reasonCodePattern.MatchString(result.ReasonCode)
	default:
		return false
	}
}

func operationResult(requestID string, result AdapterResult) *privilegedv1.OperationResult {
	return &privilegedv1.OperationResult{
		RequestId:  requestID,
		Outcome:    result.Outcome,
		ReasonCode: result.ReasonCode,
	}
}

func rpcError(err error) error {
	if violation, ok := asViolation(err); ok {
		return status.Error(violation.Code, violation.ReasonCode)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		return status.Error(codes.AlreadyExists, ErrIdempotencyConflict.Error())
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	return status.Error(codes.Internal, "adapter-failure")
}

func (s *Server) authorizeAndAudit(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
	operationCtx, cancel := boundedContext(ctx, operationTimeout)
	defer cancel()

	requestID, fingerprint := requestAuditMetadata(info.FullMethod, request)
	peerCredentials := peerCredentialsFromContext(operationCtx)
	if !s.allowlist.allowsMethod(info.FullMethod) {
		err = status.Error(codes.PermissionDenied, "method-not-allowlisted")
	} else {
		response, err = handler(operationCtx, request)
	}
	outcome, reason := auditOutcome(response, err)
	s.audit.Record(operationCtx, AuditEvent{
		Event:              "rpc-operation",
		Method:             safeAuditMethod(info.FullMethod),
		RequestID:          requestID,
		RequestFingerprint: fingerprint,
		Outcome:            outcome,
		ReasonCode:         reason,
		PeerPID:            peerCredentials.PID,
		PeerUID:            peerCredentials.UID,
		PeerGID:            peerCredentials.GID,
	})
	if state, ok := operationCtx.Value(rpcAuditStateKey{}).(*rpcAuditState); ok {
		state.recorded.Store(true)
	}
	return response, err
}

func boundedContext(ctx context.Context, maximum time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= maximum {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, maximum)
}

func requestAuditMetadata(method string, request any) (string, string) {
	message, ok := request.(proto.Message)
	if !ok || message == nil {
		return "", ""
	}
	fingerprint, _ := requestFingerprint(method, message)
	var requestID string
	switch typed := request.(type) {
	case *privilegedv1.EnsureAddressRequest:
		requestID = typed.GetRequestId()
	case *privilegedv1.ApplyNftablesPolicyRequest:
		requestID = typed.GetRequestId()
	case *privilegedv1.ReconcileContainerRequest:
		requestID = typed.GetRequestId()
	case *privilegedv1.EnsureNetworkNamespaceRequest:
		requestID = typed.GetRequestId()
	}
	if !requestIDPattern.MatchString(requestID) {
		requestID = ""
	}
	return requestID, fingerprint
}

func safeAuditMethod(method string) string {
	if auditRPCMethodPattern.MatchString(method) {
		return method
	}
	return "unknown"
}

func requestFingerprint(method string, message proto.Message) (string, error) {
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(method))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(encoded)
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func auditOutcome(response any, err error) (string, string) {
	if err != nil {
		code := status.Code(err)
		outcome := "failed"
		switch code {
		case codes.InvalidArgument, codes.PermissionDenied, codes.AlreadyExists,
			codes.Unimplemented, codes.ResourceExhausted, codes.Unauthenticated:
			outcome = "rejected"
		case codes.Canceled, codes.DeadlineExceeded:
			outcome = "cancelled"
		}
		message := status.Convert(err).Message()
		if reasonCodePattern.MatchString(message) {
			return outcome, message
		}
		return outcome, "grpc-" + strings.ToLower(code.String())
	}
	result := responseOperationResult(response)
	if result == nil {
		return "accepted", "ok"
	}
	if result.GetOutcome() == privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED {
		return "unsupported", result.GetReasonCode()
	}
	return "accepted", result.GetReasonCode()
}

func responseOperationResult(response any) *privilegedv1.OperationResult {
	switch typed := response.(type) {
	case *privilegedv1.EnsureAddressResponse:
		return typed.GetResult()
	case *privilegedv1.ApplyNftablesPolicyResponse:
		return typed.GetResult()
	case *privilegedv1.ReconcileContainerResponse:
		return typed.GetResult()
	case *privilegedv1.EnsureNetworkNamespaceResponse:
		return typed.GetResult()
	default:
		return nil
	}
}

func peerCredentialsFromContext(ctx context.Context) PeerCredentials {
	peerValue, ok := peer.FromContext(ctx)
	if !ok || peerValue.AuthInfo == nil {
		return PeerCredentials{}
	}
	switch auth := peerValue.AuthInfo.(type) {
	case PeerAuthInfo:
		return auth.Credentials
	case *PeerAuthInfo:
		return auth.Credentials
	default:
		return PeerCredentials{}
	}
}

type rpcAuditStateKey struct{}

type rpcAuditState struct {
	method   string
	recorded atomic.Bool
}

type auditStatsHandler struct {
	recorder AuditRecorder
}

func (h *auditStatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	return context.WithValue(ctx, rpcAuditStateKey{}, &rpcAuditState{method: safeAuditMethod(info.FullMethodName)})
}

func (h *auditStatsHandler) HandleRPC(ctx context.Context, event stats.RPCStats) {
	ended, ok := event.(*stats.End)
	if !ok {
		return
	}
	state, ok := ctx.Value(rpcAuditStateKey{}).(*rpcAuditState)
	if !ok || state.recorded.Swap(true) {
		return
	}
	peerCredentials := peerCredentialsFromContext(ctx)
	outcome, reason := auditOutcome(nil, ended.Error)
	h.recorder.Record(ctx, AuditEvent{
		Event:      "rpc-operation",
		Method:     state.method,
		Outcome:    outcome,
		ReasonCode: reason,
		PeerPID:    peerCredentials.PID,
		PeerUID:    peerCredentials.UID,
		PeerGID:    peerCredentials.GID,
	})
}

func (h *auditStatsHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}

func (*auditStatsHandler) HandleConn(context.Context, stats.ConnStats) {}

var _ privilegedv1.PrivilegedHelperServiceServer = (*Server)(nil)
