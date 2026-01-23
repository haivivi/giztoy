package doubaospeech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// ================== 请求结构体 ==================

// appInfo 应用信息
type appInfo struct {
	AppID   string `json:"appid"`
	Token   string `json:"token,omitempty"`
	Cluster string `json:"cluster"`
}

// userInfo 用户信息
type userInfo struct {
	UID string `json:"uid"`
}

// ttsAudioParams TTS 音频参数
type ttsAudioParams struct {
	VoiceType   string  `json:"voice_type"`
	Encoding    string  `json:"encoding,omitempty"`
	SpeedRatio  float64 `json:"speed_ratio,omitempty"`
	VolumeRatio float64 `json:"volume_ratio,omitempty"`
	PitchRatio  float64 `json:"pitch_ratio,omitempty"`
	Emotion     string  `json:"emotion,omitempty"`
	Language    string  `json:"language,omitempty"`
}

// ttsRequestParams TTS 请求参数
type ttsRequestParams struct {
	ReqID           string `json:"reqid"`
	Text            string `json:"text"`
	TextType        string `json:"text_type,omitempty"`
	Operation       string `json:"operation,omitempty"`
	SilenceDuration int    `json:"silence_duration,omitempty"`
	WithFrontend    bool   `json:"with_frontend,omitempty"`
	FrontendType    string `json:"frontend_type,omitempty"`
}

// ttsRequest TTS 请求体
type ttsRequest struct {
	App     appInfo          `json:"app"`
	User    userInfo         `json:"user"`
	Audio   ttsAudioParams   `json:"audio"`
	Request ttsRequestParams `json:"request"`
}

// asrAudioParams ASR 音频参数
type asrAudioParams struct {
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Channel    int    `json:"channel,omitempty"`
	Bits       int    `json:"bits,omitempty"`
	URL        string `json:"url,omitempty"`
	Data       string `json:"data,omitempty"`
}

// asrRequestParams ASR 请求参数
type asrRequestParams struct {
	ReqID          string `json:"reqid"`
	Language       string `json:"language,omitempty"`
	EnableITN      bool   `json:"enable_itn,omitempty"`
	EnablePunc     bool   `json:"enable_punc,omitempty"`
	EnableDDC      bool   `json:"enable_ddc,omitempty"`
	ShowUtterances bool   `json:"show_utterances,omitempty"`
	ResultType     string `json:"result_type,omitempty"`
	Workflow       string `json:"workflow,omitempty"`
	Command        string `json:"command,omitempty"`
}

// asrRequest ASR 请求体
type asrRequest struct {
	App     appInfo          `json:"app"`
	User    userInfo         `json:"user"`
	Audio   asrAudioParams   `json:"audio"`
	Request asrRequestParams `json:"request"`
}

// asyncTTSSubmitRequest 异步 TTS 提交请求
type asyncTTSSubmitRequest struct {
	AppID       string  `json:"appid"`
	ReqID       string  `json:"reqid"`
	Text        string  `json:"text"`
	VoiceType   string  `json:"voice_type"`
	Format      string  `json:"format,omitempty"`
	SampleRate  int     `json:"sample_rate,omitempty"`
	SpeedRatio  float64 `json:"speed_ratio,omitempty"`
	VolumeRatio float64 `json:"volume_ratio,omitempty"`
	PitchRatio  float64 `json:"pitch_ratio,omitempty"`
	CallbackURL string  `json:"callback_url,omitempty"`
}

// asyncTTSQueryRequest 异步 TTS 查询请求
type asyncTTSQueryRequest struct {
	AppID string `json:"appid"`
	ReqID string `json:"reqid"`
}

// asyncASRSubmitRequest 异步 ASR 提交请求
type asyncASRSubmitRequest struct {
	AppID          string `json:"appid"`
	ReqID          string `json:"reqid"`
	AudioURL       string `json:"audio_url"`
	Language       string `json:"language,omitempty"`
	EnableITN      bool   `json:"enable_itn,omitempty"`
	EnablePunc     bool   `json:"enable_punc,omitempty"`
	EnableSpeaker  bool   `json:"enable_speaker,omitempty"`
	SpeakerCount   int    `json:"speaker_count,omitempty"`
	CallbackURL    string `json:"callback_url,omitempty"`
}

// ================== HTTP 请求辅助函数 ==================

// doJSONRequest 发送 JSON 请求
func (c *Client) doJSONRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return wrapError(err, "marshal request body")
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	url := c.config.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return wrapError(err, "create request")
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		return wrapError(err, "send request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return wrapError(err, "read response")
	}

	logID := resp.Header.Get("X-Tt-Logid")

	if resp.StatusCode != http.StatusOK {
		if apiErr := parseAPIError(resp.StatusCode, respBody, logID); apiErr != nil {
			return apiErr
		}
		return &Error{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			LogID:      logID,
		}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return wrapError(err, "unmarshal response")
		}
	}

	return nil
}

// generateReqID 生成请求 ID
func generateReqID() string {
	return uuid.New().String()
}

// buildTTSRequest 构建 TTS 请求
func (c *Client) buildTTSRequest(text, voiceType string) *ttsRequest {
	return &ttsRequest{
		App: appInfo{
			AppID:   c.config.appID,
			Token:   c.config.accessToken, // Required in request body
			Cluster: c.config.cluster,
		},
		User: userInfo{
			UID: c.config.userID,
		},
		Audio: ttsAudioParams{
			VoiceType: voiceType,
		},
		Request: ttsRequestParams{
			ReqID:     generateReqID(),
			Text:      text,
			TextType:  "plain",
			Operation: "query",
		},
	}
}

// buildASRRequest 构建 ASR 请求
func (c *Client) buildASRRequest(format string) *asrRequest {
	return &asrRequest{
		App: appInfo{
			AppID:   c.config.appID,
			Token:   c.config.accessToken, // Required in request body
			Cluster: c.config.cluster,
		},
		User: userInfo{
			UID: c.config.userID,
		},
		Audio: asrAudioParams{
			Format: format,
		},
		Request: asrRequestParams{
			ReqID: generateReqID(),
		},
	}
}
