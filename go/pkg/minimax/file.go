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
// The purpose parameter is required and specifies the file category to list.
// Valid values: voice_clone, prompt_audio, t2a_async_input
func (s *FileService) List(ctx context.Context, purpose FilePurpose) (*FileListResponse, error) {
	path := "/v1/files/list?purpose=" + url.QueryEscape(string(purpose))

	var resp struct {
		Files    []FileInfo `json:"files"`
		BaseResp *baseResp  `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &FileListResponse{
		Files: resp.Files,
	}, nil
}

// Get retrieves information about a specific file.
func (s *FileService) Get(ctx context.Context, fileID string) (*FileInfo, error) {
	var resp struct {
		File     FileInfo  `json:"file"`
		BaseResp *baseResp `json:"base_resp"`
	}

	path := "/v1/files/retrieve?file_id=" + url.QueryEscape(fileID)
	err := s.client.http.request(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp.File, nil
}

// Download downloads a file's content.
//
// The returned io.ReadCloser must be closed by the caller.
func (s *FileService) Download(ctx context.Context, fileID string) (io.ReadCloser, error) {
	downloadURL := s.client.config.baseURL + "/v1/files/retrieve_content?file_id=" + url.QueryEscape(fileID)

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
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
//
// The purpose parameter must match the purpose used when uploading the file.
func (s *FileService) Delete(ctx context.Context, fileID string, purpose FilePurpose) error {
	// Convert fileID string to int64 (API requires numeric file_id)
	var fileIDNum int64
	if _, err := fmt.Sscanf(fileID, "%d", &fileIDNum); err != nil {
		return fmt.Errorf("invalid file_id format: %s", fileID)
	}

	req := struct {
		FileID  int64  `json:"file_id"`
		Purpose string `json:"purpose"`
	}{
		FileID:  fileIDNum,
		Purpose: string(purpose),
	}
	return s.client.http.request(ctx, "POST", "/v1/files/delete", req, nil)
}
