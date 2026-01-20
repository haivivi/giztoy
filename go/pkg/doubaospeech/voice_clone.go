package doubaospeech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

// VoiceCloneService represents voice cloning service
// VoiceCloneService provides voice cloning functionality
//
// API Documentation: https://www.volcengine.com/docs/6561/1305191
//
// Endpoints:
//   - Upload audio: POST /api/v1/mega_tts/audio/upload
//   - Query status: GET /api/v1/mega_tts/status
//
// Note: List/Delete operations use Console API (see console.go)
type VoiceCloneService struct {
	client *Client
}

// newVoiceCloneService creates voice clone service
func newVoiceCloneService(c *Client) *VoiceCloneService {
	return &VoiceCloneService{client: c}
}

// Train trains custom voice using audio data
//
// The speaker_id should follow the format: S_xxxxxxxxx (e.g., S_TR0rbVuI1)
// Audio requirements:
//   - Duration: 10-60 seconds recommended
//   - Formats: wav, mp3, ogg, pcm
//   - Sample rate: 16kHz or 24kHz
//
// After training completes, use the speaker_id in TTS with:
//   - Cluster: volcano_icl (for ICL 1.0) or volcano_mega (for DiT)
//   - Voice type: your speaker_id
func (s *VoiceCloneService) Train(ctx context.Context, req *VoiceCloneTrainRequest) (*Task[VoiceCloneResult], error) {
	// Audio format - infer from data or use wav as default
	audioFormat := "wav"
	if len(req.AudioData) > 0 && len(req.AudioData[0]) > 0 {
		audioFormat = detectAudioFormat(req.AudioData[0])
	}

	// Model type (1=ICL1.0, 2=DiT标准, 3=DiT还原, 4=ICL2.0)
	modelType := 1 // default to ICL 1.0
	switch req.ModelType {
	case VoiceCloneModelStandard:
		modelType = 1
	case VoiceCloneModelPro:
		modelType = 3 // DiT 还原版
	}

	// Build JSON request body
	requestBody := map[string]any{
		"appid":        s.client.config.appID,
		"speaker_id":   req.SpeakerID,
		"audio_format": audioFormat,
		"model_type":   modelType,
	}

	// Optional fields
	if req.Language != "" {
		// Language: 0=zh, 1=en, 2=ja (guessing based on common patterns)
		lang := 0 // default zh
		if req.Language == LanguageEnUS || req.Language == LanguageEnGB {
			lang = 1
		} else if req.Language == LanguageJaJP {
			lang = 2
		}
		requestBody["language"] = lang
	}

	if req.Text != "" {
		requestBody["text"] = req.Text
	}

	// Add audio data as base64
	if len(req.AudioData) > 0 {
		requestBody["audio_data"] = base64.StdEncoding.EncodeToString(req.AudioData[0])
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, wrapError(err, "marshal request")
	}

	reqURL := s.client.config.baseURL + "/api/v1/mega_tts/audio/upload"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, wrapError(err, "create request")
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.client.setAuthHeaders(httpReq)

	resp, err := s.client.config.httpClient.Do(httpReq)
	if err != nil {
		return nil, wrapError(err, "send request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError(err, "read response")
	}

	logID := resp.Header.Get("X-Tt-Logid")

	if resp.StatusCode != http.StatusOK {
		if apiErr := parseAPIError(resp.StatusCode, respBody, logID); apiErr != nil {
			return nil, apiErr
		}
	}

	// Parse response - mega_tts uses BaseResp format
	var apiResp struct {
		BaseResp struct {
			StatusCode    int    `json:"StatusCode"`
			StatusMessage string `json:"StatusMessage"`
		} `json:"BaseResp"`
		SpeakerID string `json:"speaker_id"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, wrapError(err, "unmarshal response")
	}

	if apiResp.BaseResp.StatusCode != 0 {
		return nil, &Error{
			Code:    apiResp.BaseResp.StatusCode,
			Message: apiResp.BaseResp.StatusMessage,
			LogID:   logID,
		}
	}

	speakerID := apiResp.SpeakerID
	if speakerID == "" {
		speakerID = req.SpeakerID
	}

	return newTask[VoiceCloneResult]("", s.client, taskTypeVoiceClone, speakerID), nil
}

// GetStatus queries training status
//
// Returns the current status of a voice clone training task
func (s *VoiceCloneService) GetStatus(ctx context.Context, speakerID string) (*VoiceCloneStatus, error) {
	params := url.Values{}
	params.Set("appid", s.client.config.appID)
	params.Set("speaker_id", speakerID)

	reqURL := s.client.config.baseURL + "/api/v1/mega_tts/status?" + params.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, wrapError(err, "create request")
	}

	s.client.setAuthHeaders(httpReq)

	resp, err := s.client.config.httpClient.Do(httpReq)
	if err != nil {
		return nil, wrapError(err, "send request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError(err, "read response")
	}

	logID := resp.Header.Get("X-Tt-Logid")

	if resp.StatusCode != http.StatusOK {
		if apiErr := parseAPIError(resp.StatusCode, respBody, logID); apiErr != nil {
			return nil, apiErr
		}
	}

	var apiResp struct {
		BaseResp struct {
			StatusCode    int    `json:"StatusCode"`
			StatusMessage string `json:"StatusMessage"`
		} `json:"BaseResp"`
		SpeakerID string `json:"speaker_id"`
		Status    string `json:"status"`
		DemoAudio string `json:"demo_audio,omitempty"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, wrapError(err, "unmarshal response")
	}

	if apiResp.BaseResp.StatusCode != 0 {
		return nil, &Error{
			Code:    apiResp.BaseResp.StatusCode,
			Message: apiResp.BaseResp.StatusMessage,
			LogID:   logID,
		}
	}

	// Convert status
	var status VoiceCloneStatusType
	switch apiResp.Status {
	case "Processing":
		status = VoiceCloneStatusProcessing
	case "Success":
		status = VoiceCloneStatusSuccess
	case "Failed":
		status = VoiceCloneStatusFailed
	default:
		status = VoiceCloneStatusPending
	}

	return &VoiceCloneStatus{
		SpeakerID: apiResp.SpeakerID,
		Status:    status,
		DemoAudio: apiResp.DemoAudio,
	}, nil
}

// detectAudioFormat detects audio format from file header
func detectAudioFormat(data []byte) string {
	if len(data) < 12 {
		return "wav"
	}

	// Check for WAV (RIFF header)
	if string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return "wav"
	}

	// Check for MP3 (ID3 or sync word)
	if string(data[0:3]) == "ID3" || (data[0] == 0xFF && (data[1]&0xE0) == 0xE0) {
		return "mp3"
	}

	// Check for OGG (OggS)
	if string(data[0:4]) == "OggS" {
		return "ogg"
	}

	return "wav" // default
}
