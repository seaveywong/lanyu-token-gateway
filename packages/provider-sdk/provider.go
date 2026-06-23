package provider_sdk

import (
	"context"
	"encoding/json"
	"time"
)

// ---------------------------------------------------------------------------
// Core identifiers
// ---------------------------------------------------------------------------

// ProviderID uniquely identifies an AI provider (e.g. "openai", "anthropic").
type ProviderID string

// ModelID is a canonical model identifier within the gateway model catalog.
type ModelID string

// Modality describes the input/output modality of a request or model.
type Modality string

const (
	ModalityText       Modality = "text"
	ModalityImage      Modality = "image"
	ModalityAudio      Modality = "audio"
	ModalityVideo      Modality = "video"
	ModalityMultimodal Modality = "multimodal"
)

// ---------------------------------------------------------------------------
// SupportField enum — how well a model supports a given capability.
// ---------------------------------------------------------------------------

// SupportField indicates the level of support a model provides for a
// particular feature or capability.
type SupportField string

const (
	SupportNative     SupportField = "native"      // natively supported by the provider/model
	SupportTranslated SupportField = "translated"  // supported via gateway-side translation
	SupportBestEffort SupportField = "best_effort" // partially supported; may degrade
	SupportUnsupported SupportField = "unsupported" // not supported
)

// ---------------------------------------------------------------------------
// Credentials & validation
// ---------------------------------------------------------------------------

// SourceCredential holds the credential information for an upstream account
// source. The concrete credential data is opaque to the adapter contract;
// each adapter interprets its own credential format.
type SourceCredential struct {
	SourceID   string            `json:"source_id"`
	SourceType string            `json:"source_type"`
	ProviderID ProviderID        `json:"provider_id"`
	Credential json.RawMessage   `json:"credential"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ValidationResult describes the outcome of validating a source credential.
type ValidationResult struct {
	Valid    bool   `json:"valid"`
	Message  string `json:"message,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

// ---------------------------------------------------------------------------
// Models & capabilities
// ---------------------------------------------------------------------------

// ProviderModel describes a model exposed by a provider.
type ProviderModel struct {
	ID           ModelID            `json:"id"`
	ProviderID   ProviderID         `json:"provider_id"`
	DisplayName  string             `json:"display_name"`
	Modality     Modality           `json:"modality"`
	Capabilities ModelCapabilities  `json:"capabilities"`
	Pricing      ModelPricing       `json:"pricing,omitempty"`
	Limits       ModelLimits        `json:"limits,omitempty"`
	Deprecated   bool               `json:"deprecated,omitempty"`
}

// ModelCapabilities describes what a model can and cannot do.
type ModelCapabilities struct {
	SupportsStreaming     SupportField `json:"supports_streaming"`
	SupportsToolUse       SupportField `json:"supports_tool_use"`
	SupportsImageInput    SupportField `json:"supports_image_input"`
	SupportsAudioInput    SupportField `json:"supports_audio_input"`
	SupportsVideoInput    SupportField `json:"supports_video_input"`
	SupportsJSONMode      SupportField `json:"supports_json_mode"`
	SupportsSystemPrompt  SupportField `json:"supports_system_prompt"`
	SupportsMultiTurn     SupportField `json:"supports_multi_turn"`
	SupportsReasoning     SupportField `json:"supports_reasoning"`
	MaxContextTokens      int           `json:"max_context_tokens"`
	MaxOutputTokens       int           `json:"max_output_tokens"`
}

// ModelPricing holds the cost information for a model.
type ModelPricing struct {
	InputPriceMicroUSD  int64 `json:"input_price_micro_usd"`
	OutputPriceMicroUSD int64 `json:"output_price_micro_usd"`
	CacheReadPriceMicroUSD int64 `json:"cache_read_price_micro_usd,omitempty"`
	Currency            string `json:"currency,omitempty"`
}

// ModelLimits describes rate and usage limits for a model.
type ModelLimits struct {
	MaxRequestsPerMinute int `json:"max_requests_per_minute,omitempty"`
	MaxTokensPerMinute   int `json:"max_tokens_per_minute,omitempty"`
	MaxConcurrentRequests int `json:"max_concurrent_requests,omitempty"`
}

// ---------------------------------------------------------------------------
// Canonical request / response
// ---------------------------------------------------------------------------

// Message represents a single message in a conversation (OpenAI-compatible shape).
type Message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`              // string or multimodal array
	Name       string          `json:"name,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

// ToolDefinition describes a tool available to the model.
type ToolDefinition struct {
	Type     string          `json:"type"`
	Function json.RawMessage `json:"function,omitempty"`
}

// ResponseFormat constrains the model's output format.
type ResponseFormat struct {
	Type       string          `json:"type"` // "text", "json_object", "json_schema"
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// GenerationParams collects generation hyperparameters.
type GenerationParams struct {
	Temperature      float64 `json:"temperature,omitempty"`
	TopP             float64 `json:"top_p,omitempty"`
	TopK             int     `json:"top_k,omitempty"`
	MaxTokens        int     `json:"max_tokens,omitempty"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64 `json:"presence_penalty,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	Seed             *int64  `json:"seed,omitempty"`
}

// PrivacyPolicy specifies data-handling instructions for the upstream call.
type PrivacyPolicy struct {
	DisableLogging bool `json:"disable_logging,omitempty"`
	DisableTraining bool `json:"disable_training,omitempty"`
	DataRegion     string `json:"data_region,omitempty"`
}

// CanonicalRequest is the provider-agnostic representation of an AI request.
// It is constructed by the gateway's translation layer and passed to every
// adapter for invocation.
type CanonicalRequest struct {
	RequestID        string           `json:"request_id"`
	TenantID         string           `json:"tenant_id"`
	ProjectID        string           `json:"project_id"`
	KeyID            string           `json:"key_id"`
	RequestedModel   ModelID          `json:"requested_model"`
	Modality         Modality         `json:"modality"`
	Messages         []Message        `json:"messages"`
	Tools            []ToolDefinition `json:"tools,omitempty"`
	ToolChoice       json.RawMessage  `json:"tool_choice,omitempty"`
	ResponseFormat   *ResponseFormat  `json:"response_format,omitempty"`
	GenerationParams GenerationParams `json:"generation_params,omitempty"`
	Stream           bool             `json:"stream"`
	IdempotencyKey   string           `json:"idempotency_key,omitempty"`
	PrivacyPolicy    PrivacyPolicy    `json:"privacy_policy,omitempty"`
}

// CanonicalResponse is the provider-agnostic representation of a complete
// (non-streaming) AI response.
type CanonicalResponse struct {
	ID           string          `json:"id"`
	Model        ModelID         `json:"model"`
	Choices      []ResponseChoice `json:"choices"`
	Usage        Usage           `json:"usage"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

// ResponseChoice represents a single completion choice.
type ResponseChoice struct {
	Index        int             `json:"index"`
	Message      Message         `json:"message"`
	FinishReason string          `json:"finish_reason,omitempty"`
	LogProbs     json.RawMessage `json:"logprobs,omitempty"`
}

// ---------------------------------------------------------------------------
// Streaming
// ---------------------------------------------------------------------------

// StreamEvent is a single event emitted during a streaming response.
type StreamEvent struct {
	EventType string          `json:"event_type"` // "delta", "done", "error"
	Data      json.RawMessage `json:"data"`
}

// EventSink is a callback that receives streaming events from an adapter.
// The adapter calls this function for each chunk; the caller is responsible
// for the sink's thread safety.
type EventSink func(event StreamEvent) error

// ---------------------------------------------------------------------------
// Cost estimation
// ---------------------------------------------------------------------------

// CostEstimate is a pre-flight cost estimate for a request.
type CostEstimate struct {
	EstimatedCostMicroUSD int64  `json:"estimated_cost_micro_usd"`
	EstimatedInputTokens  int    `json:"estimated_input_tokens"`
	EstimatedOutputTokens int    `json:"estimated_output_tokens"`
	Currency              string `json:"currency,omitempty"`
	Notes                 string `json:"notes,omitempty"`
}

// ---------------------------------------------------------------------------
// Usage
// ---------------------------------------------------------------------------

// Usage captures token and modality usage from an upstream call.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadTokens          int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens         int `json:"cache_write_tokens,omitempty"`
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	ImageCount               int `json:"image_count,omitempty"`
	AudioSeconds             int `json:"audio_seconds,omitempty"`
	TotalTokens              int `json:"total_tokens"`
}

// ---------------------------------------------------------------------------
// Resolved source
// ---------------------------------------------------------------------------

// ResolvedSource is the concrete upstream account/source selected by the
// router for a particular request.
type ResolvedSource struct {
	SourceID       string     `json:"source_id"`
	SourceType     string     `json:"source_type"`
	ProviderID     ProviderID `json:"provider_id"`
	ResolvedModel  ModelID    `json:"resolved_model"`
	EndpointURL    string     `json:"endpoint_url"`
	Credential     json.RawMessage `json:"credential"`
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// ProviderError is a structured error returned by an adapter when an upstream
// call fails. It preserves the original error while adding classification.
type ProviderError struct {
	Code       string `json:"code"`        // machine-readable error code
	Message    string `json:"message"`     // human-readable description
	StatusCode int    `json:"status_code"` // HTTP status from upstream (0 if none)
	Retryable  bool   `json:"retryable"`   // whether the request can be retried
	Fatal      bool   `json:"fatal"`       // source should be disabled immediately
	Raw        error  `json:"-"`           // underlying error (omitted from JSON)
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Raw != nil {
		return e.Code + ": " + e.Message + " (" + e.Raw.Error() + ")"
	}
	return e.Code + ": " + e.Message
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *ProviderError) Unwrap() error {
	return e.Raw
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// HealthResult describes the health status of an upstream source.
type HealthResult struct {
	Healthy      bool      `json:"healthy"`
	Latency      time.Duration `json:"latency"`
	ErrorMessage string    `json:"error_message,omitempty"`
	LastChecked  time.Time `json:"last_checked"`
}

// ---------------------------------------------------------------------------
// ProviderAdapter interface (§5.2 of the full implementation spec)
// ---------------------------------------------------------------------------

// ProviderAdapter is the core contract that every upstream AI provider
// adapter must implement. It normalises provider-specific APIs into the
// gateway's canonical request/response model.
//
// Each method must be safe for concurrent use.
type ProviderAdapter interface {
	// Provider returns the unique identifier for this provider.
	Provider() ProviderID

	// Validate checks whether the given source credential is valid and usable.
	// It typically makes a lightweight API call (e.g. list models) to verify.
	Validate(ctx context.Context, source SourceCredential) ValidationResult

	// DiscoverModels returns all models currently available from this provider
	// for the given credential.
	DiscoverModels(ctx context.Context, source SourceCredential) []ProviderModel

	// Capabilities returns the capability matrix for the named model.
	Capabilities(model string) ModelCapabilities

	// Estimate returns a pre-flight cost estimate for a canonical request.
	Estimate(ctx context.Context, req CanonicalRequest) CostEstimate

	// Invoke executes a synchronous (non-streaming) request and returns the
	// complete response along with token/multimodal usage.
	Invoke(ctx context.Context, req CanonicalRequest, source ResolvedSource) (CanonicalResponse, Usage, error)

	// Stream executes a streaming request, calling emit for each chunk.
	// It returns the final cumulative usage when the stream completes.
	Stream(ctx context.Context, req CanonicalRequest, source ResolvedSource, emit EventSink) (Usage, error)

	// NormalizeError converts a raw upstream error into a structured
	// ProviderError with appropriate classification.
	NormalizeError(err error) ProviderError

	// Health performs a health check against the given source.
	Health(ctx context.Context, source ResolvedSource) HealthResult
}
