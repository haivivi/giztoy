package doubaospeech

import (
	"context"
	"net/http"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

// meetingService 会议场景服务实现
type meetingService struct {
	client *Client
}

// newMeetingService 创建会议服务
func newMeetingService(c *Client) iface.MeetingService {
	return &meetingService{client: c}
}

// CreateTask 创建会议转写任务
func (s *meetingService) CreateTask(ctx context.Context, req *iface.MeetingTaskRequest) (*iface.Task[iface.MeetingResult], error) {
	submitReq := map[string]interface{}{
		"appid":    s.client.config.appID,
		"reqid":    generateReqID(),
		"audio_url": req.AudioURL,
	}

	if req.Format != "" {
		submitReq["format"] = string(req.Format)
	}
	if req.Language != "" {
		submitReq["language"] = string(req.Language)
	}
	if req.SpeakerCount > 0 {
		submitReq["speaker_count"] = req.SpeakerCount
	}
	if req.EnableSpeakerDiarization {
		submitReq["enable_speaker_diarization"] = true
	}
	if req.EnableTimestamp {
		submitReq["enable_timestamp"] = true
	}
	if req.CallbackURL != "" {
		submitReq["callback_url"] = req.CallbackURL
	}

	var resp asyncTaskResponse
	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/meeting/create", submitReq, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, &Error{
			Code:    resp.Code,
			Message: resp.Message,
			ReqID:   resp.ReqID,
		}
	}

	return newTask[iface.MeetingResult](resp.TaskID, s.client, taskTypeMeeting, submitReq["reqid"].(string)), nil
}

// GetTask 查询任务状态
func (s *meetingService) GetTask(ctx context.Context, taskID string) (*iface.MeetingTaskStatus, error) {
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
			Result   *struct {
				Text     string `json:"text"`
				Duration int    `json:"duration"`
				Segments []struct {
					Text      string `json:"text"`
					StartTime int    `json:"start_time"`
					EndTime   int    `json:"end_time"`
					SpeakerID string `json:"speaker_id,omitempty"`
				} `json:"segments,omitempty"`
			} `json:"result,omitempty"`
		} `json:"data"`
	}

	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/meeting/query", queryReq, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
		}
	}

	status := &iface.MeetingTaskStatus{
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
	case "failed":
		status.Status = iface.TaskStatusFailed
	default:
		status.Status = iface.TaskStatusPending
	}

	// 转换结果
	if apiResp.Data.Result != nil {
		result := &iface.MeetingResult{
			Text:     apiResp.Data.Result.Text,
			Duration: apiResp.Data.Result.Duration,
		}
		for _, seg := range apiResp.Data.Result.Segments {
			result.Segments = append(result.Segments, iface.MeetingSegment{
				Text:      seg.Text,
				StartTime: seg.StartTime,
				EndTime:   seg.EndTime,
				SpeakerID: seg.SpeakerID,
			})
		}
		status.Result = result
	}

	return status, nil
}

// 注册实现验证
var _ iface.MeetingService = (*meetingService)(nil)
