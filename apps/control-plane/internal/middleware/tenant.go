package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RequireOrgContext extracts the :orgId URL parameter and stores it in the
// request context. For routes that do not have an :orgId parameter, it is a
// no-op and passes through unchanged.
func RequireOrgContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "orgId")
		if orgID != "" {
			ctx := context.WithValue(r.Context(), CtxOrgID, orgID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
