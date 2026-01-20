package doubaospeech

import (
	"context"
	"net/http"
)

// MeetingService represents meeting transcription service
// meetingService provides meeting operations
// MeetingService provides meeting transcription functionality
type MeetingService struct {
	client *Client
}

// newMeetingService creates meeting service
func newMeetingService(c *Client) *MeetingService {
	return &MeetingService{client: c}
}

// CreateTask creates meeting transcription task
func (s *MeetingService) CreateTask(ctx context.Context, req *MeetingTaskRequest) (*Task[MeetingResult], error) {
	submitReq := map[string]any{
		"appid":     s.client.config.appID,
		"reqid":     generateReqID(),
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

	return newTask[MeetingResult](resp.TaskID, s.client, taskTypeMeeting, submitReq["reqid"].(string)), nil
}

// GetTask queries task status
func (s *MeetingService) GetTask(ctx context.Context, taskID string) (*MeetingTaskStatus, error) {
	queryReq := map[string]any{
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

	status := &MeetingTaskStatus{
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
	case "failed":
		status.Status = TaskStatusFailed
	default:
		status.Status = TaskStatusPending
	}

	// Convert result
	if apiResp.Data.Result != nil {
		result := &MeetingResult{
			Text:     apiResp.Data.Result.Text,
			Duration: apiResp.Data.Result.Duration,
		}
		for _, seg := range apiResp.Data.Result.Segments {
			result.Segments = append(result.Segments, MeetingSegment{
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
