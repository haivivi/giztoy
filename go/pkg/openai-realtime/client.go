package openairealtime

import (
	"context"
	"net/http"
)

const (
	// DefaultWebSocketURL is the default WebSocket endpoint.
	DefaultWebSocketURL = "wss://api.openai.com/v1/realtime"

	// DefaultHTTPURL is the default HTTP endpoint (for WebRTC session creation).
	DefaultHTTPURL = "https://api.openai.com/v1/realtime"
)

// Client is the OpenAI Realtime API client.
type Client struct {
	config *clientConfig
}

// clientConfig holds the client configuration.
type clientConfig struct {
	apiKey       string
	organization string
	project      string
	wsURL        string
	httpURL      string
	httpClient   *http.Client
}

// Option configures the Client.
type Option func(*clientConfig)

// NewClient creates a new OpenAI Realtime client.
//
// The apiKey is required and can be obtained from:
// https://platform.openai.com/api-keys
func NewClient(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		panic("openai-realtime: API key is required")
	}

	cfg := &clientConfig{
		apiKey:     apiKey,
		wsURL:      DefaultWebSocketURL,
		httpURL:    DefaultHTTPURL,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &Client{config: cfg}
}

// WithOrganization sets the organization ID for API requests.
func WithOrganization(orgID string) Option {
	return func(c *clientConfig) {
		c.organization = orgID
	}
}

// WithProject sets the project ID for API requests.
func WithProject(projectID string) Option {
	return func(c *clientConfig) {
		c.project = projectID
	}
}

// WithWebSocketURL sets the WebSocket URL.
func WithWebSocketURL(url string) Option {
	return func(c *clientConfig) {
		c.wsURL = url
	}
}

// WithHTTPURL sets the HTTP URL for WebRTC session creation.
func WithHTTPURL(url string) Option {
	return func(c *clientConfig) {
		c.httpURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// ConnectWebSocket establishes a WebSocket connection to the Realtime API.
// This is suitable for server-side applications.
func (c *Client) ConnectWebSocket(ctx context.Context, config *ConnectConfig) (Session, error) {
	return c.connectWebSocket(ctx, config)
}

// ConnectWebRTC establishes a WebRTC connection to the Realtime API.
// This is suitable for client-side applications with lower latency requirements.
// The returned WebRTCSession provides additional methods for accessing audio tracks.
func (c *Client) ConnectWebRTC(ctx context.Context, config *ConnectConfig) (*WebRTCSession, error) {
	return c.connectWebRTC(ctx, config)
}
