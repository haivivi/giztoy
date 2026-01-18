package minimax

import (
	"context"
	"io"
)

// VoiceService provides voice management operations.
type VoiceService struct {
	client *Client
}

// newVoiceService creates a new voice service.
func newVoiceService(client *Client) *VoiceService {
	return &VoiceService{client: client}
}

// List returns the list of available voices.
//
// Use VoiceTypeAll to list all voices, VoiceTypeSystem for system voices,
// or VoiceTypeCloning for cloned voices.
func (s *VoiceService) List(ctx context.Context, voiceType VoiceType) (*VoiceListResponse, error) {
	var resp struct {
		Voices   []VoiceInfo `json:"voices"`
		BaseResp *baseResp   `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "GET", "/v1/voice/list?voice_type="+string(voiceType), nil, &resp)
	if err != nil {
		return nil, err
	}

	return &VoiceListResponse{
		Voices: resp.Voices,
	}, nil
}

// Delete deletes a custom voice.
func (s *VoiceService) Delete(ctx context.Context, voiceID string) error {
	req := struct {
		VoiceID string `json:"voice_id"`
	}{
		VoiceID: voiceID,
	}

	return s.client.http.request(ctx, "POST", "/v1/voice/delete", req, nil)
}

// UploadCloneAudio uploads an audio file for voice cloning.
//
// The returned file_id can be used in the Clone method.
func (s *VoiceService) UploadCloneAudio(ctx context.Context, file io.Reader, filename string) (*UploadResponse, error) {
	var resp struct {
		FileID   string    `json:"file_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	fields := map[string]string{
		"purpose": string(FilePurposeVoiceClone),
	}

	err := s.client.http.uploadFile(ctx, "/v1/files/upload", file, filename, fields, &resp)
	if err != nil {
		return nil, err
	}

	return &UploadResponse{
		FileID: resp.FileID,
	}, nil
}

// UploadDemoAudio uploads a demo audio file for voice cloning.
//
// This is optional and can enhance the cloning quality.
func (s *VoiceService) UploadDemoAudio(ctx context.Context, file io.Reader, filename string) (*UploadResponse, error) {
	var resp struct {
		FileID   string    `json:"file_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	fields := map[string]string{
		"purpose": string(FilePurposeVoiceCloneDemo),
	}

	err := s.client.http.uploadFile(ctx, "/v1/files/upload", file, filename, fields, &resp)
	if err != nil {
		return nil, err
	}

	return &UploadResponse{
		FileID: resp.FileID,
	}, nil
}

// Clone performs voice cloning.
//
// The cloned voice is temporary and will be deleted after 7 days of inactivity.
func (s *VoiceService) Clone(ctx context.Context, req *VoiceCloneRequest) (*VoiceCloneResponse, error) {
	var resp struct {
		VoiceID   string    `json:"voice_id"`
		DemoAudio string    `json:"demo_audio"` // hex-encoded
		BaseResp  *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/voice/clone", req, &resp)
	if err != nil {
		return nil, err
	}

	result := &VoiceCloneResponse{
		VoiceID: resp.VoiceID,
	}

	// Decode demo audio if present
	if resp.DemoAudio != "" {
		audio, err := decodeHexAudio(resp.DemoAudio)
		if err != nil {
			return nil, err
		}
		result.DemoAudio = audio
	}

	return result, nil
}

// Design creates a voice from a text description.
//
// The designed voice is temporary and will be deleted after 7 days of inactivity.
func (s *VoiceService) Design(ctx context.Context, req *VoiceDesignRequest) (*VoiceDesignResponse, error) {
	var resp struct {
		VoiceID   string    `json:"voice_id"`
		DemoAudio string    `json:"demo_audio"` // hex-encoded
		BaseResp  *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/voice/design", req, &resp)
	if err != nil {
		return nil, err
	}

	result := &VoiceDesignResponse{
		VoiceID: resp.VoiceID,
	}

	// Decode demo audio if present
	if resp.DemoAudio != "" {
		audio, err := decodeHexAudio(resp.DemoAudio)
		if err != nil {
			return nil, err
		}
		result.DemoAudio = audio
	}

	return result, nil
}
