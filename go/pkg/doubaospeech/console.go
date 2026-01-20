package doubaospeech

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	consoleBaseURL = "https://open.volcengineapi.com"
	consoleService = "speech_saas_prod"
	consoleRegion  = "cn-north-1"
)

// Console represents Volcengine Console API client
// Supports two authentication methods:
//  1. API Key (recommended): Use NewConsoleWithAPIKey
//  2. AK/SK signature: Use NewConsole
type Console struct {
	config *consoleConfig
}

// consoleConfig represents console client configuration
type consoleConfig struct {
	// API Key authentication (simple)
	apiKey string

	// AK/SK authentication (advanced)
	accessKey string // Volcengine Access Key
	secretKey string // Volcengine Secret Key

	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// ConsoleOption represents configuration option function for Console
type ConsoleOption func(*consoleConfig)

// NewConsoleWithAPIKey creates Console client using API Key authentication
//
// apiKey is from Volcengine console:
// https://console.volcengine.com/speech/new/setting/apikeys
//
// This is the recommended authentication method for Console API.
func NewConsoleWithAPIKey(apiKey string, opts ...ConsoleOption) *Console {
	config := &consoleConfig{
		apiKey:  apiKey,
		baseURL: consoleBaseURL,
		timeout: defaultTimeout,
	}

	for _, opt := range opts {
		opt(config)
	}

	if config.httpClient == nil {
		config.httpClient = &http.Client{
			Timeout: config.timeout,
		}
	}

	return &Console{
		config: config,
	}
}

// NewConsole creates Console client using AK/SK signature authentication
//
// accessKey and secretKey are from Volcengine IAM console
func NewConsole(accessKey, secretKey string, opts ...ConsoleOption) *Console {
	config := &consoleConfig{
		accessKey: accessKey,
		secretKey: secretKey,
		baseURL:   consoleBaseURL,
		timeout:   defaultTimeout,
	}

	for _, opt := range opts {
		opt(config)
	}

	if config.httpClient == nil {
		config.httpClient = &http.Client{
			Timeout: config.timeout,
		}
	}

	return &Console{
		config: config,
	}
}

// ConsoleWithBaseURL sets console API base URL
func ConsoleWithBaseURL(url string) ConsoleOption {
	return func(c *consoleConfig) {
		c.baseURL = url
	}
}

// ConsoleWithHTTPClient sets custom HTTP client
func ConsoleWithHTTPClient(client *http.Client) ConsoleOption {
	return func(c *consoleConfig) {
		c.httpClient = client
	}
}

// ConsoleWithTimeout sets request timeout
func ConsoleWithTimeout(timeout time.Duration) ConsoleOption {
	return func(c *consoleConfig) {
		c.timeout = timeout
	}
}

// ListTimbresRequest represents timbre list request
type ListTimbresRequest struct {
	PageNumber int    `json:"PageNumber,omitempty"`
	PageSize   int    `json:"PageSize,omitempty"`
	TimbreType string `json:"TimbreType,omitempty"`
}

// ListTimbresResponse represents timbre list response
type ListTimbresResponse struct {
	Timbres []TimbreInfo `json:"Timbres"`
}

// TimbreInfo represents big model timbre information
type TimbreInfo struct {
	SpeakerID   string             `json:"SpeakerID"`
	TimbreInfos []TimbreDetailInfo `json:"TimbreInfos"`
}

// TimbreDetailInfo represents timbre detail info
type TimbreDetailInfo struct {
	SpeakerName string           `json:"SpeakerName"`
	Gender      string           `json:"Gender"`
	Age         string           `json:"Age"`
	Categories  []TimbreCategory `json:"Categories"`
	Emotions    []TimbreEmotion  `json:"Emotions"`
}

// TimbreCategory represents timbre category
type TimbreCategory struct {
	Category string `json:"Category"`
}

// TimbreEmotion represents timbre emotion
type TimbreEmotion struct {
	Emotion     string `json:"Emotion"`
	EmotionType string `json:"EmotionType"`
	DemoText    string `json:"DemoText"`
	DemoURL     string `json:"DemoURL"`
}

// ListSpeakersRequest represents speaker list request
type ListSpeakersRequest struct {
	PageNumber  int    `json:"PageNumber,omitempty"`
	PageSize    int    `json:"PageSize,omitempty"`
	SpeakerType string `json:"SpeakerType,omitempty"`
	Language    string `json:"Language,omitempty"`
}

// ListSpeakersResponse represents speaker list response
type ListSpeakersResponse struct {
	Total    int           `json:"Total"`
	Speakers []SpeakerInfo `json:"Speakers"`
}

// SpeakerInfo represents speaker information (new API)
type SpeakerInfo struct {
	ID        string `json:"ID"`
	VoiceType string `json:"VoiceType"`
	Name      string `json:"Name"`
	Avatar    string `json:"Avatar"`
	Gender    string `json:"Gender"`
	Age       string `json:"Age"`
	TrialURL  string `json:"TrialURL,omitempty"`
}

// VoiceCloneTrainStatus represents voice clone training status
type VoiceCloneTrainStatus struct {
	SpeakerID     string `json:"SpeakerID"`
	InstanceNO    string `json:"InstanceNO"`
	IsActivatable bool   `json:"IsActivatable"`
	State         string `json:"State"`
	DemoAudio     string `json:"DemoAudio,omitempty"`
	Version       string `json:"Version"`
	CreateTime    int64  `json:"CreateTime"`
	ExpireTime    int64  `json:"ExpireTime"`
	Alias         string `json:"Alias,omitempty"`
	ResourceID    string `json:"ResourceID"`
}

// ListVoiceCloneStatusRequest represents list voice clone status request
type ListVoiceCloneStatusRequest struct {
	AppID      string `json:"AppID"`
	PageNumber int    `json:"PageNumber,omitempty"`
	PageSize   int    `json:"PageSize,omitempty"`
	Status     string `json:"Status,omitempty"`
}

// ListVoiceCloneStatusResponse represents list voice clone status response
type ListVoiceCloneStatusResponse struct {
	Total    int                     `json:"Total"`
	Statuses []VoiceCloneTrainStatus `json:"Statuses"`
}

// ListTimbres lists available TTS timbres (big model)
// API: ListBigModelTTSTimbres, Version: 2025-05-20
// Doc: https://www.volcengine.com/docs/6561/1770994
func (c *Console) ListTimbres(ctx context.Context, req *ListTimbresRequest) (*ListTimbresResponse, error) {
	var resp ListTimbresResponse
	if err := c.doRequest(ctx, "ListBigModelTTSTimbres", "2025-05-20", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListSpeakers lists available speakers (new API)
// API: ListSpeakers, Version: 2025-05-20
// Doc: https://www.volcengine.com/docs/6561/2160690
func (c *Console) ListSpeakers(ctx context.Context, req *ListSpeakersRequest) (*ListSpeakersResponse, error) {
	var resp ListSpeakersResponse
	if err := c.doRequest(ctx, "ListSpeakers", "2025-05-20", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListVoiceCloneStatus lists voice clone training status
func (c *Console) ListVoiceCloneStatus(ctx context.Context, req *ListVoiceCloneStatusRequest) (*ListVoiceCloneStatusResponse, error) {
	var resp ListVoiceCloneStatusResponse
	if err := c.doRequest(ctx, "ListMegaTTSTrainStatus", "2023-11-07", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// doRequest makes a request to Volcengine OpenAPI
func (c *Console) doRequest(ctx context.Context, action, version string, body any, result any) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	// Build URL
	u, err := url.Parse(c.config.baseURL)
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("Action", action)
	q.Set("Version", version)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Set authentication
	if c.config.apiKey != "" {
		// API Key authentication (simple)
		req.Header.Set("x-api-key", c.config.apiKey)
	} else if c.config.accessKey != "" && c.config.secretKey != "" {
		// AK/SK signature authentication
		if err := c.signRequest(req, bodyBytes); err != nil {
			return fmt.Errorf("sign request: %w", err)
		}
	} else {
		return fmt.Errorf("no authentication configured: use API Key or AK/SK")
	}

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Parse response
	var apiResp struct {
		ResponseMetadata struct {
			RequestID string `json:"RequestId"`
			Action    string `json:"Action"`
			Version   string `json:"Version"`
			Error     *struct {
				Code    string `json:"Code"`
				CodeN   int    `json:"CodeN"`
				Message string `json:"Message"`
			} `json:"Error,omitempty"`
		} `json:"ResponseMetadata"`
		Result json.RawMessage `json:"Result"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	if apiResp.ResponseMetadata.Error != nil {
		return &Error{
			Code:    apiResp.ResponseMetadata.Error.CodeN,
			Message: apiResp.ResponseMetadata.Error.Message,
		}
	}

	if result != nil && len(apiResp.Result) > 0 {
		if err := json.Unmarshal(apiResp.Result, result); err != nil {
			return fmt.Errorf("unmarshal result: %w", err)
		}
	}

	return nil
}

// signRequest signs the request using Volcengine V4 signature
func (c *Console) signRequest(req *http.Request, body []byte) error {
	now := time.Now().UTC()
	dateStr := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")

	// Set required headers
	req.Header.Set("X-Date", dateStr)
	req.Header.Set("Host", req.URL.Host)

	// Calculate content hash
	contentHash := sha256Hex(body)
	req.Header.Set("X-Content-Sha256", contentHash)

	// Build canonical request
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQueryString := req.URL.RawQuery

	// Signed headers
	signedHeaders := []string{"content-type", "host", "x-content-sha256", "x-date"}
	sort.Strings(signedHeaders)

	var canonicalHeaders strings.Builder
	for _, h := range signedHeaders {
		value := req.Header.Get(h)
		if h == "host" {
			value = req.URL.Host
		}
		canonicalHeaders.WriteString(strings.ToLower(h))
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(strings.TrimSpace(value))
		canonicalHeaders.WriteString("\n")
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders.String(),
		strings.Join(signedHeaders, ";"),
		contentHash,
	)

	// Build string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/request", shortDate, consoleRegion, consoleService)
	stringToSign := fmt.Sprintf("HMAC-SHA256\n%s\n%s\n%s",
		dateStr,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	// Calculate signature
	kDate := hmacSHA256([]byte(c.config.secretKey), shortDate)
	kRegion := hmacSHA256(kDate, consoleRegion)
	kService := hmacSHA256(kRegion, consoleService)
	kSigning := hmacSHA256(kService, "request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	// Build authorization header
	authorization := fmt.Sprintf("HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.config.accessKey,
		credentialScope,
		strings.Join(signedHeaders, ";"),
		signature,
	)

	req.Header.Set("Authorization", authorization)

	return nil
}

// sha256Hex calculates SHA256 hash and returns hex string
func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// hmacSHA256 calculates HMAC-SHA256
func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
