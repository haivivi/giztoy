package minimax

import (
	"net/http"
	"time"
)

const (
	// DefaultBaseURL is the default MiniMax API base URL.
	DefaultBaseURL = "https://api.minimax.chat"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second

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
	timeout    time.Duration
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
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		c.timeout = timeout
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
//
// Example:
//
//	client := minimax.NewClient("your-api-key")
//	client := minimax.NewClient("your-api-key", minimax.WithTimeout(60*time.Second))
func NewClient(apiKey string, opts ...Option) *Client {
	cfg := &clientConfig{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{
			Timeout: cfg.timeout,
		}
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
