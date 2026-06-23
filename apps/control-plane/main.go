// Control Plane is the administration and management API for the Lanyu Token
// Gateway. It serves the admin dashboard (/admin-api/) and the customer
// portal (/portal-api/), handling user management, channel configuration,
// billing, and operational controls.
//
// It is the runtime entry point for the Lanyu Token Gateway control plane.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/database"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/handler"
	mw "github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/service"
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

	// --- Initialize database connections ---
	db, err := database.New(ctx, *cfg)
	if err != nil {
		logger.Error("failed to initialize database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("database connections established")

	// --- Repository layer ---
	userRepo := repository.NewUserRepo(db.Pool)
	orgRepo := repository.NewOrgRepo(db.Pool)
	memberRepo := repository.NewMemberRepo(db.Pool)
	projectRepo := repository.NewProjectRepo(db.Pool)
	apiKeyRepo := repository.NewAPIKeyRepo(db.Pool)
	auditRepo := repository.NewAuditRepo(db.Pool)

	// --- Service layer ---
	// Note: pepper should be loaded from cfg.Auth.PepperPath at startup.
	pepper := []byte("placeholder-pepper") // TODO: load from cfg.Auth.PepperPath

	userSvc := service.NewUserService(userRepo, auditRepo)
	orgSvc := service.NewOrgService(orgRepo, memberRepo, auditRepo)
	projectSvc := service.NewProjectService(projectRepo, orgRepo, memberRepo, auditRepo)
	apiKeySvcCfg := service.APIKeyServiceConfig{
		Pepper:        pepper,
		KeyPrefix:     cfg.Auth.KeyPrefix,
		KeyByteLength: cfg.Auth.KeyByteLength,
		DefaultExpiry: cfg.Auth.DefaultExpiryDuration,
	}
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, projectRepo, memberRepo, auditRepo, apiKeySvcCfg)
	auditSvc := service.NewAuditService(auditRepo)

	accountSourceRepo := repository.NewAccountSourceRepo(db.Pool)
	channelRepo := repository.NewChannelRepo(db.Pool)
	modelRepo := repository.NewModelRepo(db.Pool)

	accountSourceSvc := service.NewAccountSourceService(accountSourceRepo, auditRepo)
	channelSvc := service.NewChannelService(channelRepo, accountSourceRepo, auditRepo)
	routingSvc := service.NewRoutingService(accountSourceRepo, modelRepo, db.Redis)
	healthSvc := service.NewHealthService(accountSourceRepo, db.Redis)

	// channelSvc and routingSvc are wired here for future use when channel and
	// routing HTTP handlers are added to the admin and portal APIs.
	_, _ = channelSvc, routingSvc

	// --- Adapters: bridge service types to handler interfaces ---
	// Some handler interfaces require methods not yet on the service types
	// (GetByID, ListAll). Thin adapters wrap the services and add stubs.

	var (
		userServiceAdapter          handler.UserService          = &userServiceBridge{svc: userSvc, repo: userRepo}
		orgServiceAdapter           handler.OrgService           = &orgServiceBridge{svc: orgSvc, repo: orgRepo}
		projectServiceAdapter       handler.ProjectService       = projectSvc
		apiKeyServiceAdapter        handler.APIKeyService        = &apiKeyServiceBridge{svc: apiKeySvc}
		auditServiceAdapter         handler.AuditService         = auditSvc
		accountSourceServiceAdapter handler.AccountSourceService = &accountSourceServiceBridge{svc: accountSourceSvc}
	)

	// --- Handler layer ---
	jwtSecret := []byte(cfg.Auth.JWTSecret)

	authHandler := handler.NewAuthHandler(
		userServiceAdapter,
		jwtSecret,
		cfg.Auth.AccessTokenExpiry,
		cfg.Auth.RefreshTokenExpiry,
	)
	orgHandler := handler.NewOrgHandler(orgServiceAdapter)
	projectHandler := handler.NewProjectHandler(projectServiceAdapter)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyServiceAdapter)
	adminHandler := handler.NewAdminHandler(
		orgHandler,
		accountSourceServiceAdapter,
		auditServiceAdapter,
		userServiceAdapter,
	)

	// --- Start background health check loop ---
	go healthSvc.StartHealthCheckLoop(context.Background(), 60*time.Second)

	// --- Middleware layer ---
	authMiddleware := mw.NewAuthMiddleware(jwtSecret)

	corsOrigins := []string{
		"http://localhost:5173", // admin-web dev server
		"http://localhost:5174", // portal-web dev server
	}

	// --- Build HTTP router ---
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(mw.CORSMiddleware(corsOrigins))

	// Health check (public)
	r.Get("/health", healthHandler)

	// --- Portal API: public auth routes ---
	r.Post("/portal-api/auth/register", authHandler.Register)
	r.Post("/portal-api/auth/login", authHandler.Login)
	r.Post("/portal-api/auth/refresh", authHandler.RefreshToken)

	// --- Portal API: authenticated routes ---
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		// Auth
		r.Post("/portal-api/auth/logout", authHandler.Logout)
		r.Get("/portal-api/auth/me", authHandler.Me)
		r.Post("/portal-api/auth/mfa/setup", authHandler.SetupMFA)
		r.Post("/portal-api/auth/mfa/enable", authHandler.EnableMFA)

		// Organizations
		r.Get("/portal-api/orgs", orgHandler.List)
		r.Post("/portal-api/orgs", orgHandler.Create)
		r.Get("/portal-api/orgs/{orgId}", orgHandler.Get)

		// Projects (org-scoped)
		r.Get("/portal-api/orgs/{orgId}/projects", projectHandler.ListByOrg)

		// API Keys (project-scoped)
		r.Get("/portal-api/projects/{projectId}/keys", apiKeyHandler.ListByProject)
		r.Post("/portal-api/projects/{projectId}/keys", apiKeyHandler.Create)
		r.Post("/portal-api/projects/{projectId}/keys/{keyId}/revoke", apiKeyHandler.Revoke)
	})

	// --- Admin API: platform-level routes ---
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(mw.RequireRole("platform_admin", "platform_owner"))

		r.Get("/admin-api/users", adminHandler.ListUsers)
		r.Get("/admin-api/orgs", adminHandler.ListOrgs)
		r.Get("/admin-api/account-sources", adminHandler.ListAccountSources)
		r.Post("/admin-api/account-sources", adminHandler.CreateAccountSource)
		r.Get("/admin-api/audit-logs", adminHandler.ListAuditLogs)
	})

	// --- Start HTTP server ---
	addr := cfg.Server.Host + ":" + cfg.Server.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
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

	logger.Info("control-plane starting", slog.String("addr", addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	<-idleConnsClosed
	logger.Info("control-plane stopped")
}

// healthHandler returns a simple health-check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok","service":"control-plane"}`)
}

// ---------------------------------------------------------------------------
// Service adapters — bridge the *service.XxxService concrete types to the
// handler.XxxService interfaces. These exist to:
//   1. Add methods not yet on the service types (GetByID, ListAll).
//   2. Adapt return types where service types differ from handler interface.
// Once the service layer fully implements the handler interfaces, these
// adapters can be removed.
// ---------------------------------------------------------------------------

// userServiceBridge adapts *service.UserService to handler.UserService.
type userServiceBridge struct {
	svc  *service.UserService
	repo *repository.UserRepo
}

func (b *userServiceBridge) Register(ctx context.Context, email, password, displayName string) (*repository.User, error) {
	return b.svc.Register(ctx, email, password, displayName)
}
func (b *userServiceBridge) Login(ctx context.Context, email, password string) (*repository.User, error) {
	return b.svc.Login(ctx, email, password)
}
func (b *userServiceBridge) SetupMFA(ctx context.Context, userID string) (string, string, error) {
	return b.svc.SetupMFA(ctx, userID)
}
func (b *userServiceBridge) EnableMFA(ctx context.Context, userID, code string) ([]string, error) {
	return b.svc.EnableMFA(ctx, userID, code)
}
func (b *userServiceBridge) VerifyMFA(ctx context.Context, userID, code string) (bool, error) {
	return b.svc.VerifyMFA(ctx, userID, code)
}
func (b *userServiceBridge) GetByID(ctx context.Context, id string) (*repository.User, error) {
	return b.repo.FindByID(ctx, id)
}
func (b *userServiceBridge) ListAll(ctx context.Context, page, pageSize int) ([]repository.User, int, error) {
	// TODO: implement paginated user listing when UserRepo.ListAll is added.
	return nil, 0, fmt.Errorf("ListAll not yet implemented")
}

// orgServiceBridge adapts *service.OrgService to handler.OrgService.
type orgServiceBridge struct {
	svc  *service.OrgService
	repo *repository.OrgRepo
}

func (b *orgServiceBridge) Create(ctx context.Context, userID, name string) (*repository.Organization, error) {
	return b.svc.Create(ctx, userID, name)
}
func (b *orgServiceBridge) GetByID(ctx context.Context, id string) (*repository.Organization, error) {
	return b.svc.GetByID(ctx, id)
}
func (b *orgServiceBridge) ListByUser(ctx context.Context, userID string) ([]repository.Organization, error) {
	return b.svc.ListByUser(ctx, userID)
}
func (b *orgServiceBridge) ListAll(ctx context.Context, page, pageSize int) ([]repository.Organization, int, error) {
	// TODO: implement paginated org listing when OrgRepo.ListAll is added.
	return nil, 0, fmt.Errorf("ListAll not yet implemented")
}

// apiKeyServiceBridge adapts *service.APIKeyService to handler.APIKeyService.
// It converts service.GeneratedKey to handler.APIKeyCreateResult.
type apiKeyServiceBridge struct {
	svc *service.APIKeyService
}

func (b *apiKeyServiceBridge) Create(ctx context.Context, projectID, userID, name, env string) (*handler.APIKeyCreateResult, error) {
	gk, err := b.svc.Create(ctx, projectID, userID, name, env)
	if err != nil {
		return nil, err
	}
	var expiresAt *string
	if gk.ExpiresAt != nil {
		s := gk.ExpiresAt.Format(time.RFC3339)
		expiresAt = &s
	}
	return &handler.APIKeyCreateResult{
		ID:        gk.ID,
		RawKey:    gk.RawKey,
		Prefix:    gk.Prefix,
		ExpiresAt: expiresAt,
	}, nil
}
func (b *apiKeyServiceBridge) ListByProject(ctx context.Context, projectID, userID string) ([]repository.APIKey, error) {
	return b.svc.ListByProject(ctx, projectID, userID)
}
func (b *apiKeyServiceBridge) Revoke(ctx context.Context, keyID, userID string) error {
	return b.svc.Revoke(ctx, keyID, userID)
}

// accountSourceServiceBridge adapts *service.AccountSourceService to
// handler.AccountSourceService. It converts service-layer types to the
// response types expected by the admin handler.
type accountSourceServiceBridge struct {
	svc *service.AccountSourceService
}

func (b *accountSourceServiceBridge) List(ctx context.Context, page, pageSize int) ([]handler.AccountSourceResponse, int, error) {
	sources, total, err := b.svc.List(ctx, "", page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]handler.AccountSourceResponse, len(sources))
	for i, s := range sources {
		responses[i] = handler.AccountSourceResponse{
			ID:         s.ID,
			Name:       s.Name,
			SourceType: s.SourceType,
			Status:     s.Status,
		}
	}
	return responses, total, nil
}

func (b *accountSourceServiceBridge) Create(ctx context.Context, name, sourceType, providerID string, credentialCiphertext []byte, createdBy string) (*handler.AccountSourceResponse, error) {
	source, err := b.svc.Create(ctx, createdBy, service.CreateSourceParams{
		Name:       name,
		SourceType: sourceType,
		ProviderID: &providerID,
		Credential: string(credentialCiphertext),
	})
	if err != nil {
		return nil, err
	}
	return &handler.AccountSourceResponse{
		ID:         source.ID,
		Name:       source.Name,
		SourceType: source.SourceType,
		Status:     source.Status,
	}, nil
}
