package embed

import "net/http"

// config holds shared configuration for embedder implementations.
type config struct {
	model      string
	dim        int
	baseURL    string
	httpClient *http.Client
}

// Option configures an embedder.
type Option func(*config)

// WithModel sets the embedding model name.
func WithModel(model string) Option {
	return func(c *config) { c.model = model }
}

// WithDimension sets the desired output vector dimensionality.
// Not all models support this (e.g. text-embedding-v1/v2 have fixed dims).
func WithDimension(dim int) Option {
	return func(c *config) { c.dim = dim }
}

// WithBaseURL overrides the API base URL.
func WithBaseURL(url string) Option {
	return func(c *config) { c.baseURL = url }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) { c.httpClient = client }
}
