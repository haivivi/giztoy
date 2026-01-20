package doubaospeech

import (
	"context"
	"net/http"
)

// MediaService represents media processing service
// mediaService provides media operations
// MediaService provides media processing functionality (subtitle extraction)
type MediaService struct {
	client *Client
}

// newMediaService creates media service
func newMediaService(c *Client) *MediaService {
	return &MediaService{client: c}
}

// ExtractSubtitle extracts subtitles from media
func (s *MediaService) ExtractSubtitle(ctx context.Context, req *SubtitleRequest) (*Task[SubtitleResult], error) {
	submitReq := map[string]any{
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

	return newTask[SubtitleResult](resp.TaskID, s.client, taskTypeSubtitle, submitReq["reqid"].(string)), nil
}

// GetSubtitleTask queries subtitle task status
func (s *MediaService) GetSubtitleTask(ctx context.Context, taskID string) (*SubtitleTaskStatus, error) {
	queryReq := map[string]any{
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

	status := &SubtitleTaskStatus{
		TaskID:   apiResp.Data.TaskID,
		Progress: apiResp.Data.Progress,
	}

	// Convert status
	switch apiResp.Data.Status {
	case "submitted", "pending":
		status.Status = TaskStatusPending
	case "running", "processing":
		status.Status = TaskStatusProcessing
	case "success":
		status.Status = TaskStatusSuccess
		status.Result = &SubtitleResult{
			SubtitleURL: apiResp.Data.SubtitleURL,
			Duration:    apiResp.Data.Duration,
		}
	case "failed":
		status.Status = TaskStatusFailed
	default:
		status.Status = TaskStatusPending
	}

	return status, nil
}
