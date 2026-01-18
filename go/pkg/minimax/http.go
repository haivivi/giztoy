package minimax

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// httpClient handles HTTP communication with the MiniMax API.
type httpClient struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	maxRetries int
}

// newHTTPClient creates a new HTTP client.
func newHTTPClient(cfg *clientConfig) *httpClient {
	return &httpClient{
		client:     cfg.httpClient,
		baseURL:    cfg.baseURL,
		apiKey:     cfg.apiKey,
		maxRetries: cfg.maxRetries,
	}
}

// apiResponse is the common response wrapper from MiniMax API.
type apiResponse struct {
	BaseResp *baseResp       `json:"base_resp,omitempty"`
	Data     json.RawMessage `json:"-"` // Will be unmarshaled separately
}

type baseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// request makes an HTTP request to the API with retry support.
func (h *httpClient) request(ctx context.Context, method, path string, body any, result any) error {
	var bodyData []byte
	if body != nil {
		var err error
		bodyData, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, ...
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := h.doRequest(ctx, method, path, bodyData, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if apiErr, ok := AsError(err); ok {
			if !apiErr.Retryable() {
				return err
			}
		} else {
			// Non-API errors (network errors) are retryable
			continue
		}
	}

	return lastErr
}

// doRequest performs a single HTTP request.
func (h *httpClient) doRequest(ctx context.Context, method, path string, bodyData []byte, result any) error {
	url := h.baseURL + path

	var bodyReader io.Reader
	if bodyData != nil {
		bodyReader = bytes.NewReader(bodyData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	h.setHeaders(req)
	if bodyData != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	return h.handleResponse(resp, result)
}

// requestStream makes a streaming HTTP request to the API.
func (h *httpClient) requestStream(ctx context.Context, method, path string, body any) (*http.Response, error) {
	url := h.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	h.setHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	// Check for error response (non-streaming)
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, h.handleErrorResponse(resp)
	}

	return resp, nil
}

// uploadFile uploads a file using multipart form data with streaming.
// This avoids loading the entire file into memory.
func (h *httpClient) uploadFile(ctx context.Context, path string, file io.Reader, filename string, fields map[string]string, result any) error {
	url := h.baseURL + path

	// Use io.Pipe for streaming upload to avoid loading entire file into memory
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write multipart data in a goroutine
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()

		// Add file field
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			errCh <- fmt.Errorf("create form file: %w", err)
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			errCh <- fmt.Errorf("copy file: %w", err)
			return
		}

		// Add other fields
		for key, value := range fields {
			if err := writer.WriteField(key, value); err != nil {
				errCh <- fmt.Errorf("write field %s: %w", key, err)
				return
			}
		}

		if err := writer.Close(); err != nil {
			errCh <- fmt.Errorf("close writer: %w", err)
			return
		}

		errCh <- nil
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		pr.Close()
		return fmt.Errorf("create request: %w", err)
	}

	h.setHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors from the goroutine
	if writeErr := <-errCh; writeErr != nil {
		return writeErr
	}

	return h.handleResponse(resp, result)
}

// setHeaders sets common headers for API requests.
func (h *httpClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+h.apiKey)
	req.Header.Set("User-Agent", "giztoy-minimax-go/1.0")
}

// handleResponse handles the API response.
func (h *httpClient) handleResponse(resp *http.Response, result any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return h.parseError(body, resp.StatusCode)
	}

	// First check for API-level error in base_resp
	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err == nil {
		if apiResp.BaseResp != nil && apiResp.BaseResp.StatusCode != 0 {
			return &Error{
				StatusCode: apiResp.BaseResp.StatusCode,
				StatusMsg:  apiResp.BaseResp.StatusMsg,
				HTTPStatus: resp.StatusCode,
			}
		}
	}

	// Parse response into result if provided
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// handleErrorResponse handles an error response.
func (h *httpClient) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read error response: %w", err)
	}
	return h.parseError(body, resp.StatusCode)
}

// parseError parses an error response body.
func (h *httpClient) parseError(body []byte, httpStatus int) error {
	var apiResp struct {
		BaseResp *baseResp `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.BaseResp != nil {
		return &Error{
			StatusCode: apiResp.BaseResp.StatusCode,
			StatusMsg:  apiResp.BaseResp.StatusMsg,
			HTTPStatus: httpStatus,
		}
	}

	return &Error{
		StatusCode: httpStatus,
		StatusMsg:  string(body),
		HTTPStatus: httpStatus,
	}
}

// sseReader helps read Server-Sent Events from a response.
type sseReader struct {
	reader  *bufio.Reader
	resp    *http.Response
	onClose func()
}

// newSSEReader creates a new SSE reader.
func newSSEReader(resp *http.Response) *sseReader {
	return &sseReader{
		reader: bufio.NewReader(resp.Body),
		resp:   resp,
	}
}

// readEvent reads the next SSE event.
// Returns (data, isDone, error).
func (r *sseReader) readEvent() ([]byte, bool, error) {
	var data []byte

	for {
		line, err := r.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil, true, nil
			}
			return nil, false, err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			// Empty line marks end of event
			if len(data) > 0 {
				return data, false, nil
			}
			continue
		}

		if bytes.HasPrefix(line, []byte("data:")) {
			eventData := bytes.TrimPrefix(line, []byte("data:"))
			eventData = bytes.TrimSpace(eventData)

			// Check for stream end marker
			if bytes.Equal(eventData, []byte("[DONE]")) {
				return nil, true, nil
			}

			data = eventData
		}
	}
}

// close closes the SSE reader.
func (r *sseReader) close() {
	r.resp.Body.Close()
	if r.onClose != nil {
		r.onClose()
	}
}

// decodeHexAudio decodes hex-encoded audio data.
func decodeHexAudio(hexData string) ([]byte, error) {
	// Remove any whitespace
	hexData = strings.ReplaceAll(hexData, " ", "")
	hexData = strings.ReplaceAll(hexData, "\n", "")

	return hex.DecodeString(hexData)
}
