package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/config"
	"forgejo.org/opcp-serverless-brik/pkg/executor"
	"forgejo.org/opcp-serverless-brik/pkg/health"
)

// Server is the HTTP server for the worker API.
type Server struct {
	executor  *executor.Executor
	health    *health.Checker
	config    *config.Config
	whitelist []string
	server    *http.Server
}

// NewServer creates a new API server with the given dependencies.
func NewServer(exec *executor.Executor, hc *health.Checker, cfg *config.Config, whitelist []string) *Server {
	s := &Server{
		executor:  exec,
		health:    hc,
		config:    cfg,
		whitelist: whitelist,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", s.handleSubmitJob)
	mux.HandleFunc("GET /jobs/{id}", s.handleGetResult)
	mux.HandleFunc("GET /jobs/{id}/logs", s.handleStreamLogs)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)

	s.server = &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // Disabled for SSE streaming
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start begins listening for HTTP requests. It blocks until the server is
// shut down or an error occurs.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}

	slog.Info("server listening", "addr", s.server.Addr)

	// Run server in a goroutine so we can react to context cancellation.
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the HTTP server, waiting for in-flight
// requests to complete within the given context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.server.Shutdown(shutdownCtx)
}
