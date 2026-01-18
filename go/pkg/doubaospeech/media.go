package doubaospeech

import (
	"context"
	"net/http"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

// mediaService 音视频处理服务实现
type mediaService struct {
	client *Client
}

// newMediaService 创建音视频处理服务
func newMediaService(c *Client) iface.MediaService {
	return &mediaService{client: c}
}

// ExtractSubtitle 提取字幕
func (s *mediaService) ExtractSubtitle(ctx context.Context, req *iface.SubtitleRequest) (*iface.Task[iface.SubtitleResult], error) {
	submitReq := map[string]interface{}{
		"appid":     s.client.config.appID,
		"reqid":     generateReqID(),
		"media_url": req.MediaURL,
	}

	if req.Language != "" {
		submitReq["language"] = string(req.Language)
	}
	if req.Format != "" {
		submitReq["output_format"] = string(req.Format)
	}
	if req.EnableTranslation {
		submitReq["enable_translation"] = true
		if req.TargetLanguage != "" {
			submitReq["target_language"] = string(req.TargetLanguage)
		}
	}
	if req.CallbackURL != "" {
		submitReq["callback_url"] = req.CallbackURL
	}

	var resp asyncTaskResponse
	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/subtitle/submit", submitReq, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, &Error{
			Code:    resp.Code,
			Message: resp.Message,
			ReqID:   resp.ReqID,
		}
	}

	return newTask[iface.SubtitleResult](resp.TaskID, s.client, taskTypeSubtitle, submitReq["reqid"].(string)), nil
}

// GetSubtitleTask 查询字幕任务状态
func (s *mediaService) GetSubtitleTask(ctx context.Context, taskID string) (*iface.SubtitleTaskStatus, error) {
	queryReq := map[string]interface{}{
		"appid":   s.client.config.appID,
		"task_id": taskID,
	}

	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID          string `json:"task_id"`
			Status          string `json:"status"`
			Progress        int    `json:"progress,omitempty"`
			SubtitleURL     string `json:"subtitle_url,omitempty"`
			SubtitleContent string `json:"subtitle_content,omitempty"`
			Duration        int    `json:"duration,omitempty"`
		} `json:"data"`
	}

	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/subtitle/query", queryReq, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
		}
	}

	status := &iface.SubtitleTaskStatus{
		TaskID:   apiResp.Data.TaskID,
		Progress: apiResp.Data.Progress,
	}

	// 转换状态
	switch apiResp.Data.Status {
	case "submitted", "pending":
		status.Status = iface.TaskStatusPending
	case "running", "processing":
		status.Status = iface.TaskStatusProcessing
	case "success":
		status.Status = iface.TaskStatusSuccess
		status.Result = &iface.SubtitleResult{
			SubtitleURL: apiResp.Data.SubtitleURL,
			Duration:    apiResp.Data.Duration,
		}
	case "failed":
		status.Status = iface.TaskStatusFailed
	default:
		status.Status = iface.TaskStatusPending
	}

	return status, nil
}

// 注册实现验证
var _ iface.MediaService = (*mediaService)(nil)
