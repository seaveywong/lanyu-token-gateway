package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RoleHierarchy defines the privilege ordering for platform and organization roles.
// Higher values represent greater privilege.
var RoleHierarchy = map[string]int{
	"viewer":         0,
	"developer":      1,
	"org_admin":      2,
	"org_owner":      3,
	"support":        4,
	"operator":       5,
	"finance":        6,
	"platform_admin": 7,
	"platform_owner": 8,
}

// HasMinimumRole returns true if the userRole is at least as privileged as minRole.
func HasMinimumRole(userRole, minRole string) bool {
	userLevel, ok := RoleHierarchy[userRole]
	if !ok {
		return false
	}
	minLevel, ok := RoleHierarchy[minRole]
	if !ok {
		return false
	}
	return userLevel >= minLevel
}

// RequireRole returns middleware that checks if the authenticated user has at
// least one of the specified roles. Returns 403 if the requirement is not met.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRoles := RolesFromContext(r.Context())

			for _, userRole := range userRoles {
				for _, required := range roles {
					if userRole == required {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			writeForbidden(w, "insufficient permissions")
		})
	}
}

// RequireOrgRole returns middleware that checks if the authenticated user
// holds at least minRole within the organization identified by the :orgId URL
// parameter. Returns 403 if the requirement is not met.
func RequireOrgRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID := chi.URLParam(r, "orgId")
			ctxOrgID, _ := r.Context().Value(CtxOrgID).(string)

			// The user's org context must match the URL parameter.
			if ctxOrgID == "" || (orgID != "" && ctxOrgID != orgID) {
				writeForbidden(w, "organization access denied")
				return
			}

			userRoles := RolesFromContext(r.Context())
			for _, role := range userRoles {
				if HasMinimumRole(role, minRole) {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeForbidden(w, "insufficient organization role")
		})
	}
}

// writeForbidden sends a 403 JSON error response.
func writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"code":"forbidden","message":"` + message + `"}`))
}
