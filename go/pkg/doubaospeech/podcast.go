package doubaospeech

import (
	"context"
	"net/http"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

// podcastService 播客场景服务实现
type podcastService struct {
	client *Client
}

// newPodcastService 创建播客服务
func newPodcastService(c *Client) iface.PodcastService {
	return &podcastService{client: c}
}

// CreateTask 创建播客合成任务
func (s *podcastService) CreateTask(ctx context.Context, req *iface.PodcastTaskRequest) (*iface.Task[iface.PodcastResult], error) {
	// 构建对话列表
	dialogues := make([]map[string]interface{}, len(req.Script))
	for i, line := range req.Script {
		d := map[string]interface{}{
			"speaker": line.SpeakerID,
			"text":    line.Text,
		}
		if line.Emotion != "" {
			d["emotion"] = line.Emotion
		}
		if line.SpeedRatio != 0 {
			d["speed_ratio"] = line.SpeedRatio
		}
		dialogues[i] = d
	}

	submitReq := map[string]interface{}{
		"app": map[string]interface{}{
			"appid":   s.client.config.appID,
			"cluster": s.client.config.cluster,
		},
		"user": map[string]interface{}{
			"uid": s.client.config.userID,
		},
		"request": map[string]interface{}{
			"reqid":     generateReqID(),
			"dialogues": dialogues,
		},
	}

	if req.Encoding != "" {
		submitReq["audio"] = map[string]interface{}{
			"encoding": string(req.Encoding),
		}
		if req.SampleRate != 0 {
			submitReq["audio"].(map[string]interface{})["sample_rate"] = int(req.SampleRate)
		}
	}

	if req.CallbackURL != "" {
		submitReq["request"].(map[string]interface{})["callback_url"] = req.CallbackURL
	}

	var resp asyncTaskResponse
	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/podcast/submit", submitReq, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, &Error{
			Code:    resp.Code,
			Message: resp.Message,
			ReqID:   resp.ReqID,
		}
	}

	return newTask[iface.PodcastResult](resp.TaskID, s.client, taskTypePodcast, submitReq["request"].(map[string]interface{})["reqid"].(string)), nil
}

// GetTask 查询任务状态
func (s *podcastService) GetTask(ctx context.Context, taskID string) (*iface.PodcastTaskStatus, error) {
	queryReq := map[string]interface{}{
		"appid":   s.client.config.appID,
		"task_id": taskID,
	}

	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID   string `json:"task_id"`
			Status   string `json:"status"`
			Progress int    `json:"progress,omitempty"`
			AudioURL string `json:"audio_url,omitempty"`
			Duration int    `json:"duration,omitempty"`
		} `json:"data"`
	}

	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/podcast/query", queryReq, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
		}
	}

	status := &iface.PodcastTaskStatus{
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
		status.Result = &iface.PodcastResult{
			AudioURL: apiResp.Data.AudioURL,
			Duration: apiResp.Data.Duration,
		}
	case "failed":
		status.Status = iface.TaskStatusFailed
	default:
		status.Status = iface.TaskStatusPending
	}

	return status, nil
}

// 注册实现验证
var _ iface.PodcastService = (*podcastService)(nil)
