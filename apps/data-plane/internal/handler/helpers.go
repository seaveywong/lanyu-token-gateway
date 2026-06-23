package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	provider_sdk "github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk"
	"github.com/seaveywong/lanyu-token-gateway/packages/contracts"
)

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to encode JSON response", slog.String("error", err.Error()))
		}
	}
}

// respondError writes a structured error response.
func respondError(w http.ResponseWriter, status int, code contracts.ErrorCode, message, requestID string) {
	respondJSON(w, status, contracts.GatewayError{
		Code:      code,
		Message:   message,
		Type:      "api_error",
		RequestID: requestID,
	})
}

// respondGatewayError writes a GatewayError as a JSON response.
func respondGatewayError(w http.ResponseWriter, err contracts.GatewayError) {
	status := err.Code.HTTPStatus()
	if status == 0 {
		status = http.StatusInternalServerError
	}
	respondJSON(w, status, err)
}

// decodeJSON decodes the request body as JSON into the given value.
// Returns an error if the body is invalid or the Content-Type is not application/json.
func decodeJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return json.NewDecoder(nil).Decode(v) // will return io.EOF
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// providerErrorToGatewayError converts a provider_sdk.ProviderError to a contracts.GatewayError.
func providerErrorToGatewayError(pe provider_sdk.ProviderError, requestID string) contracts.GatewayError {
	// Map provider error to gateway error based on status code and error code
	switch {
	case pe.StatusCode == http.StatusTooManyRequests:
		return contracts.GatewayError{
			Code:      contracts.ErrRateLimitExceeded,
			Message:   pe.Message,
			Type:      "upstream_error",
			RequestID: requestID,
		}
	case pe.StatusCode == http.StatusGatewayTimeout || pe.StatusCode == http.StatusRequestTimeout:
		return contracts.GatewayError{
			Code:      contracts.ErrUpstreamTimeout,
			Message:   pe.Message,
			Type:      "upstream_error",
			RequestID: requestID,
		}
	case pe.StatusCode >= 500:
		return contracts.GatewayError{
			Code:      contracts.ErrProviderUnavailable,
			Message:   pe.Message,
			Type:      "upstream_error",
			RequestID: requestID,
		}
	case pe.StatusCode >= 400:
		return contracts.GatewayError{
			Code:      contracts.ErrInvalidRequest,
			Message:   pe.Message,
			Type:      "upstream_error",
			RequestID: requestID,
		}
	default:
		return contracts.GatewayError{
			Code:      contracts.ErrInternalError,
			Message:   pe.Message,
			Type:      "upstream_error",
			RequestID: requestID,
		}
	}
}
