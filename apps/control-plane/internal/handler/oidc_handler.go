package handler

import (
	"log/slog"
	"net/http"
)

// OIDCConfig holds the configuration for the OIDC provider.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// OIDCHandler handles OIDC Single Sign-On login and callback.
// Full OIDC integration requires golang.org/x/oauth2 and a third-party OIDC library.
// This skeleton implements the routes and configuration wiring so the SSO flow
// is ready to be completed once the dependencies are added.
type OIDCHandler struct {
	userService UserService
	jwtSecret   []byte
	oidcConfig  OIDCConfig
}

// NewOIDCHandler creates a new OIDCHandler.
func NewOIDCHandler(userService UserService, jwtSecret []byte, oidcConfig OIDCConfig) *OIDCHandler {
	return &OIDCHandler{
		userService: userService,
		jwtSecret:   jwtSecret,
		oidcConfig:  oidcConfig,
	}
}

// IsEnabled returns true if the OIDC provider is configured.
func (h *OIDCHandler) IsEnabled() bool {
	return h.oidcConfig.IssuerURL != "" && h.oidcConfig.ClientID != ""
}

// Login handles GET /portal-api/auth/oidc/login.
// Redirects the user to the OIDC provider's authorization endpoint.
// State and nonce are generated and stored (in a session or cookie) to prevent CSRF.
//
// Full implementation steps:
//  1. Generate a cryptographically random state and nonce
//  2. Store them (e.g., in a short-lived cookie or Redis)
//  3. Build the OIDC authorization URL with response_type=code, scope=openid+email+profile,
//     redirect_uri, client_id, state, nonce
//  4. HTTP 302 redirect to the provider
func (h *OIDCHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.IsEnabled() {
		respondError(w, http.StatusServiceUnavailable, "oidc_disabled",
			"OIDC is not configured", requestID(r))
		return
	}

	slog.Info("oidc login requested (not_implemented skeleton)",
		slog.String("issuer", h.oidcConfig.IssuerURL))

	// TODO: Full OIDC implementation requires:
	//   - golang.org/x/oauth2 for the OAuth2 code exchange
	//   - github.com/coreos/go-oidc/v3/oidc (or similar) for OIDC ID token verification
	//   - State/nonce generation and cookie storage for CSRF protection
	//   - User profile extraction (email, name) from the ID token claims
	//   - Local user lookup-or-create logic
	//   - JWT access+refresh token issuance
	//   - Redirect to frontend with tokens
	respondError(w, http.StatusNotImplemented, "not_implemented",
		"OIDC login is configured but not yet implemented (skeleton — add go-oidc dependency)", requestID(r))
}

// Callback handles GET /portal-api/auth/oidc/callback.
// The OIDC provider redirects here after the user authenticates.
//
// Full implementation steps:
//  1. Validate state parameter against stored value (CSRF protection)
//  2. Exchange authorization code for tokens (access_token, refresh_token, id_token)
//  3. Verify the ID token (issuer, audience, nonce, expiry, signature)
//  4. Extract user claims (email, email_verified, name, sub)
//  5. Find existing user by OIDC sub or email, or create a new user
//  6. Issue JWT access+refresh tokens via the local auth system
//  7. Set session cookie or redirect to frontend with tokens in URL fragment
func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if !h.IsEnabled() {
		respondError(w, http.StatusServiceUnavailable, "oidc_disabled",
			"OIDC is not configured", requestID(r))
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		respondError(w, http.StatusBadRequest, "invalid_request",
			"missing authorization code", requestID(r))
		return
	}

	slog.Info("oidc callback received (not_implemented skeleton)",
		slog.String("code_prefix", code[:min(8, len(code))]),
		slog.String("state", state))

	// TODO: See Login() for implementation steps.
	respondError(w, http.StatusNotImplemented, "not_implemented",
		"OIDC callback is configured but not yet implemented (skeleton)", requestID(r))
}
