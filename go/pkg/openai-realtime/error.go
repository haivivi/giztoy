package openairealtime

import "fmt"

// Error represents an API error from OpenAI Realtime.
type Error struct {
	// Type is the error type (e.g., "invalid_request_error").
	Type string `json:"type,omitzero"`

	// Code is the error code (e.g., "invalid_value").
	Code string `json:"code,omitzero"`

	// Message is the human-readable error message.
	Message string `json:"message,omitzero"`

	// Param is the parameter that caused the error, if applicable.
	Param string `json:"param,omitzero"`

	// EventID is the ID of the event that caused the error.
	EventID string `json:"event_id,omitzero"`

	// HTTPStatus is the HTTP status code, if applicable.
	HTTPStatus int `json:"-"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("openai-realtime: %s: %s", e.Code, e.Message)
	}
	if e.Type != "" {
		return fmt.Sprintf("openai-realtime: %s: %s", e.Type, e.Message)
	}
	return fmt.Sprintf("openai-realtime: %s", e.Message)
}

// EventError contains error information from error events.
type EventError struct {
	Type    string `json:"type,omitzero"`
	Code    string `json:"code,omitzero"`
	Message string `json:"message,omitzero"`
	Param   string `json:"param,omitzero"`
	EventID string `json:"event_id,omitzero"`
}

// ToError converts EventError to Error.
func (e *EventError) ToError() *Error {
	return &Error{
		Type:    e.Type,
		Code:    e.Code,
		Message: e.Message,
		Param:   e.Param,
		EventID: e.EventID,
	}
}
