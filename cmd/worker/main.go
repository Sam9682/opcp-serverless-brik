package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/api"
	"forgejo.org/opcp-serverless-brik/pkg/config"
	"forgejo.org/opcp-serverless-brik/pkg/executor"
	"forgejo.org/opcp-serverless-brik/pkg/health"
	"forgejo.org/opcp-serverless-brik/pkg/runtime"
)

func main() {
	// Set up structured logging with JSON handler to stdout.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("configuration loaded",
		"listen_addr", cfg.ListenAddr,
		"runtime", cfg.Runtime,
		"runtime_socket", cfg.RuntimeSocket,
	)

	// Create runtime from configuration.
	rt, err := runtime.New(cfg)
	if err != nil {
		slog.Error("failed to create runtime", "error", err)
		os.Exit(1)
	}

	// Verify runtime connectivity (requirement 6.5).
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if err := rt.Ping(pingCtx); err != nil {
		slog.Error("container runtime is unreachable", "runtime", cfg.Runtime, "socket", cfg.RuntimeSocket, "error", err)
		os.Exit(1)
	}

	slog.Info("container runtime connected", "runtime", cfg.Runtime)

	// Create executor.
	exec := executor.NewExecutor(rt, cfg)

	// Create health checker.
	hc := health.NewChecker(rt, 3*time.Second)

	// Create HTTP server.
	server := api.NewServer(exec, hc, cfg, cfg.RegistryWhitelist)

	// Set up signal handling for graceful shutdown (SIGTERM, SIGINT).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Start HTTP server — blocks until shutdown signal or error.
	if err := server.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("worker shut down gracefully")
}
