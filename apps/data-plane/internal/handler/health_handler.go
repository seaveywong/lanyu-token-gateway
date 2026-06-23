package handler

import (
	"net/http"
)

// HealthHandler returns a simple health-check response.
// GET /health
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "data-plane",
	})
}
