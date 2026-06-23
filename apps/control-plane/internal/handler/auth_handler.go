package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/packages/auth"
)

// AuthHandler handles authentication endpoints: register, login, logout,
// token refresh, current-user lookup, and MFA setup/enable.
type AuthHandler struct {
	userService        UserService
	jwtSecret          []byte
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(userService UserService, jwtSecret []byte, accessExpiry, refreshExpiry time.Duration) *AuthHandler {
	return &AuthHandler{
		userService:        userService,
		jwtSecret:          jwtSecret,
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
	}
}

// Register handles POST /portal-api/auth/register.
// Creates a new user account from email, password, and optional display_name.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "email and password are required", requestID(r))
		return
	}

	user, err := h.userService.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		slog.Error("register failed", slog.String("error", err.Error()), slog.String("email", req.Email))
		respondError(w, http.StatusConflict, "invalid_request", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

// Login handles POST /portal-api/auth/login.
// On success it returns access_token, refresh_token, and user, and also sets a
// "session" HttpOnly cookie with the access token.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "email and password are required", requestID(r))
		return
	}

	user, err := h.userService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		slog.Error("login failed", slog.String("error", err.Error()), slog.String("email", req.Email))
		respondError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials", requestID(r))
		return
	}

	// Generate token pair.
	accessToken, refreshToken, err := auth.GenerateTokenPair(
		user.ID, "", nil, h.jwtSecret, h.accessTokenExpiry, h.refreshTokenExpiry,
	)
	if err != nil {
		slog.Error("token generation failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to generate tokens", requestID(r))
		return
	}

	// Set session cookie (HttpOnly, Secure, SameSite lax).
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(h.accessTokenExpiry),
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

// RefreshToken handles POST /portal-api/auth/refresh.
// Accepts a refresh_token, validates it, and issues a new token pair.
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.RefreshToken == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required", requestID(r))
		return
	}

	claims, err := auth.ValidateToken(req.RefreshToken, h.jwtSecret)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired refresh token", requestID(r))
		return
	}

	// Issue a fresh token pair using the user ID from the refresh token.
	accessToken, refreshToken, err := auth.GenerateTokenPair(
		claims.UserID, claims.OrgID, claims.Roles, h.jwtSecret, h.accessTokenExpiry, h.refreshTokenExpiry,
	)
	if err != nil {
		slog.Error("token refresh failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to refresh tokens", requestID(r))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(h.accessTokenExpiry),
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// Logout handles POST /portal-api/auth/logout.
// Clears the session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	respondJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// Me handles GET /portal-api/auth/me.
// Returns the currently authenticated user's profile.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	user, err := h.userService.GetByID(r.Context(), userID)
	if err != nil {
		slog.Error("fetch current user failed", slog.String("error", err.Error()), slog.String("user_id", userID))
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to fetch user", requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// SetupMFA handles POST /portal-api/auth/mfa/setup.
// Generates a new TOTP secret and returns the otpauth URL for QR code display.
func (h *AuthHandler) SetupMFA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	secret, qrURL, err := h.userService.SetupMFA(r.Context(), userID)
	if err != nil {
		slog.Error("mfa setup failed", slog.String("error", err.Error()), slog.String("user_id", userID))
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to setup MFA", requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"secret": secret,
		"qr_url": qrURL,
	})
}

// EnableMFA handles POST /portal-api/auth/mfa/enable.
// Verifies the provided TOTP code and enables MFA for the user, returning
// recovery codes on success.
func (h *AuthHandler) EnableMFA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Code == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "code is required", requestID(r))
		return
	}

	recoveryCodes, err := h.userService.EnableMFA(r.Context(), userID, req.Code)
	if err != nil {
		slog.Error("mfa enable failed", slog.String("error", err.Error()), slog.String("user_id", userID))
		respondError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"mfa_enabled":    true,
		"recovery_codes": recoveryCodes,
	})
}
