package minimax

import (
	"errors"
	"fmt"
)

// API error status codes.
const (
	StatusCodeInvalidAPIKey    = 1001
	StatusCodeRateLimit        = 1002
	StatusCodeInsufficientQuota = 1003
	StatusCodeInvalidRequestMin = 2000
	StatusCodeInvalidRequestMax = 2999
	StatusCodeServerErrorMin   = 5000
)

// Error represents a MiniMax API error.
type Error struct {
	// StatusCode is the API error code.
	StatusCode int `json:"status_code"`

	// StatusMsg is the error message.
	StatusMsg string `json:"status_msg"`

	// TraceID is the request trace ID for debugging.
	TraceID string `json:"trace_id"`

	// HTTPStatus is the HTTP status code.
	HTTPStatus int `json:"-"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("minimax: %s (code=%d, trace=%s)", e.StatusMsg, e.StatusCode, e.TraceID)
}

// IsRateLimit returns true if this is a rate limit error.
func (e *Error) IsRateLimit() bool {
	return e.StatusCode == StatusCodeRateLimit || e.HTTPStatus == 429
}

// IsInvalidAPIKey returns true if this is an invalid API key error.
func (e *Error) IsInvalidAPIKey() bool {
	return e.StatusCode == StatusCodeInvalidAPIKey || e.HTTPStatus == 401
}

// IsInsufficientQuota returns true if this is an insufficient quota error.
func (e *Error) IsInsufficientQuota() bool {
	return e.StatusCode == StatusCodeInsufficientQuota
}

// IsInvalidRequest returns true if this is an invalid request error.
func (e *Error) IsInvalidRequest() bool {
	return e.StatusCode >= StatusCodeInvalidRequestMin && e.StatusCode <= StatusCodeInvalidRequestMax
}

// IsServerError returns true if this is a server-side error.
func (e *Error) IsServerError() bool {
	return e.StatusCode >= StatusCodeServerErrorMin || e.HTTPStatus >= 500
}

// Retryable returns true if the request can be retried.
func (e *Error) Retryable() bool {
	return e.IsRateLimit() || e.IsServerError()
}

// AsError extracts *Error from an error.
//
// Example:
//
//	if e, ok := minimax.AsError(err); ok {
//	    if e.IsRateLimit() {
//	        // Handle rate limiting
//	    }
//	}
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
