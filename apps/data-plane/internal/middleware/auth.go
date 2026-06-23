package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/repository"
	"github.com/seaveywong/lanyu-token-gateway/packages/apikey"
	"github.com/seaveywong/lanyu-token-gateway/packages/contracts"
)

// Context keys for storing authenticated request information.
type ctxKey string

const (
	ctxKeyOrgID     ctxKey = "org_id"
	ctxKeyProjectID ctxKey = "project_id"
	ctxKeyKeyID     ctxKey = "key_id"
)

// AuthMiddleware performs API key authentication for incoming requests.
type AuthMiddleware struct {
	apiKeyRepo *repository.APIKeyRepo
	pepper     []byte
	logger     *slog.Logger
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(apiKeyRepo *repository.APIKeyRepo, pepper []byte) *AuthMiddleware {
	return &AuthMiddleware{
		apiKeyRepo: apiKeyRepo,
		pepper:     pepper,
		logger:     slog.Default().With(slog.String("component", "auth_middleware")),
	}
}

// Authenticate is a chi middleware that:
// 1. Extracts the API key from Authorization: Bearer <key>
// 2. Computes the HMAC hash using the pepper
// 3. Looks up the hash in the database
// 4. Checks revocation, expiry, and IP allowlist
// 5. Stores key info in context (OrgID, ProjectID, KeyID)
// 6. Returns 401 if invalid
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeAuthError(w, contracts.ErrInvalidAPIKey, "missing or malformed Authorization header", "")
			return
		}
		rawKey := strings.TrimPrefix(authHeader, "Bearer ")
		if rawKey == "" {
			writeAuthError(w, contracts.ErrInvalidAPIKey, "empty API key", "")
			return
		}

		// 2. Compute HMAC hash using pepper
		hash, _, err := apikey.HashKey(m.pepper, rawKey)
		if err != nil {
			m.logger.Error("failed to hash key", slog.String("error", err.Error()))
			writeAuthError(w, contracts.ErrInternalError, "authentication processing failed", "")
			return
		}

		// 3. Look up hash in database
		keyRecord, err := m.apiKeyRepo.FindByHash(r.Context(), hash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAuthError(w, contracts.ErrInvalidAPIKey, "invalid API key", "")
				return
			}
			m.logger.Error("failed to look up API key", slog.String("error", err.Error()))
			writeAuthError(w, contracts.ErrInternalError, "authentication lookup failed", "")
			return
		}

		// 4. Check revocation
		if keyRecord.RevokedAt != nil {
			writeAuthError(w, contracts.ErrKeyDisabled, "API key has been revoked", "")
			return
		}

		// 4. Check expiry
		if keyRecord.ExpiresAt != nil && keyRecord.ExpiresAt.Before(time.Now()) {
			writeAuthError(w, contracts.ErrKeyDisabled, "API key has expired", "")
			return
		}

		// 4. Check IP allowlist
		if keyRecord.IPAllowlistJSON != "" && keyRecord.IPAllowlistJSON != "[]" {
			clientIP := extractClientIP(r)
			if !isIPAllowed(keyRecord.IPAllowlistJSON, clientIP) {
				m.logger.Warn("IP not in allowlist",
					slog.String("key_id", keyRecord.ID),
					slog.String("client_ip", clientIP),
				)
				writeAuthError(w, contracts.ErrKeyDisabled, "IP address not allowed", "")
				return
			}
		}

		// 5. Store key info in context
		ctx := context.WithValue(r.Context(), ctxKeyOrgID, keyRecord.OrganizationID)
		ctx = context.WithValue(ctx, ctxKeyProjectID, keyRecord.ProjectID)
		ctx = context.WithValue(ctx, ctxKeyKeyID, keyRecord.ID)

		// Update last_used_at asynchronously (best-effort, don't block the request)
		go func() {
			if err := m.apiKeyRepo.UpdateLastUsed(context.Background(), keyRecord.ID); err != nil {
				m.logger.Warn("failed to update last_used_at",
					slog.String("key_id", keyRecord.ID),
					slog.String("error", err.Error()),
				)
			}
		}()

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OrgIDFromContext extracts the organization ID from the request context.
func OrgIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyOrgID).(string)
	return v
}

// ProjectIDFromContext extracts the project ID from the request context.
func ProjectIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyProjectID).(string)
	return v
}

// KeyIDFromContext extracts the API key ID from the request context.
func KeyIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyKeyID).(string)
	return v
}

// writeAuthError writes a standardized authentication error response.
func writeAuthError(w http.ResponseWriter, code contracts.ErrorCode, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code.HTTPStatus())
	json.NewEncoder(w).Encode(contracts.GatewayError{
		Code:      code,
		Message:   message,
		Type:      "authentication_error",
		RequestID: requestID,
	})
}

// extractClientIP extracts the client IP from the request.
func extractClientIP(r *http.Request) string {
	// Try X-Forwarded-For first (set by chi's RealIP middleware)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	// Fall back to X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isIPAllowed checks whether the given IP is in the JSON allowlist.
// The allowlist is a JSON array of CIDR strings or plain IPs.
func isIPAllowed(allowlistJSON, clientIP string) bool {
	var allowlist []string
	if err := json.Unmarshal([]byte(allowlistJSON), &allowlist); err != nil {
		return false
	}
	if len(allowlist) == 0 {
		return true // empty allowlist means all IPs allowed
	}

	parsedIP := net.ParseIP(clientIP)
	if parsedIP == nil {
		return false
	}

	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// Check if entry is a CIDR
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			if cidr.Contains(parsedIP) {
				return true
			}
		} else {
			// Plain IP match
			if entry == clientIP {
				return true
			}
		}
	}
	return false
}
