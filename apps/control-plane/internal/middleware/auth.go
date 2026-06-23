package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/seaveywong/lanyu-token-gateway/packages/auth"
)

// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey string

const (
	// CtxUserID stores the authenticated user's UUID in context.
	CtxUserID contextKey = "user_id"
	// CtxUserEmail stores the authenticated user's email in context.
	CtxUserEmail contextKey = "user_email"
	// CtxRoles stores the authenticated user's roles in context.
	CtxRoles contextKey = "roles"
	// CtxOrgID stores the current organization ID in context (set by auth token or tenant middleware).
	CtxOrgID contextKey = "org_id"
)

// AuthMiddleware validates JWT tokens from the Authorization header or session cookie.
type AuthMiddleware struct {
	JWTSecret []byte
}

// NewAuthMiddleware creates a new AuthMiddleware with the given JWT secret.
func NewAuthMiddleware(jwtSecret []byte) *AuthMiddleware {
	return &AuthMiddleware{JWTSecret: jwtSecret}
}

// Authenticate is a chi middleware that validates the JWT and stores user info in context.
// It checks the Authorization: Bearer header first, then falls back to the "session" cookie.
// Returns 401 if neither provides a valid token.
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeUnauthorized(w, "missing authentication")
			return
		}

		claims, err := auth.ValidateToken(token, m.JWTSecret)
		if err != nil {
			writeUnauthorized(w, "invalid or expired token")
			return
		}

		ctx := enrichContext(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is like Authenticate but does not error if no auth is present.
// It silently continues without setting user context when no valid token is found.
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := auth.ValidateToken(token, m.JWTSecret)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := enrichContext(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext extracts the authenticated user ID from the context.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxUserID).(string); ok {
		return v
	}
	return ""
}

// UserEmailFromContext extracts the authenticated user email from the context.
func UserEmailFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxUserEmail).(string); ok {
		return v
	}
	return ""
}

// OrgIDFromContext extracts the current organization ID from the context.
func OrgIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxOrgID).(string); ok {
		return v
	}
	return ""
}

// RolesFromContext extracts the authenticated user's roles from the context.
func RolesFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(CtxRoles).([]string); ok {
		return v
	}
	return nil
}

// extractToken pulls a JWT from the Authorization header (Bearer scheme) or the
// "session" cookie, in that order of precedence.
func extractToken(r *http.Request) string {
	// 1. Authorization: Bearer <jwt>
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	// 2. "session" cookie
	cookie, err := r.Cookie("session")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

// enrichContext adds claims data to the request context.
func enrichContext(ctx context.Context, claims *auth.Claims) context.Context {
	ctx = context.WithValue(ctx, CtxUserID, claims.UserID)
	ctx = context.WithValue(ctx, CtxRoles, claims.Roles)
	if claims.OrgID != "" {
		ctx = context.WithValue(ctx, CtxOrgID, claims.OrgID)
	}
	return ctx
}

// writeUnauthorized sends a 401 JSON error response.
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"code":"unauthorized","message":"` + message + `"}`))
}
