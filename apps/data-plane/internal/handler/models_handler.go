package handler

import (
	"net/http"
)

// ModelsHandler handles model listing requests.
type ModelsHandler struct{}

// NewModelsHandler creates a new ModelsHandler.
func NewModelsHandler() *ModelsHandler {
	return &ModelsHandler{}
}

// modelEntry represents a single model in the list response.
type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// modelListResponse is the OpenAI-compatible model list response.
type modelListResponse struct {
	Object string       `json:"object"`
	Data   []modelEntry `json:"data"`
}

// ListModels returns the list of models the authenticated key has access to.
// GET /v1/models
func (h *ModelsHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	// For now, return a hardcoded list of common models.
	// This will be replaced with a dynamic model catalog lookup in the future.
	models := []modelEntry{
		// OpenAI models
		{ID: "gpt-4o", Object: "model", Created: 1715367049, OwnedBy: "openai"},
		{ID: "gpt-4o-mini", Object: "model", Created: 1721172741, OwnedBy: "openai"},
		{ID: "gpt-4-turbo", Object: "model", Created: 1712601677, OwnedBy: "openai"},
		{ID: "gpt-4", Object: "model", Created: 1687882411, OwnedBy: "openai"},
		{ID: "gpt-3.5-turbo", Object: "model", Created: 1677649963, OwnedBy: "openai"},
		// Anthropic models
		{ID: "claude-sonnet-4-20250514", Object: "model", Created: 1747228800, OwnedBy: "anthropic"},
		{ID: "claude-3-5-sonnet-latest", Object: "model", Created: 1721673600, OwnedBy: "anthropic"},
		{ID: "claude-3-5-haiku-latest", Object: "model", Created: 1721673600, OwnedBy: "anthropic"},
		// Google models
		{ID: "gemini-2.5-flash", Object: "model", Created: 1735689600, OwnedBy: "google"},
		{ID: "gemini-2.5-pro", Object: "model", Created: 1735689600, OwnedBy: "google"},
	}

	respondJSON(w, http.StatusOK, modelListResponse{
		Object: "list",
		Data:   models,
	})
}
