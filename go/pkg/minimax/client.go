package minimax

import (
	"net/http"
)

const (
	// DefaultBaseURL is the default MiniMax API base URL (China).
	DefaultBaseURL = "https://api.minimaxi.com"

	// BaseURLGlobal is the MiniMax API base URL for global/overseas users.
	BaseURLGlobal = "https://api.minimaxi.chat"

	// DefaultMaxRetries is the default maximum number of retries.
	DefaultMaxRetries = 3
)

// Client is the MiniMax API client.
type Client struct {
	// Text provides text generation (chat completion) operations.
	Text *TextService

	// Speech provides speech synthesis operations.
	Speech *SpeechService

	// Voice provides voice management operations.
	Voice *VoiceService

	// Video provides video generation operations.
	Video *VideoService

	// Image provides image generation operations.
	Image *ImageService

	// Music provides music generation operations.
	Music *MusicService

	// File provides file management operations.
	File *FileService

	config *clientConfig
	http   *httpClient
}

// clientConfig holds the client configuration.
type clientConfig struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
}

// Option is a function that configures the client.
type Option func(*clientConfig)

// WithBaseURL sets a custom base URL for the API.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
// Use this to configure timeouts, transport settings, or other HTTP options.
//
// Example:
//
//	client := minimax.NewClient(apiKey, minimax.WithHTTPClient(&http.Client{
//	    Timeout: 60 * time.Second,
//	}))
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// WithRetry sets the maximum number of retries for transient errors.
func WithRetry(maxRetries int) Option {
	return func(c *clientConfig) {
		c.maxRetries = maxRetries
	}
}

// NewClient creates a new MiniMax API client.
//
// The apiKey is required and can be obtained from the MiniMax platform.
// Request timeouts should be controlled via context.Context.
//
// NewClient panics if apiKey is empty.
//
// Example:
//
//	client := minimax.NewClient("your-api-key")
//
//	// With timeout per request:
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	result, err := client.Text.CreateChatCompletion(ctx, req)
func NewClient(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		panic("minimax: apiKey must be non-empty")
	}

	cfg := &clientConfig{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		maxRetries: DefaultMaxRetries,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{}
	}

	c := &Client{
		config: cfg,
		http:   newHTTPClient(cfg),
	}

	// Initialize services
	c.Text = newTextService(c)
	c.Speech = newSpeechService(c)
	c.Voice = newVoiceService(c)
	c.Video = newVideoService(c)
	c.Image = newImageService(c)
	c.Music = newMusicService(c)
	c.File = newFileService(c)

	return c
}

// APIKey returns the configured API key.
func (c *Client) APIKey() string {
	return c.config.apiKey
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string {
	return c.config.baseURL
}
