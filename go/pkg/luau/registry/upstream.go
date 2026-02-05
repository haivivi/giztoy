package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Upstream represents an upstream registry server.
type Upstream struct {
	// Name is a display name for this upstream.
	Name string `json:"name"`

	// API is the base URL for API requests.
	// Example: "https://registry.giztoy.io/api/v1"
	API string `json:"api"`

	// Download is the base URL for downloading packages.
	// If empty, uses API + "/download".
	Download string `json:"download,omitempty"`

	// Auth contains authentication configuration.
	Auth *UpstreamAuth `json:"auth,omitempty"`

	// Priority determines the order of lookup (lower = higher priority).
	Priority int `json:"priority,omitempty"`

	// Timeout for HTTP requests (default: 30s).
	Timeout time.Duration `json:"timeout,omitempty"`
}

// UpstreamAuth holds authentication configuration.
type UpstreamAuth struct {
	// Type is the authentication type: "token" or "basic".
	Type string `json:"type"`

	// Token is used for token-based auth (Authorization: Bearer <token>).
	Token string `json:"token,omitempty"`

	// Username is used for basic auth.
	Username string `json:"username,omitempty"`

	// Password is used for basic auth.
	Password string `json:"password,omitempty"`
}

// DownloadURL returns the download base URL.
func (u *Upstream) DownloadURL() string {
	if u.Download != "" {
		return u.Download
	}
	return strings.TrimSuffix(u.API, "/") + "/download"
}

// GetTimeout returns the configured timeout or default.
func (u *Upstream) GetTimeout() time.Duration {
	if u.Timeout > 0 {
		return u.Timeout
	}
	return 30 * time.Second
}

// UpstreamResponse holds the response from an upstream query.
type UpstreamResponse struct {
	// Meta contains package metadata.
	Meta *PackageMeta `json:"meta,omitempty"`

	// Versions lists all available versions.
	Versions []string `json:"versions,omitempty"`

	// Tarball is the URL to download the package.
	Tarball string `json:"tarball,omitempty"`

	// Checksum is the expected checksum of the tarball.
	Checksum string `json:"checksum,omitempty"`
}

// UpstreamClient provides methods to interact with an upstream registry.
type UpstreamClient struct {
	upstream *Upstream
	client   *http.Client
}

// NewUpstreamClient creates a client for an upstream registry.
func NewUpstreamClient(upstream *Upstream) *UpstreamClient {
	return &UpstreamClient{
		upstream: upstream,
		client: &http.Client{
			Timeout: upstream.GetTimeout(),
		},
	}
}

// GetPackageMeta fetches metadata for a package version.
func (c *UpstreamClient) GetPackageMeta(ctx context.Context, name, version string) (*UpstreamResponse, error) {
	// Build URL: /api/v1/packages/@scope/name/versions/1.0.0
	u, err := url.JoinPath(c.upstream.API, "packages", name, "versions", version)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream error: %s", resp.Status)
	}

	var result UpstreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListVersions fetches all available versions for a package.
func (c *UpstreamClient) ListVersions(ctx context.Context, name string) ([]string, error) {
	// Build URL: /api/v1/packages/@scope/name
	u, err := url.JoinPath(c.upstream.API, "packages", name)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream error: %s", resp.Status)
	}

	var result UpstreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Versions, nil
}

// DownloadPackage downloads a package tarball.
func (c *UpstreamClient) DownloadPackage(ctx context.Context, tarballURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", tarballURL, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download error: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// setAuth sets authentication headers on a request.
func (c *UpstreamClient) setAuth(req *http.Request) {
	if c.upstream.Auth == nil {
		return
	}

	switch c.upstream.Auth.Type {
	case "token":
		req.Header.Set("Authorization", "Bearer "+c.upstream.Auth.Token)
	case "basic":
		req.SetBasicAuth(c.upstream.Auth.Username, c.upstream.Auth.Password)
	}
}

// FetchPackage fetches and parses a package from the upstream.
func (c *UpstreamClient) FetchPackage(ctx context.Context, name, version string) (*Package, error) {
	// Get metadata
	resp, err := c.GetPackageMeta(ctx, name, version)
	if err != nil {
		return nil, err
	}

	// Build tarball URL if not provided
	tarballURL := resp.Tarball
	if tarballURL == "" {
		tarballURL = fmt.Sprintf("%s/%s/%s.tgz",
			c.upstream.DownloadURL(), name, version)
	}

	// Download tarball
	data, err := c.DownloadPackage(ctx, tarballURL)
	if err != nil {
		return nil, err
	}

	// Verify checksum if provided
	if resp.Checksum != "" {
		if err := VerifyChecksum(data, resp.Checksum); err != nil {
			return nil, err
		}
	}

	// Parse tarball
	return ParseTarball(data)
}
