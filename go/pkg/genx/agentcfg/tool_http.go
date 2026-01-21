package agentcfg

import (
	"encoding/json"
	"fmt"
)

// HTTPTool is an HTTP request tool.
//
// Validation:
//   - Inherits ToolBase validation (Name required)
//   - Method: validated via HTTPMethod unmarshal
//   - Endpoint: required, non-empty URL string
type HTTPTool struct {
	ToolBase          `msgpack:",inline"`
	Method            HTTPMethod        `json:"method" msgpack:"method"`                                                // GET, POST, PUT, DELETE, PATCH
	Endpoint          string            `json:"endpoint" msgpack:"endpoint"`                                            // HTTP endpoint URL
	Headers           map[string]string `json:"headers,omitzero" msgpack:"headers,omitempty"`                           // custom headers (values support ${ENV_VAR})
	Auth              *HTTPAuth         `json:"auth,omitzero" msgpack:"auth,omitempty"`                                 // authentication
	ReqBodyJQ         *JQExpr           `json:"req_body_jq,omitzero" msgpack:"req_body_jq,omitempty"`                   // jq expression to build request body
	RespBodyJQ        *JQExpr           `json:"resp_body_jq,omitzero" msgpack:"resp_body_jq,omitempty"`                 // jq expression to extract response
	MaxResponseSizeMB int64             `json:"max_response_size_mb,omitzero" msgpack:"max_response_size_mb,omitempty"` // max response body size in MB (default 1)
}

// validate checks if the HTTPTool fields are valid.
func (t *HTTPTool) validate() error {
	if t.Name == "" {
		return fmt.Errorf("http tool: name is required")
	}
	if t.Endpoint == "" {
		return fmt.Errorf("tool %s: endpoint is required", t.Name)
	}
	if t.Auth != nil {
		if err := t.Auth.validate(); err != nil {
			return fmt.Errorf("tool %s: %w", t.Name, err)
		}
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *HTTPTool) UnmarshalJSON(data []byte) error {
	type Alias HTTPTool
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*t = HTTPTool(alias)
	return t.validate()
}

// HTTPAuth defines HTTP authentication.
//
// Validation:
//   - Type: validated via HTTPAuthType unmarshal
//   - Token: required when Type is "bearer"
type HTTPAuth struct {
	Type  HTTPAuthType `json:"type" msgpack:"type"`                      // bearer, basic
	Token string       `json:"token,omitzero" msgpack:"token,omitempty"` // token or ${ENV_VAR}
}

// validate checks if the HTTPAuth fields are valid.
func (a *HTTPAuth) validate() error {
	if a.Type == HTTPAuthTypeBearer && a.Token == "" {
		return fmt.Errorf("auth: token is required for bearer authentication")
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (a *HTTPAuth) UnmarshalJSON(data []byte) error {
	type Alias HTTPAuth
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*a = HTTPAuth(alias)
	return a.validate()
}
