// Package api owns the public REST API boundary and minimal HTTP server.
package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Readiness is the narrow health dependency exposed to the HTTP layer.
type Readiness interface {
	Ready(context.Context) error
}

// Server is a start-once HTTP lifecycle component.
type Server struct {
	address                string
	readiness              Readiness
	auditReader            audit.Reader
	auditAuthorizer        AuditAuthorizer
	deviceService          DeviceService
	deviceInventoryService DeviceInventoryService
	deviceAuthorizer       DeviceAdminAuthorizer
	environmentService     EnvironmentService
	environmentAuth        EnvironmentAuthorizer
	decoyService           DecoyService
	healthService          HealthService
	authService            AuthService
	logger                 *slog.Logger
	http                   *http.Server
	listener               net.Listener
	errors                 chan error
	startOnce              sync.Once
	startErr               error
	tlsCertificateFile     string
	tlsPrivateKeyFile      string
	deviceClientCAPEM      []byte
	webConsoleDirectory    string
}

// NewServer creates an instrumented server without binding a socket.
func NewServer(address string, readiness Readiness, logger *slog.Logger, options ...Option) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{
		address:          address,
		readiness:        readiness,
		auditAuthorizer:  denyAuditAuthorizer{},
		deviceAuthorizer: denyDeviceAdminAuthorizer{},
		environmentAuth:  denyEnvironmentAuthorizer{},
		logger:           logger,
		errors:           make(chan error, 1),
	}
	if reader, ok := readiness.(audit.Reader); ok {
		server.auditReader = reader
	}
	for _, option := range options {
		if option != nil {
			option(server)
		}
	}
	server.http = &http.Server{
		Addr:              address,
		Handler:           otelhttp.NewHandler(securityHeaders(server.routes()), "guardian.control-plane.http"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return server
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(writer http.ResponseWriter, _ *http.Request) {
		writeStatus(writer, http.StatusOK, "live")
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if s.readiness == nil || s.readiness.Ready(ctx) != nil {
			s.logger.WarnContext(request.Context(), "readiness check failed")
			writeStatus(writer, http.StatusServiceUnavailable, "not_ready")
			return
		}
		writeStatus(writer, http.StatusOK, "ready")
	})
	mux.HandleFunc("POST /v1/auth/bootstrap", s.handleBootstrap)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /v1/auth/session", s.handleSession)
	mux.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /v1/auth/csrf", s.handleReissueCSRF)
	mux.HandleFunc("GET /v1/auth/sessions", s.handleListSessions)
	mux.HandleFunc("DELETE /v1/auth/sessions/{sessionId}", s.handleRevokeSession)
	mux.HandleFunc("POST /v1/auth/password", s.handleChangePassword)
	mux.HandleFunc("GET /v1/audit/events", s.handleListAuditEvents)
	mux.HandleFunc("GET /v1/organization", s.handleGetOrganization)
	mux.HandleFunc("GET /v1/environments", s.handleListEnvironments)
	mux.HandleFunc("POST /v1/environments", s.handleCreateEnvironment)
	mux.HandleFunc("GET /v1/environments/{environmentId}", s.handleGetEnvironment)
	mux.HandleFunc("PATCH /v1/environments/{environmentId}", s.handleUpdateEnvironment)
	mux.HandleFunc("GET /v1/environments/{environmentId}/zones", s.handleListZones)
	mux.HandleFunc("POST /v1/environments/{environmentId}/zones", s.handleCreateZone)
	mux.HandleFunc("GET /v1/environments/{environmentId}/zones/{zoneId}", s.handleGetZone)
	mux.HandleFunc("PATCH /v1/environments/{environmentId}/zones/{zoneId}", s.handleUpdateZone)
	mux.HandleFunc("DELETE /v1/environments/{environmentId}/zones/{zoneId}", s.handleDeleteZone)
	mux.HandleFunc("GET /v1/environments/{environmentId}/decoys", s.handleListDecoys)
	mux.HandleFunc("POST /v1/environments/{environmentId}/decoys", s.handleCreateDecoy)
	mux.HandleFunc("GET /v1/environments/{environmentId}/decoys/{decoyId}", s.handleGetDecoy)
	mux.HandleFunc("PATCH /v1/environments/{environmentId}/decoys/{decoyId}", s.handleUpdateDecoy)
	mux.HandleFunc("DELETE /v1/environments/{environmentId}/decoys/{decoyId}", s.handleDeleteDecoy)
	mux.HandleFunc("POST /v1/environments/{environmentId}/decoys/{decoyId}/enable", s.handleEnableDecoy)
	mux.HandleFunc("POST /v1/environments/{environmentId}/decoys/{decoyId}/disable", s.handleDisableDecoy)
	mux.HandleFunc("POST /v1/environments/{environmentId}/enrollment-tokens", s.handleCreateEnrollmentToken)
	mux.HandleFunc("GET /v1/environments/{environmentId}/enrollment-tokens", s.handleListEnrollmentTokens)
	mux.HandleFunc("DELETE /v1/environments/{environmentId}/enrollment-tokens/{tokenId}", s.handleRevokeEnrollmentToken)
	mux.HandleFunc("POST /v1/environments/{environmentId}/devices/{deviceId}/re-enrollment-token", s.handleCreateReenrollmentToken)
	mux.HandleFunc("GET /v1/environments/{environmentId}/devices", s.handleListDevices)
	mux.HandleFunc("GET /v1/environments/{environmentId}/devices/{deviceId}", s.handleGetDevice)
	mux.HandleFunc("POST /v1/enrollments", s.handleEnrollDevice)
	mux.HandleFunc("POST /v1/device-certificates:rotate", s.handleRotateDeviceCertificate)
	mux.HandleFunc("POST /v1/environments/{environmentId}/devices/{deviceId}/disable", s.handleDisableDevice)
	mux.HandleFunc("POST /v1/environments/{environmentId}/devices/{deviceId}/revoke", s.handleRevokeDevice)
	mux.HandleFunc("GET /v1/environments/{environmentId}/health", s.handleEnvironmentHealth)
	mux.HandleFunc("GET /v1/devices/{deviceId}/health", s.handleDeviceHealth)
	if s.webConsoleDirectory != "" {
		mux.HandleFunc("GET /", s.handleWebConsole)
	}
	return mux
}

// permissionsPolicy denies every powerful browser feature. The console needs
// none of them, so an injected frame or a compromised dependency cannot reach
// a camera, a microphone, a location, or a serial port even if it tries
// (WCX-06 section 9.4, decision WC-D25).
const permissionsPolicy = "accelerometer=(), ambient-light-sensor=(), autoplay=(), battery=(), " +
	"bluetooth=(), camera=(), display-capture=(), encrypted-media=(), fullscreen=(), " +
	"gamepad=(), geolocation=(), gyroscope=(), hid=(), idle-detection=(), " +
	"local-fonts=(), magnetometer=(), microphone=(), midi=(), payment=(), " +
	"picture-in-picture=(), publickey-credentials-get=(), screen-wake-lock=(), " +
	"serial=(), speaker-selection=(), usb=(), xr-spatial-tracking=()"

// strictTransportSecurity pins HTTPS for a year, including subdomains.
//
// `preload` is deliberately absent. Submitting to the preload list is
// effectively irreversible for months, so it is an owner decision about a
// deployment topology rather than a header this middleware may take on its own.
const strictTransportSecurity = "max-age=31536000; includeSubDomains"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; connect-src 'self'; font-src 'self'; form-action 'self'; frame-ancestors 'none'; img-src 'self'; script-src 'self'; style-src 'self'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Permissions-Policy", permissionsPolicy)
		// Only over TLS. A development listener on plain HTTP that sent this
		// would pin the browser to HTTPS for a host that does not serve it,
		// locking the developer out of their own machine for a year.
		if request.TLS != nil {
			writer.Header().Set("Strict-Transport-Security", strictTransportSecurity)
		}
		next.ServeHTTP(writer, request)
	})
}

// Handler exposes the production handler for bounded unit tests.
func (s *Server) Handler() http.Handler { return s.http.Handler }

// Start binds the configured listener and starts serving asynchronously.
func (s *Server) Start() error {
	s.startOnce.Do(func() {
		if s.tlsCertificateFile == "" {
			s.listener, s.startErr = net.Listen("tcp", s.address)
		} else {
			s.listener, s.startErr = s.listenTLS()
		}
		if s.startErr != nil {
			return
		}
		go func() {
			err := s.http.Serve(s.listener)
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			s.errors <- err
			close(s.errors)
		}()
	})
	return s.startErr
}

// Address returns the bound address after Start.
func (s *Server) Address() string {
	if s.listener == nil {
		return s.address
	}
	return s.listener.Addr().String()
}

// Errors reports a terminal serving error.
func (s *Server) Errors() <-chan error { return s.errors }

// Shutdown drains active HTTP requests.
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func (s *Server) listenTLS() (net.Listener, error) {
	identity, err := tls.LoadX509KeyPair(s.tlsCertificateFile, s.tlsPrivateKeyFile)
	if err != nil {
		return nil, errors.New("load Control Plane TLS identity")
	}
	clientRoots := x509.NewCertPool()
	if len(s.deviceClientCAPEM) == 0 || !clientRoots.AppendCertsFromPEM(s.deviceClientCAPEM) {
		return nil, errors.New("load device client CA")
	}
	return tls.Listen("tcp", s.address, &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{identity},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    clientRoots,
	})
}
