package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
)

type HealthService interface {
	Device(context.Context, string) (health.View, error)
	Environment(context.Context, string) (health.View, error)
}

func WithHealthService(service HealthService) Option {
	return func(server *Server) { server.healthService = service }
}

func (s *Server) handleDeviceHealth(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.healthAvailable(writer) {
		return
	}
	view, err := s.healthService.Device(request.Context(), request.PathValue("deviceId"))
	if err != nil {
		s.writeHealthError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, view)
}

func (s *Server) handleEnvironmentHealth(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.healthAvailable(writer) {
		return
	}
	view, err := s.healthService.Environment(request.Context(), request.PathValue("environmentId"))
	if err != nil {
		s.writeHealthError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, view)
}

func (s *Server) healthAvailable(writer http.ResponseWriter) bool {
	if s.healthService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "health_service_unavailable")
		return false
	}
	return true
}

func (s *Server) writeHealthError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, health.ErrNotFound) {
		writeStatus(writer, http.StatusNotFound, "not_found")
		return
	}
	s.logger.ErrorContext(request.Context(), "health read failed")
	writeStatus(writer, http.StatusInternalServerError, "internal_error")
}
