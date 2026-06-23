// Async Worker is the background job processor for the Lanyu Token Gateway.
// It consumes tasks from the transactional outbox (PostgreSQL) and/or NATS
// JetStream, handling payment settlement, channel health checks, credential
// auto-refresh, usage aggregation, webhook delivery, and other asynchronous
// workloads.
//
// It is the runtime entry point for the Lanyu Token Gateway async worker.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/packages/config"
	"github.com/seaveywong/lanyu-token-gateway/packages/observability"
)

func main() {
	// --- Load configuration ---
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --- Initialize observability ---
	ctx := context.Background()
	obsCfg := observability.ObservabilityConfig{
		ServiceName:    cfg.Observability.ServiceName,
		ServiceVersion: cfg.Observability.ServiceVersion,
		OTLPEndpoint:   cfg.Observability.OTLPEndpoint,
		LogLevel:       cfg.Observability.LogLevel,
		LogFormat:      cfg.Observability.LogFormat,
	}
	shutdown, err := observability.Init(ctx, obsCfg)
	if err != nil {
		slog.Error("failed to initialize observability", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			slog.Error("observability shutdown error", slog.String("error", err.Error()))
		}
	}()

	logger := observability.Logger()
	logger.Info("async-worker started")

	// --- Signal handling for graceful shutdown ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// --- Worker loop (placeholder) ---
	// In production this would:
	//   - Poll the outbox_events table for pending tasks.
	//   - Consume NATS JetStream messages.
	//   - Run health checks, credential refresh, usage aggregation, etc.
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-sigCh:
				logger.Info("received shutdown signal, draining worker loop")
				return
			case <-ticker.C:
				// Placeholder: heartbeat log to confirm the worker is alive.
				logger.Debug("worker heartbeat")
			}
		}
	}()

	// Wait for the worker loop to finish.
	<-done
	logger.Info("async-worker stopped")
}
