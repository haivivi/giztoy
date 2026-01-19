package dashscope

import (
	"errors"
	"fmt"
)

// Common error codes from DashScope.
const (
	// Authentication errors
	ErrCodeInvalidAPIKey     = "InvalidApiKey"
	ErrCodeAccessDenied      = "AccessDenied"
	ErrCodeWorkspaceNotFound = "WorkspaceNotFound"

	// Rate limiting
	ErrCodeRateLimitExceeded = "RateLimitExceeded"
	ErrCodeQuotaExceeded     = "QuotaExceeded"

	// Request errors
	ErrCodeInvalidParameter = "InvalidParameter"
	ErrCodeModelNotFound    = "ModelNotFound"

	// Server errors
	ErrCodeInternalError = "InternalError"
	ErrCodeServiceBusy   = "ServiceBusy"
)

// Error represents a DashScope API error.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
	HTTPStatus int    `json:"-"`
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("dashscope: %s - %s (request_id=%s, http_status=%d)",
			e.Code, e.Message, e.RequestID, e.HTTPStatus)
	}
	return fmt.Sprintf("dashscope: %s - %s (http_status=%d)",
		e.Code, e.Message, e.HTTPStatus)
}

// IsRateLimit checks if the error is due to rate limiting.
func (e *Error) IsRateLimit() bool {
	return e.Code == ErrCodeRateLimitExceeded || e.Code == ErrCodeQuotaExceeded
}

// IsAuth checks if the error is an authentication error.
func (e *Error) IsAuth() bool {
	return e.Code == ErrCodeInvalidAPIKey || e.Code == ErrCodeAccessDenied
}

// IsServerError checks if the error is a server-side error.
func (e *Error) IsServerError() bool {
	return e.Code == ErrCodeInternalError || e.Code == ErrCodeServiceBusy
}

// Retryable checks if the request can be retried.
func (e *Error) Retryable() bool {
	return e.IsRateLimit() || e.IsServerError()
}

// AsError attempts to cast an error to *Error.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
