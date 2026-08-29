package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
)

// DeviceInventoryService is the narrow P1-W11 read-only device projection.
type DeviceInventoryService interface {
	ListDevices(context.Context, string) ([]devices.InventoryDevice, error)
	Device(context.Context, string, string) (devices.InventoryDevice, error)
}

func WithDeviceInventoryService(service DeviceInventoryService) Option {
	return func(server *Server) { server.deviceInventoryService = service }
}

func (s *Server) handleListDevices(writer http.ResponseWriter, request *http.Request) {
	environmentID := request.PathValue("environmentId")
	if _, ok := s.authorizeDeviceAdmin(request, environmentID, false); !ok {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.deviceInventoryService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_inventory_unavailable")
		return
	}
	items, err := s.deviceInventoryService.ListDevices(request.Context(), environmentID)
	if err != nil {
		s.writeDeviceInventoryError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"devices": items})
}

func (s *Server) handleGetDevice(writer http.ResponseWriter, request *http.Request) {
	environmentID := request.PathValue("environmentId")
	if _, ok := s.authorizeDeviceAdmin(request, environmentID, false); !ok {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.deviceInventoryService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "device_inventory_unavailable")
		return
	}
	item, err := s.deviceInventoryService.Device(
		request.Context(), environmentID, request.PathValue("deviceId"),
	)
	if err != nil {
		s.writeDeviceInventoryError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"device": item})
}

func (s *Server) writeDeviceInventoryError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, devices.ErrInvalidInput):
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, devices.ErrNotFound):
		writeStatus(writer, http.StatusNotFound, "not_found")
	default:
		s.logger.ErrorContext(request.Context(), "device inventory read failed")
		writeStatus(writer, http.StatusInternalServerError, "internal_error")
	}
}
