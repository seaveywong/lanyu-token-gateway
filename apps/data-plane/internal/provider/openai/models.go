package openai

// ---------------------------------------------------------------------------
// OpenAI Models API wire types
// ---------------------------------------------------------------------------

// openAIModelsResponse is the response from GET /v1/models.
type openAIModelsResponse struct {
	Object string            `json:"object"`
	Data   []openAIModelData `json:"data"`
}

// openAIModelData represents a single model entry from the API.
type openAIModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// DiscoverModels is implemented in adapter.go because it requires the
// httpClient and apiBase fields from Adapter.
//
// The method:
//  1. Sends GET {apiBase}/models with the API key
//  2. Parses the response
//  3. Returns a list of ProviderModel entries with capabilities resolved
//
// This file exists to house model-specific types and any future model
// discovery helpers (e.g., model filtering, model aliasing).
//
// See adapter.go for the full DiscoverModels implementation.
