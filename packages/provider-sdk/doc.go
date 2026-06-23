// Package provider_sdk defines the core contracts and data types that every
// upstream AI provider adapter must implement for the Lanyu Token Gateway.
//
// The central abstraction is the ProviderAdapter interface (see §5.2 of the
// full implementation spec). Each adapter wraps a third-party AI API (OpenAI,
// Anthropic, Gemini, DeepSeek, etc.) and normalises it into a canonical
// request/response model so the gateway can route, meter, bill, and audit
// uniformly.
//
// Key types defined here:
//
//   - ProviderAdapter  — the interface every adapter must satisfy.
//   - CanonicalRequest — a provider-agnostic request representation.
//   - CanonicalResponse — a provider-agnostic response representation.
//   - SupportField     — enum describing how well a model supports a feature.
//
// Usage:
//
//	var adapter provider_sdk.ProviderAdapter = myopenai.New(cfg)
//	models := adapter.DiscoverModels(ctx, cred)
package provider_sdk
