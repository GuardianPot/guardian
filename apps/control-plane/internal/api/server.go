// Package api owns the public REST API boundary and minimal HTTP server.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Readiness is the narrow health dependency exposed to the HTTP layer.
type Readiness interface {
	Ready(context.Context) error
}

// Server is a start-once HTTP lifecycle component.
type Server struct {
	address   string
	readiness Readiness
	logger    *slog.Logger
	http      *http.Server
	listener  net.Listener
	errors    chan error
	startOnce sync.Once
	startErr  error
}

// NewServer creates an instrumented server without binding a socket.
func NewServer(address string, readiness Readiness, logger *slog.Logger) *Server {
	server := &Server{
		address:   address,
		readiness: readiness,
		logger:    logger,
		errors:    make(chan error, 1),
	}
	server.http = &http.Server{
		Addr:              address,
		Handler:           otelhttp.NewHandler(server.routes(), "guardian.control-plane.http"),
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
	return mux
}

// Handler exposes the production handler for bounded unit tests.
func (s *Server) Handler() http.Handler { return s.http.Handler }

// Start binds the configured listener and starts serving asynchronously.
func (s *Server) Start() error {
	s.startOnce.Do(func() {
		s.listener, s.startErr = net.Listen("tcp", s.address)
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

func writeStatus(writer http.ResponseWriter, code int, status string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(code)
	_ = json.NewEncoder(writer).Encode(map[string]string{"status": status})
}
