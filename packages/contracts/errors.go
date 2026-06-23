package contracts

// ErrorCode represents a gateway error code
type ErrorCode string

const (
	ErrInvalidAPIKey         ErrorCode = "invalid_api_key"
	ErrKeyDisabled           ErrorCode = "key_disabled"
	ErrInsufficientBalance   ErrorCode = "insufficient_balance"
	ErrProjectBudgetExceeded ErrorCode = "project_budget_exceeded"
	ErrModelNotAllowed       ErrorCode = "model_not_allowed"
	ErrInvalidRequest        ErrorCode = "invalid_request"
	ErrRequestTooLarge       ErrorCode = "request_too_large"
	ErrRateLimitExceeded     ErrorCode = "rate_limit_exceeded"
	ErrConcurrencyExceeded   ErrorCode = "concurrency_limit_exceeded"
	ErrUpstreamTimeout       ErrorCode = "upstream_timeout"
	ErrProviderUnavailable   ErrorCode = "provider_unavailable"
	ErrFeatureNotSupported   ErrorCode = "feature_not_supported"
	ErrIdempotencyConflict   ErrorCode = "idempotency_conflict"
	ErrInternalError         ErrorCode = "internal_error"
)

// HTTPStatus returns the HTTP status code for an error code
func (c ErrorCode) HTTPStatus() int {
	switch c {
	case ErrInvalidAPIKey:
		return 401
	case ErrKeyDisabled, ErrModelNotAllowed:
		return 403
	case ErrInsufficientBalance, ErrProjectBudgetExceeded:
		return 402
	case ErrInvalidRequest, ErrFeatureNotSupported:
		return 400
	case ErrRequestTooLarge:
		return 413
	case ErrRateLimitExceeded, ErrConcurrencyExceeded:
		return 429
	case ErrUpstreamTimeout:
		return 504
	case ErrProviderUnavailable:
		return 502
	case ErrIdempotencyConflict:
		return 409
	default:
		return 500
	}
}

// GatewayError is the standard error envelope
type GatewayError struct {
	Code              ErrorCode `json:"code"`
	Message           string    `json:"message"`
	Type              string    `json:"type"`
	RequestID         string    `json:"request_id,omitempty"`
	RetryAfterSeconds int       `json:"retry_after_seconds,omitempty"`
}
