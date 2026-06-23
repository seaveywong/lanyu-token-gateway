package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// ctxKeyRequestID is the context key for the request ID.
const ctxKeyRequestID ctxKey = "request_id"

// RequestIDMiddleware ensures every request has a unique X-Request-ID header.
// If the client sends an X-Request-ID header, it is preserved.
// Otherwise, a new UUID v7 is generated and set.
//
// The request ID is stored in the context and can be retrieved via RequestIDFromContext.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set the header on the response so the client can correlate
		w.Header().Set("X-Request-ID", requestID)

		// Store in context for downstream handlers
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the request ID from the request context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}
