package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/seaveywong/lanyu-token-gateway/packages/contracts"
)

// respondJSON writes a JSON response with the given HTTP status code.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// Encoding failed; log and write a fallback error.
			http.Error(w, `{"code":"internal_error","message":"failed to encode response"}`, http.StatusInternalServerError)
		}
	}
}

// respondError writes a standardized JSON error response using the GatewayError
// format defined in packages/contracts.
func respondError(w http.ResponseWriter, status int, code, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(contracts.GatewayError{
		Code:      contracts.ErrorCode(code),
		Message:   message,
		Type:      "error",
		RequestID: requestID,
	})
}

// respondGatewayError writes a GatewayError struct as a JSON response, inferring
// the HTTP status from the error code.
func respondGatewayError(w http.ResponseWriter, err contracts.GatewayError) {
	status := err.Code.HTTPStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(err)
}

// decodeJSON decodes the request body JSON into the given struct.
// Returns nil on success, or sends a 400 error response and returns the decode
// error on failure.
func decodeJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	return nil
}

// getPageParams extracts page and pageSize from the query string with sensible
// defaults (page=1, pageSize=20) and bounds checking.
func getPageParams(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}
	return page, pageSize
}

// requestID extracts the X-Request-ID header value or returns an empty string.
func requestID(r *http.Request) string {
	return r.Header.Get("X-Request-ID")
}
