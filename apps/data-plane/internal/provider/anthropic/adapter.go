// Package anthropic implements a skeleton ProviderAdapter for the Anthropic
// (Claude) API. Invoke and Stream return "not implemented" errors. This will
// be fully implemented in a later phase.
package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/packages/provider_sdk"
)

// DefaultAPIBase is the default Anthropic API base URL.
const DefaultAPIBase = "https://api.anthropic.com/v1"

// Adapter implements provider_sdk.ProviderAdapter for Anthropic as a
// skeleton. Invoke and Stream return "not implemented" errors.
type Adapter struct {
	httpClient *http.Client
	apiBase    string
}

// NewAdapter creates a new Anthropic adapter skeleton.
func NewAdapter(apiBase string) *Adapter {
	if apiBase == "" {
		apiBase = DefaultAPIBase
	}
	return &Adapter{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		apiBase: apiBase,
	}
}

// Provider returns the provider identifier.
func (a *Adapter) Provider() provider_sdk.ProviderID {
	return "anthropic"
}

// Validate performs a basic credential validation by making a lightweight
// API call to list models.
func (a *Adapter) Validate(ctx context.Context, source provider_sdk.SourceCredential) provider_sdk.ValidationResult {
	return provider_sdk.ValidationResult{
		Valid:     false,
		Message:   "Anthropic adapter is not yet implemented",
		ErrorCode: "not_implemented",
	}
}

// DiscoverModels returns an empty list — not yet implemented.
func (a *Adapter) DiscoverModels(ctx context.Context, source provider_sdk.SourceCredential) []provider_sdk.ProviderModel {
	return nil
}

// Capabilities returns a basic, optimistic capability matrix.
func (a *Adapter) Capabilities(model string) provider_sdk.ModelCapabilities {
	return provider_sdk.ModelCapabilities{
		SupportsStreaming:    provider_sdk.SupportNative,
		SupportsToolUse:      provider_sdk.SupportNative,
		SupportsImageInput:   provider_sdk.SupportNative,
		SupportsAudioInput:   provider_sdk.SupportUnsupported,
		SupportsVideoInput:   provider_sdk.SupportUnsupported,
		SupportsJSONMode:     provider_sdk.SupportBestEffort,
		SupportsSystemPrompt: provider_sdk.SupportNative,
		SupportsMultiTurn:    provider_sdk.SupportNative,
		SupportsReasoning:    provider_sdk.SupportNative,
		MaxContextTokens:     200000,
		MaxOutputTokens:      8192,
	}
}

// Estimate returns a zero-cost estimate — not yet implemented.
func (a *Adapter) Estimate(ctx context.Context, req provider_sdk.CanonicalRequest) provider_sdk.CostEstimate {
	return provider_sdk.CostEstimate{
		Currency: "USD",
		Notes:    "cost estimation not yet implemented for Anthropic",
	}
}

// Invoke returns a "not implemented" error.
func (a *Adapter) Invoke(ctx context.Context, req provider_sdk.CanonicalRequest, source provider_sdk.ResolvedSource) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
		fmt.Errorf("anthropic: Invoke not yet implemented")
}

// Stream returns a "not implemented" error.
func (a *Adapter) Stream(ctx context.Context, req provider_sdk.CanonicalRequest, source provider_sdk.ResolvedSource, emit provider_sdk.EventSink) (provider_sdk.Usage, error) {
	return provider_sdk.Usage{},
		fmt.Errorf("anthropic: Stream not yet implemented")
}

// NormalizeError converts an error into a ProviderError.
func (a *Adapter) NormalizeError(err error) provider_sdk.ProviderError {
	if err == nil {
		return provider_sdk.ProviderError{
			Code:    "unknown",
			Message: "nil error",
		}
	}
	return provider_sdk.ProviderError{
		Code:       "anthropic_error",
		Message:    err.Error(),
		Retryable:  true,
		StatusCode: 0,
		Raw:        err,
	}
}

// Health returns a not-yet-implemented health status.
func (a *Adapter) Health(ctx context.Context, source provider_sdk.ResolvedSource) provider_sdk.HealthResult {
	return provider_sdk.HealthResult{
		Healthy:      false,
		ErrorMessage: "Anthropic adapter is not yet implemented",
		LastChecked:  time.Now(),
	}
}
