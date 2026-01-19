package minimax

import (
	"context"
	"fmt"
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
// VoiceTypeCloning for cloned voices, or VoiceTypeGeneration for designed voices.
func (s *VoiceService) List(ctx context.Context, voiceType VoiceType) (*VoiceListResponse, error) {
	var resp struct {
		SystemVoices     []VoiceInfo `json:"system_voice"`
		CloningVoices    []VoiceInfo `json:"voice_cloning"`
		GenerationVoices []VoiceInfo `json:"voice_generation"`
		BaseResp         *baseResp   `json:"base_resp"`
	}

	req := struct {
		VoiceType VoiceType `json:"voice_type"`
	}{
		VoiceType: voiceType,
	}

	err := s.client.http.request(ctx, "POST", "/v1/get_voice", req, &resp)
	if err != nil {
		return nil, err
	}

	return &VoiceListResponse{
		SystemVoices:     resp.SystemVoices,
		CloningVoices:    resp.CloningVoices,
		GenerationVoices: resp.GenerationVoices,
	}, nil
}

// Delete deletes a custom voice.
//
// The voiceType must be either VoiceTypeCloning or VoiceTypeGeneration.
// System voices cannot be deleted.
func (s *VoiceService) Delete(ctx context.Context, voiceID string, voiceType VoiceType) error {
	// Validate voice type - only custom voices can be deleted
	if voiceType != VoiceTypeCloning && voiceType != VoiceTypeGeneration {
		return fmt.Errorf("invalid voice_type: must be %q or %q, got %q",
			VoiceTypeCloning, VoiceTypeGeneration, voiceType)
	}

	req := struct {
		VoiceID   string    `json:"voice_id"`
		VoiceType VoiceType `json:"voice_type"`
	}{
		VoiceID:   voiceID,
		VoiceType: voiceType,
	}

	return s.client.http.request(ctx, "POST", "/v1/delete_voice", req, nil)
}

// UploadCloneAudio uploads an audio file for voice cloning.
//
// The returned file_id can be used in the Clone method.
func (s *VoiceService) UploadCloneAudio(ctx context.Context, file io.Reader, filename string) (*UploadResponse, error) {
	var resp struct {
		File struct {
			FileID FlexibleID `json:"file_id"`
		} `json:"file"`
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
		FileID: resp.File.FileID.String(),
	}, nil
}

// UploadDemoAudio uploads a demo audio file for voice cloning.
//
// This is optional and can enhance the cloning quality.
func (s *VoiceService) UploadDemoAudio(ctx context.Context, file io.Reader, filename string) (*UploadResponse, error) {
	var resp struct {
		File struct {
			FileID FlexibleID `json:"file_id"`
		} `json:"file"`
		BaseResp *baseResp `json:"base_resp"`
	}

	fields := map[string]string{
		"purpose": string(FilePurposePromptAudio),
	}

	err := s.client.http.uploadFile(ctx, "/v1/files/upload", file, filename, fields, &resp)
	if err != nil {
		return nil, err
	}

	return &UploadResponse{
		FileID: resp.File.FileID.String(),
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

	err := s.client.http.request(ctx, "POST", "/v1/voice_clone", req, &resp)
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

	err := s.client.http.request(ctx, "POST", "/v1/voice_design", req, &resp)
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
