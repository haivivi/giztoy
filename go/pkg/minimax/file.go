package minimax

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// FileService provides file management operations.
type FileService struct {
	client *Client
}

// newFileService creates a new file service.
func newFileService(client *Client) *FileService {
	return &FileService{client: client}
}

// Upload uploads a file.
//
// The purpose parameter specifies the intended use of the file.
func (s *FileService) Upload(ctx context.Context, file io.Reader, filename string, purpose FilePurpose) (*FileInfo, error) {
	var resp struct {
		File     FileInfo  `json:"file"`
		BaseResp *baseResp `json:"base_resp"`
	}

	fields := map[string]string{
		"purpose": string(purpose),
	}

	err := s.client.http.uploadFile(ctx, "/v1/files/upload", file, filename, fields, &resp)
	if err != nil {
		return nil, err
	}

	return &resp.File, nil
}

// List returns a list of files.
//
// Use FileListOptions to filter and paginate results.
func (s *FileService) List(ctx context.Context, opts *FileListOptions) (*FileListResponse, error) {
	path := "/v1/files"
	if opts != nil {
		query := url.Values{}
		if opts.Purpose != "" {
			query.Set("purpose", string(opts.Purpose))
		}
		if opts.Limit > 0 {
			query.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.After != "" {
			query.Set("after", opts.After)
		}
		if len(query) > 0 {
			path += "?" + query.Encode()
		}
	}

	var resp struct {
		Data     []FileInfo `json:"data"`
		HasMore  bool       `json:"has_more"`
		BaseResp *baseResp  `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &FileListResponse{
		Data:    resp.Data,
		HasMore: resp.HasMore,
	}, nil
}

// Get retrieves information about a specific file.
func (s *FileService) Get(ctx context.Context, fileID string) (*FileInfo, error) {
	var resp struct {
		File     FileInfo  `json:"file"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "GET", "/v1/files/"+fileID, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp.File, nil
}

// Download downloads a file's content.
//
// The returned io.ReadCloser must be closed by the caller.
func (s *FileService) Download(ctx context.Context, fileID string) (io.ReadCloser, error) {
	url := s.client.config.baseURL + "/v1/files/" + fileID + "/content"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.client.config.apiKey)

	resp, err := s.client.config.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, s.client.http.handleErrorResponse(resp)
	}

	return resp.Body, nil
}

// Delete deletes a file.
func (s *FileService) Delete(ctx context.Context, fileID string) error {
	return s.client.http.request(ctx, "POST", "/v1/files/"+fileID+"/delete", nil, nil)
}
