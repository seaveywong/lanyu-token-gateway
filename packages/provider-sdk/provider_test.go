package provider_sdk

import (
	"context"
	"testing"
)

// mockAdapter is a minimal ProviderAdapter implementation for testing.
type mockAdapter struct {
	providerID ProviderID
}

func (m *mockAdapter) Provider() ProviderID { return m.providerID }
func (m *mockAdapter) Validate(ctx context.Context, source SourceCredential) ValidationResult {
	return ValidationResult{Valid: true}
}
func (m *mockAdapter) DiscoverModels(ctx context.Context, source SourceCredential) []ProviderModel {
	return nil
}
func (m *mockAdapter) Capabilities(model string) ModelCapabilities { return ModelCapabilities{} }
func (m *mockAdapter) Estimate(ctx context.Context, req CanonicalRequest) CostEstimate {
	return CostEstimate{}
}
func (m *mockAdapter) Invoke(ctx context.Context, req CanonicalRequest, source ResolvedSource) (CanonicalResponse, Usage, error) {
	return CanonicalResponse{}, Usage{}, nil
}
func (m *mockAdapter) Stream(ctx context.Context, req CanonicalRequest, source ResolvedSource, emit EventSink) (Usage, error) {
	return Usage{}, nil
}
func (m *mockAdapter) NormalizeError(err error) ProviderError {
	return ProviderError{Code: "unknown", Message: err.Error()}
}
func (m *mockAdapter) Health(ctx context.Context, source ResolvedSource) HealthResult {
	return HealthResult{Healthy: true}
}

// compile-time check that mockAdapter satisfies ProviderAdapter.
var _ ProviderAdapter = (*mockAdapter)(nil)

func TestProviderID_String(t *testing.T) {
	id := ProviderID("openai")
	if string(id) != "openai" {
		t.Errorf("ProviderID = %q, want %q", string(id), "openai")
	}
}

func TestSupportField_Values(t *testing.T) {
	tests := []struct {
		field SupportField
		want  string
	}{
		{SupportNative, "native"},
		{SupportTranslated, "translated"},
		{SupportBestEffort, "best_effort"},
		{SupportUnsupported, "unsupported"},
	}
	for _, tt := range tests {
		if string(tt.field) != tt.want {
			t.Errorf("SupportField = %q, want %q", string(tt.field), tt.want)
		}
	}
}

func TestProviderError_Error(t *testing.T) {
	pe := &ProviderError{
		Code:       "rate_limited",
		Message:    "too many requests",
		StatusCode: 429,
		Retryable:  true,
	}
	errStr := pe.Error()
	if errStr == "" {
		t.Error("ProviderError.Error() returned empty string")
	}
}

func TestCanonicalRequest_Fields(t *testing.T) {
	req := CanonicalRequest{
		RequestID:      "req-001",
		TenantID:       "org-001",
		ProjectID:      "proj-001",
		KeyID:          "key-001",
		RequestedModel: "gpt-4",
		Modality:       ModalityText,
		Stream:         false,
	}
	if req.RequestID != "req-001" {
		t.Errorf("RequestID = %q, want %q", req.RequestID, "req-001")
	}
}

func TestHealthResult_Fields(t *testing.T) {
	hr := HealthResult{
		Healthy: true,
	}
	if !hr.Healthy {
		t.Error("HealthResult.Healthy should be true")
	}
}

func TestModelCapabilities_Default(t *testing.T) {
	mc := ModelCapabilities{
		SupportsStreaming: SupportNative,
		MaxContextTokens:  128000,
		MaxOutputTokens:   4096,
	}
	if mc.SupportsStreaming != SupportNative {
		t.Errorf("SupportsStreaming = %q, want %q", mc.SupportsStreaming, SupportNative)
	}
	if mc.MaxContextTokens != 128000 {
		t.Errorf("MaxContextTokens = %d, want %d", mc.MaxContextTokens, 128000)
	}
}
