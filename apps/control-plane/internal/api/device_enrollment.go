package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
)

const (
	maximumDeviceRequestBytes  = 64 << 10
	maximumEnrollmentTokenSize = 64
	csrfHeaderName             = "X-CSRF-Token"
)

var errDeviceAuthorizationRequired = errors.New("device administration authorization is required")

// DeviceService is the P1-W4 application boundary used by HTTP and P1-W5.
type DeviceService interface {
	CreateEnrollmentToken(context.Context, string, string, string) (devices.EnrollmentToken, error)
	CreateReenrollmentToken(context.Context, string, string, string) (devices.EnrollmentToken, error)
	ListEnrollmentTokens(context.Context, string) ([]devices.TokenSummary, error)
	RevokeEnrollmentToken(context.Context, string, string, string) error
	Enroll(context.Context, string, string, []byte) (devices.EnrollmentResult, error)
	Rotate(context.Context, string, string, []byte) (devices.EnrollmentResult, error)
	SetDeviceState(context.Context, string, string, devices.DeviceState, string) error
	VerifyCertificate(context.Context, *x509.Certificate) (string, string, error)
}

// DeviceAdminAuthorizer is the P1-W2 seam. Mutation inputs include CSRF and
// origin values; production remains fail-closed until P1-W2 supplies it.
type DeviceAdminAuthorizer interface {
	AuthorizeDeviceAdmin(context.Context, string, string, string, string, bool) (string, error)
}

type DeviceAdminAuthorizerFunc func(context.Context, string, string, string, string, bool) (string, error)

func (fn DeviceAdminAuthorizerFunc) AuthorizeDeviceAdmin(
	ctx context.Context,
	session, environmentID, csrf, origin string,
	mutation bool,
) (string, error) {
	if fn == nil {
		return "", errDeviceAuthorizationRequired
	}
	return fn(ctx, session, environmentID, csrf, origin, mutation)
}

type denyDeviceAdminAuthorizer struct{}

func (denyDeviceAdminAuthorizer) AuthorizeDeviceAdmin(context.Context, string, string, string, string, bool) (string, error) {
	return "", errDeviceAuthorizationRequired
}

// WithDeviceService installs the product enrollment implementation.
func WithDeviceService(service DeviceService) Option {
	return func(server *Server) { server.deviceService = service }
}

// WithDeviceAdminAuthorizer supplies the P1-W2 session/CSRF integration.
func WithDeviceAdminAuthorizer(authorizer DeviceAdminAuthorizer) Option {
	return func(server *Server) {
		if authorizer != nil {
			server.deviceAuthorizer = authorizer
		}
	}
}

// WithTLSFiles enables the production TLS 1.3 listener and optional product
// device-certificate verification used by the rotation endpoint.
func WithTLSFiles(certificateFile, privateKeyFile string, deviceCAPEM []byte) Option {
	return func(server *Server) {
		server.tlsCertificateFile = certificateFile
		server.tlsPrivateKeyFile = privateKeyFile
		server.deviceClientCAPEM = append([]byte(nil), deviceCAPEM...)
	}
}

func (s *Server) handleCreateEnrollmentToken(writer http.ResponseWriter, request *http.Request) {
	environmentID := request.PathValue("environmentId")
	actorID, ok := s.authorizeDeviceAdmin(request, environmentID, true)
	if !ok {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.deviceService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_service_unavailable")
		return
	}
	var input struct {
		DeviceName string `json:"device_name"`
	}
	if !decodeBoundedJSON(writer, request, &input) {
		return
	}
	token, err := s.deviceService.CreateEnrollmentToken(request.Context(), environmentID, input.DeviceName, actorID)
	if err != nil {
		writeDeviceError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, token)
}

func (s *Server) handleListEnrollmentTokens(writer http.ResponseWriter, request *http.Request) {
	environmentID := request.PathValue("environmentId")
	if _, ok := s.authorizeDeviceAdmin(request, environmentID, false); !ok {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.deviceService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_service_unavailable")
		return
	}
	tokens, err := s.deviceService.ListEnrollmentTokens(request.Context(), environmentID)
	if err != nil {
		writeDeviceError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"tokens": tokens})
}

func (s *Server) handleRevokeEnrollmentToken(writer http.ResponseWriter, request *http.Request) {
	environmentID := request.PathValue("environmentId")
	actorID, ok := s.authorizeDeviceAdmin(request, environmentID, true)
	if !ok {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.deviceService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_service_unavailable")
		return
	}
	if err := s.deviceService.RevokeEnrollmentToken(request.Context(), environmentID, request.PathValue("tokenId"), actorID); err != nil {
		writeDeviceError(writer, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateReenrollmentToken(writer http.ResponseWriter, request *http.Request) {
	environmentID := request.PathValue("environmentId")
	actorID, ok := s.authorizeDeviceAdmin(request, environmentID, true)
	if !ok {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.deviceService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_service_unavailable")
		return
	}
	token, err := s.deviceService.CreateReenrollmentToken(
		request.Context(), environmentID, request.PathValue("deviceId"), actorID,
	)
	if err != nil {
		writeDeviceError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, token)
}

func (s *Server) handleEnrollDevice(writer http.ResponseWriter, request *http.Request) {
	if request.TLS == nil {
		writeStatus(writer, http.StatusUpgradeRequired, "tls_required")
		return
	}
	token, ok := singleBearerToken(request)
	if !ok {
		writeStatus(writer, http.StatusUnauthorized, "enrollment_denied")
		return
	}
	if s.deviceService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_service_unavailable")
		return
	}
	var input struct {
		CSRPEM string `json:"csr_pem"`
	}
	if !decodeBoundedJSON(writer, request, &input) {
		return
	}
	source := request.RemoteAddr
	if host, _, splitErr := net.SplitHostPort(request.RemoteAddr); splitErr == nil {
		source = host
	}
	if source == "" {
		source = "unknown-source"
	}
	result, err := s.deviceService.Enroll(request.Context(), source, token, []byte(input.CSRPEM))
	if err != nil {
		if errors.Is(err, devices.ErrEnrollmentRateLimited) {
			writer.Header().Set("Retry-After", "60")
			writeStatus(writer, http.StatusTooManyRequests, "enrollment_denied")
			return
		}
		writeStatus(writer, http.StatusUnauthorized, "enrollment_denied")
		return
	}
	writeJSON(writer, http.StatusCreated, enrollmentResponse(result))
}

func (s *Server) handleRotateDeviceCertificate(writer http.ResponseWriter, request *http.Request) {
	if request.TLS == nil || len(request.TLS.PeerCertificates) < 1 || s.deviceService == nil {
		writeStatus(writer, http.StatusUnauthorized, "certificate_denied")
		return
	}
	deviceID, serial, err := s.deviceService.VerifyCertificate(request.Context(), request.TLS.PeerCertificates[0])
	if err != nil {
		writeStatus(writer, http.StatusUnauthorized, "certificate_denied")
		return
	}
	var input struct {
		CSRPEM string `json:"csr_pem"`
	}
	if !decodeBoundedJSON(writer, request, &input) {
		return
	}
	result, err := s.deviceService.Rotate(request.Context(), deviceID, serial, []byte(input.CSRPEM))
	if err != nil {
		writeDeviceError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, enrollmentResponse(result))
}

func (s *Server) handleDisableDevice(writer http.ResponseWriter, request *http.Request) {
	s.handleSetDeviceState(writer, request, devices.DeviceDisabled)
}

func (s *Server) handleRevokeDevice(writer http.ResponseWriter, request *http.Request) {
	s.handleSetDeviceState(writer, request, devices.DeviceRevoked)
}

func (s *Server) handleSetDeviceState(writer http.ResponseWriter, request *http.Request, state devices.DeviceState) {
	environmentID := request.PathValue("environmentId")
	actorID, ok := s.authorizeDeviceAdmin(request, environmentID, true)
	if !ok {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.deviceService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_service_unavailable")
		return
	}
	if err := s.deviceService.SetDeviceState(request.Context(), environmentID, request.PathValue("deviceId"), state, actorID); err != nil {
		writeDeviceError(writer, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) authorizeDeviceAdmin(request *http.Request, environmentID string, mutation bool) (string, bool) {
	session, ok := singleAuditSession(request)
	if !ok || s.deviceAuthorizer == nil {
		return "", false
	}
	csrf, origin := "", ""
	if mutation {
		csrf = request.Header.Get(csrfHeaderName)
		origin = request.Header.Get("Origin")
		if csrf == "" || origin == "" || len(csrf) > 512 || len(origin) > 2048 {
			return "", false
		}
	}
	actorID, err := s.deviceAuthorizer.AuthorizeDeviceAdmin(request.Context(), session, environmentID, csrf, origin, mutation)
	return actorID, err == nil && actorID != "" && len(actorID) <= 256
}

func singleBearerToken(request *http.Request) (string, bool) {
	values := request.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(values[0], "Bearer ")
	return token, len(token) > 0 && len(token) <= maximumEnrollmentTokenSize && strings.TrimSpace(token) == token
}

func decodeBoundedJSON(writer http.ResponseWriter, request *http.Request, destination any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, maximumDeviceRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func enrollmentResponse(result devices.EnrollmentResult) map[string]any {
	return map[string]any{
		"contract_version":   "guardian.device.v1",
		"device_id":          result.Certificate.DeviceID,
		"environment_id":     result.Certificate.EnvironmentID,
		"certificate_serial": result.Certificate.Serial,
		"certificate_pem":    string(result.Certificate.PEM),
		"ca_certificate_pem": string(result.CAPEM),
		"not_before":         result.Certificate.NotBefore,
		"not_after":          result.Certificate.NotAfter,
	}
}

func writeDeviceError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, devices.ErrInvalidInput):
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, devices.ErrTokenInvalid), errors.Is(err, devices.ErrTokenExpired),
		errors.Is(err, devices.ErrTokenConsumed), errors.Is(err, devices.ErrTokenRevoked),
		errors.Is(err, devices.ErrDeviceDisabled), errors.Is(err, devices.ErrDeviceRevoked),
		errors.Is(err, devices.ErrCertificateRevoked), errors.Is(err, devices.ErrCertificateStale):
		writeStatus(writer, http.StatusConflict, "operation_denied")
	default:
		writeStatus(writer, http.StatusInternalServerError, "internal_error")
	}
}
