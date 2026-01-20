package dashscope

import (
	"net/http"
)

const (
	// DefaultRealtimeURL is the default WebSocket endpoint for Qwen-Omni-Realtime.
	DefaultRealtimeURL = "wss://dashscope.aliyuncs.com/api-ws/v1/realtime"

	// DefaultHTTPBaseURL is the default HTTP endpoint.
	DefaultHTTPBaseURL = "https://dashscope.aliyuncs.com"
)

// Client is the DashScope API client.
type Client struct {
	Realtime *RealtimeService

	config *clientConfig
}

// clientConfig holds the client configuration.
type clientConfig struct {
	apiKey      string
	workspaceID string
	baseURL     string
	httpBaseURL string
	httpClient  *http.Client
}

// Option configures the Client.
type Option func(*clientConfig)

// NewClient creates a new DashScope client.
//
// The apiKey is required and can be obtained from:
// https://bailian.console.aliyun.com/?apiKey=1
func NewClient(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		panic("dashscope: API key is required")
	}

	cfg := &clientConfig{
		apiKey:      apiKey,
		baseURL:     DefaultRealtimeURL,
		httpBaseURL: DefaultHTTPBaseURL,
		httpClient:  http.DefaultClient,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	c := &Client{config: cfg}
	c.Realtime = &RealtimeService{client: c}
	return c
}

// WithWorkspace sets the workspace ID for resource isolation.
func WithWorkspace(workspaceID string) Option {
	return func(c *clientConfig) {
		c.workspaceID = workspaceID
	}
}

// WithBaseURL sets the WebSocket base URL.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.baseURL = url
	}
}

// WithHTTPBaseURL sets the HTTP base URL.
func WithHTTPBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.httpBaseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}
