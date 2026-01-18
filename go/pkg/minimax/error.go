package minimax

import (
	"errors"
	"fmt"
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
	return e.StatusCode == 1002 || e.HTTPStatus == 429
}

// IsInvalidAPIKey returns true if this is an invalid API key error.
func (e *Error) IsInvalidAPIKey() bool {
	return e.StatusCode == 1001 || e.HTTPStatus == 401
}

// IsInsufficientQuota returns true if this is an insufficient quota error.
func (e *Error) IsInsufficientQuota() bool {
	return e.StatusCode == 1003
}

// IsInvalidRequest returns true if this is an invalid request error.
func (e *Error) IsInvalidRequest() bool {
	return e.StatusCode >= 2000 && e.StatusCode < 3000
}

// IsServerError returns true if this is a server-side error.
func (e *Error) IsServerError() bool {
	return e.StatusCode >= 5000 || e.HTTPStatus >= 500
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
