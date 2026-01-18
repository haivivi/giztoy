package doubaospeech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

// voiceCloneService 声音复刻服务实现
type voiceCloneService struct {
	client *Client
}

// newVoiceCloneService 创建声音复刻服务
func newVoiceCloneService(c *Client) iface.VoiceCloneService {
	return &voiceCloneService{client: c}
}

// Train 训练自定义音色
func (s *voiceCloneService) Train(ctx context.Context, req *iface.VoiceCloneTrainRequest) (*iface.Task[iface.VoiceCloneResult], error) {
	// 构建 multipart 请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加基础字段
	writer.WriteField("appid", s.client.config.appID)
	writer.WriteField("voice_name", req.SpeakerID)
	writer.WriteField("language", string(req.Language))

	if req.ModelType != "" {
		writer.WriteField("clone_type", string(req.ModelType))
	}
	if req.Text != "" {
		writer.WriteField("text", req.Text)
	}

	// 添加音频文件（如果有）
	for i, audioData := range req.AudioData {
		part, err := writer.CreateFormFile("audio_file", fmt.Sprintf("audio_%d.wav", i))
		if err != nil {
			return nil, wrapError(err, "create form file")
		}
		if _, err := part.Write(audioData); err != nil {
			return nil, wrapError(err, "write audio data")
		}
	}

	if err := writer.Close(); err != nil {
		return nil, wrapError(err, "close writer")
	}

	url := s.client.config.baseURL + "/api/v1/voice_clone/submit"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, wrapError(err, "create request")
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
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

	// 解析响应
	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID  string `json:"task_id"`
			VoiceID string `json:"voice_id"`
			Status  string `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, wrapError(err, "unmarshal response")
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
			LogID:   logID,
		}
	}

	return newTask[iface.VoiceCloneResult](apiResp.Data.TaskID, s.client, taskTypeVoiceClone, apiResp.Data.VoiceID), nil
}

// GetStatus 查询训练状态
func (s *voiceCloneService) GetStatus(ctx context.Context, speakerID string) (*iface.VoiceCloneStatus, error) {
	queryReq := map[string]interface{}{
		"appid":    s.client.config.appID,
		"voice_id": speakerID,
	}

	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID    string `json:"task_id"`
			VoiceID   string `json:"voice_id"`
			Status    string `json:"status"`
			VoiceName string `json:"voice_name"`
			CreatedAt string `json:"created_at"`
		} `json:"data"`
	}

	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/voice_clone/query", queryReq, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
		}
	}

	// 转换状态
	var status iface.VoiceCloneStatusType
	switch apiResp.Data.Status {
	case "processing":
		status = iface.VoiceCloneStatusProcessing
	case "success":
		status = iface.VoiceCloneStatusSuccess
	case "failed":
		status = iface.VoiceCloneStatusFailed
	default:
		status = iface.VoiceCloneStatusPending
	}

	return &iface.VoiceCloneStatus{
		SpeakerID: apiResp.Data.VoiceID,
		Status:    status,
	}, nil
}

// List 列出已训练的音色
func (s *voiceCloneService) List(ctx context.Context) ([]*iface.VoiceCloneInfo, error) {
	url := s.client.config.baseURL + "/api/v1/voice_clone/list?appid=" + s.client.config.appID
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Voices []struct {
				VoiceID   string `json:"voice_id"`
				VoiceName string `json:"voice_name"`
				Status    string `json:"status"`
				Language  string `json:"language"`
				CloneType string `json:"clone_type"`
				CreatedAt int64  `json:"created_at"`
			} `json:"voices"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, wrapError(err, "unmarshal response")
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
			LogID:   logID,
		}
	}

	var result []*iface.VoiceCloneInfo
	for _, v := range apiResp.Data.Voices {
		var status iface.VoiceCloneStatusType
		switch v.Status {
		case "processing":
			status = iface.VoiceCloneStatusProcessing
		case "success":
			status = iface.VoiceCloneStatusSuccess
		case "failed":
			status = iface.VoiceCloneStatusFailed
		default:
			status = iface.VoiceCloneStatusPending
		}

		var modelType iface.VoiceCloneModelType
		switch v.CloneType {
		case "standard":
			modelType = iface.VoiceCloneModelStandard
		case "pro", "professional":
			modelType = iface.VoiceCloneModelPro
		}

		result = append(result, &iface.VoiceCloneInfo{
			SpeakerID: v.VoiceID,
			Status:    status,
			Language:  iface.Language(v.Language),
			ModelType: modelType,
			CreatedAt: v.CreatedAt,
		})
	}

	return result, nil
}

// Delete 删除已训练的音色
func (s *voiceCloneService) Delete(ctx context.Context, speakerID string) error {
	deleteReq := map[string]interface{}{
		"appid":    s.client.config.appID,
		"voice_id": speakerID,
	}

	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/voice_clone/delete", deleteReq, &apiResp); err != nil {
		return err
	}

	if apiResp.Code != 0 {
		return &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
		}
	}

	return nil
}

// 注册实现验证
var _ iface.VoiceCloneService = (*voiceCloneService)(nil)
