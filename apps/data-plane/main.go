// Data Plane is the user-facing API gateway that handles incoming AI API
// requests (e.g. /v1/chat/completions), authenticates clients, routes to
// upstream providers, meters usage, and returns responses.
//
// It is the runtime entry point for the Lanyu Token Gateway data plane.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	// "github.com/prometheus/client_golang/prometheus/promhttp" // TODO: enable when dependencies available

	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/cache"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/database"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/handler"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/provider"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/provider/anthropic"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/provider/gemini"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/provider/openai"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/repository"
	"github.com/seaveywong/lanyu-token-gateway/packages/config"
	"github.com/seaveywong/lanyu-token-gateway/packages/observability"
)

func main() {
	// --- 1. Load configuration ---
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --- 2. Initialize observability ---
	obsCfg := observability.ObservabilityConfig{
		ServiceName:    cfg.Observability.ServiceName,
		ServiceVersion: cfg.Observability.ServiceVersion,
		OTLPEndpoint:   cfg.Observability.OTLPEndpoint,
		LogLevel:       cfg.Observability.LogLevel,
		LogFormat:      cfg.Observability.LogFormat,
	}
	obsShutdown, err := observability.Init(context.Background(), obsCfg)
	if err != nil {
		slog.Error("failed to initialize observability", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := obsShutdown(context.Background()); err != nil {
			slog.Error("observability shutdown error", slog.String("error", err.Error()))
		}
	}()

	logger := observability.Logger()

	// --- 3. Initialize database connections ---
	db, err := database.New(context.Background(), *cfg)
	if err != nil {
		logger.Error("failed to initialize database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("database connections established")

	// --- 4. Initialize repositories ---
	apiKeyRepo := repository.NewAPIKeyRepo(db.Pool)
	sourceRepo := repository.NewSourceRepo(db.Pool)
	usageRepo := repository.NewUsageRepo(db.Pool)

	// --- 5. Initialize provider registry and register adapters ---
	reg := provider.NewRegistry()

	// Register OpenAI adapter
	openaiAdapter := openai.NewAdapter("")
	if err := reg.Register(openaiAdapter); err != nil {
		logger.Error("failed to register OpenAI adapter", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("registered OpenAI adapter")

	// Register Anthropic adapter
	anthropicAdapter := anthropic.NewAdapter("")
	if err := reg.Register(anthropicAdapter); err != nil {
		logger.Error("failed to register Anthropic adapter", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("registered Anthropic adapter")

	// Register Gemini adapter
	geminiAdapter := gemini.NewAdapter("")
	if err := reg.Register(geminiAdapter); err != nil {
		logger.Error("failed to register Gemini adapter", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("registered Gemini adapter")

	// --- 6. Initialize middleware ---
	// TODO: Load pepper from KMS/secure file. For now use placeholder.
	pepper := []byte("placeholder-pepper")

	authMiddleware := middleware.NewAuthMiddleware(apiKeyRepo, pepper)
	rateLimiter := middleware.NewRateLimiter(db.Redis)

	// --- 6.5 Initialize cache ---
	exactCache := cache.NewExactCache(db.Redis)

	// --- 7. Initialize handlers ---
	modelsHandler := handler.NewModelsHandler()
	chatHandler := handler.NewChatHandler(reg, sourceRepo, usageRepo, exactCache)
	embeddingsHandler := handler.NewEmbeddingsHandler(reg, sourceRepo, usageRepo)
	imageHandler := handler.NewImageHandler(reg, sourceRepo, usageRepo)
	audioHandler := handler.NewAudioHandler(reg, sourceRepo, usageRepo)
	moderationHandler := handler.NewModerationHandler(reg, sourceRepo, usageRepo)

	// --- 8. Build chi router ---
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestIDMiddleware)

	// Public routes
	r.Get("/health", handler.HealthHandler)

	// Metrics endpoint (internal -- only for Prometheus scraping, not exposed publicly)
	// r.Get("/metrics", promhttp.Handler().ServeHTTP) // TODO: enable when prometheus client_golang available

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		// Model listing
		r.Get("/v1/models", modelsHandler.ListModels)

		// Chat completions (with rate limiting)
		r.Group(func(r chi.Router) {
			r.Use(rateLimiter.Limit)

			r.Post("/v1/chat/completions", chatHandler.CreateChatCompletion)
			r.Post("/v1/embeddings", embeddingsHandler.CreateEmbedding)
		})

		// Image generation endpoint
		r.Post("/v1/images/generations", imageHandler.CreateImage)

		// Audio endpoints
		r.Post("/v1/audio/speech", audioHandler.CreateSpeech)
		r.Post("/v1/audio/transcriptions", audioHandler.CreateTranscription)

		// Moderation endpoint
		r.Post("/v1/moderations", moderationHandler.CreateModeration)
	})

	// --- 9. Start HTTP server with graceful shutdown ---
	addr := cfg.Server.Host + ":" + cfg.Server.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM
	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Info("received signal, shutting down", slog.String("signal", sig.String()))

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", slog.String("error", err.Error()))
		}
		close(idleConnsClosed)
	}()

	logger.Info("data-plane starting", slog.String("addr", addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	<-idleConnsClosed
	logger.Info("data-plane stopped")
}
