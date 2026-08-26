package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/auth"
)

const authSessionCookieName = "__Host-guardian_session"

// AuthService is the local owner-authentication boundary exposed by the API.
type AuthService interface {
	Bootstrap(context.Context, string, string, string) (auth.BootstrapResult, error)
	LoginTOTP(context.Context, string, string, string, string) (auth.SessionCredentials, error)
	LoginRecovery(context.Context, string, string, string, string) (auth.SessionCredentials, error)
	AuthorizeRead(context.Context, string) (auth.Session, error)
	Logout(context.Context, string, string, string) error
	Sessions(context.Context, string) ([]auth.Session, error)
	RevokeSession(context.Context, string, string, string, string) error
	ChangePassword(context.Context, string, string, string, string, string) (auth.SessionCredentials, error)
}

// WithAuthService installs the P1-W2 local authentication implementation.
func WithAuthService(service AuthService) Option {
	return func(server *Server) { server.authService = service }
}

func (s *Server) handleBootstrap(writer http.ResponseWriter, request *http.Request) {
	if !s.requireTLS(writer, request) {
		return
	}
	token, ok := singleBearerToken(request)
	if !ok || s.authService == nil {
		writeStatus(writer, http.StatusUnauthorized, "bootstrap_denied")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeBoundedJSON(writer, request, &input) {
		return
	}
	result, err := s.authService.Bootstrap(request.Context(), token, input.Username, input.Password)
	if err != nil {
		writeAuthError(writer, err, "bootstrap_denied")
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (s *Server) handleLogin(writer http.ResponseWriter, request *http.Request) {
	if !s.requireTLS(writer, request) {
		return
	}
	if s.authService == nil {
		writeStatus(writer, http.StatusUnauthorized, "authentication_denied")
		return
	}
	var input struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		TOTPCode     string `json:"totp_code,omitempty"`
		RecoveryCode string `json:"recovery_code,omitempty"`
	}
	if !decodeBoundedJSON(writer, request, &input) {
		return
	}
	if (input.TOTPCode == "") == (input.RecoveryCode == "") {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return
	}
	var (
		credentials auth.SessionCredentials
		err         error
	)
	if input.TOTPCode != "" {
		credentials, err = s.authService.LoginTOTP(request.Context(), input.Username, input.Password, input.TOTPCode, requestSource(request))
	} else {
		credentials, err = s.authService.LoginRecovery(request.Context(), input.Username, input.Password, input.RecoveryCode, requestSource(request))
	}
	if err != nil {
		writeAuthError(writer, err, "authentication_denied")
		return
	}
	writeSessionCredentials(writer, http.StatusOK, credentials)
}

func (s *Server) handleSession(writer http.ResponseWriter, request *http.Request) {
	if !s.requireTLS(writer, request) {
		return
	}
	sessionToken, ok := authCookie(request)
	if !ok || s.authService == nil {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	session, err := s.authService.AuthorizeRead(request.Context(), sessionToken)
	if err != nil {
		writeAuthError(writer, err, "unauthorized")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) handleLogout(writer http.ResponseWriter, request *http.Request) {
	if !s.requireTLS(writer, request) {
		return
	}
	session, csrf, origin, ok := authMutation(request)
	if !ok || s.authService == nil {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.authService.Logout(request.Context(), session, csrf, origin); err != nil {
		writeAuthError(writer, err, "unauthorized")
		return
	}
	clearAuthCookie(writer)
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSessions(writer http.ResponseWriter, request *http.Request) {
	if !s.requireTLS(writer, request) {
		return
	}
	sessionToken, ok := authCookie(request)
	if !ok || s.authService == nil {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessions, err := s.authService.Sessions(request.Context(), sessionToken)
	if err != nil {
		writeAuthError(writer, err, "unauthorized")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"sessions": sessions})
}

func (s *Server) handleRevokeSession(writer http.ResponseWriter, request *http.Request) {
	if !s.requireTLS(writer, request) {
		return
	}
	session, csrf, origin, ok := authMutation(request)
	if !ok || s.authService == nil {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.authService.RevokeSession(request.Context(), session, csrf, origin, request.PathValue("sessionId")); err != nil {
		writeAuthError(writer, err, "unauthorized")
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleChangePassword(writer http.ResponseWriter, request *http.Request) {
	if !s.requireTLS(writer, request) {
		return
	}
	session, csrf, origin, ok := authMutation(request)
	if !ok || s.authService == nil {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeBoundedJSON(writer, request, &input) {
		return
	}
	credentials, err := s.authService.ChangePassword(request.Context(), session, csrf, origin, input.CurrentPassword, input.NewPassword)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidPassword) || errors.Is(err, auth.ErrInvalidInput) {
			writeStatus(writer, http.StatusBadRequest, "invalid_request")
			return
		}
		writeAuthError(writer, err, "unauthorized")
		return
	}
	writeSessionCredentials(writer, http.StatusOK, credentials)
}

func (s *Server) requireTLS(writer http.ResponseWriter, request *http.Request) bool {
	if request.TLS == nil {
		writeStatus(writer, http.StatusUpgradeRequired, "tls_required")
		return false
	}
	return true
}

func authCookie(request *http.Request) (string, bool) {
	cookies := request.CookiesNamed(authSessionCookieName)
	if len(cookies) != 1 || len(cookies[0].Value) != auth.OpaqueSecretLength || strings.TrimSpace(cookies[0].Value) != cookies[0].Value {
		return "", false
	}
	return cookies[0].Value, true
}

func authMutation(request *http.Request) (string, string, string, bool) {
	session, ok := authCookie(request)
	csrf, origin := request.Header.Get(csrfHeaderName), request.Header.Get("Origin")
	return session, csrf, origin, ok && len(csrf) == auth.OpaqueSecretLength && len(origin) <= 2048
}

func writeSessionCredentials(writer http.ResponseWriter, code int, credentials auth.SessionCredentials) {
	http.SetCookie(writer, &http.Cookie{
		Name: authSessionCookieName, Value: credentials.SessionToken, Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	writeJSON(writer, code, map[string]any{"csrf_token": credentials.CSRFToken, "session": credentials.Session})
}

func clearAuthCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name: authSessionCookieName, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}

func requestSource(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if request.RemoteAddr != "" && len(request.RemoteAddr) <= 256 {
		return request.RemoteAddr
	}
	return "unknown-source"
}

func writeAuthError(writer http.ResponseWriter, err error, denial string) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput), errors.Is(err, auth.ErrInvalidPassword):
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, auth.ErrRateLimited):
		writer.Header().Set("Retry-After", "60")
		writeStatus(writer, http.StatusTooManyRequests, denial)
	case auth.IsDenied(err), errors.Is(err, auth.ErrBootstrapUnavailable):
		writeStatus(writer, http.StatusUnauthorized, denial)
	default:
		writeStatus(writer, http.StatusInternalServerError, "internal_error")
	}
}
