package doubaospeech

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// taskType represents task type
type taskType string

const (
	taskTypeTTSAsync   taskType = "tts_async"
	taskTypeASRFile    taskType = "asr_file"
	taskTypeVoiceClone taskType = "voice_clone"
	taskTypeMeeting    taskType = "meeting"
	taskTypePodcast    taskType = "podcast"
	taskTypeSubtitle   taskType = "subtitle"
)

// newTask creates async task
func newTask[T any](id string, client *Client, tt taskType, reqID string) *Task[T] {
	// Note: The actual polling logic is implemented via WaitTask function
	return &Task[T]{
		ID: id,
	}
}

// queryTaskStatus queries task status
func (c *Client) queryTaskStatus(ctx context.Context, taskType taskType, reqID string) (*taskStatusResult, error) {
	var path string
	switch taskType {
	case taskTypeTTSAsync:
		path = "/api/v1/tts_async/query"
	case taskTypeASRFile:
		path = "/api/v1/asr/query"
	case taskTypeVoiceClone:
		path = "/api/v1/voice_clone/query"
	case taskTypeMeeting:
		path = "/api/v1/meeting/query"
	case taskTypePodcast:
		path = "/api/v1/podcast/query"
	case taskTypeSubtitle:
		path = "/api/v1/subtitle/query"
	default:
		return nil, newAPIError(0, "unknown task type")
	}

	queryReq := map[string]any{
		"appid": c.config.appID,
		"reqid": reqID,
	}

	var resp taskStatusResult
	if err := c.doJSONRequest(ctx, http.MethodPost, path, queryReq, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// taskStatusResult represents task status result
type taskStatusResult struct {
	ReqID         string          `json:"reqid"`
	TaskID        string          `json:"task_id"`
	Status        string          `json:"status"`
	Progress      int             `json:"progress,omitempty"`
	Code          int             `json:"code"`
	Message       string          `json:"message"`
	AudioURL      string          `json:"audio_url,omitempty"`
	AudioDuration int             `json:"audio_duration,omitempty"`
	AudioSize     int64           `json:"audio_size,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
}

// toTaskStatus converts to TaskStatus
func (r *taskStatusResult) toTaskStatus() TaskStatus {
	switch r.Status {
	case "submitted", "pending":
		return TaskStatusPending
	case "running", "processing":
		return TaskStatusProcessing
	case "success":
		return TaskStatusSuccess
	case "failed":
		return TaskStatusFailed
	case "cancelled":
		return TaskStatusCancelled
	default:
		return TaskStatusPending
	}
}

// WaitTask waits for task completion
func WaitTask[T any](ctx context.Context, client *Client, taskType taskType, reqID string, interval time.Duration) (*T, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			result, err := client.queryTaskStatus(ctx, taskType, reqID)
			if err != nil {
				return nil, err
			}

			status := result.toTaskStatus()
			switch status {
			case TaskStatusSuccess:
				return parseTaskResult[T](result)
			case TaskStatusFailed:
				return nil, &Error{
					Code:    result.Code,
					Message: result.Message,
					ReqID:   result.ReqID,
				}
			case TaskStatusCancelled:
				return nil, newAPIError(0, "task cancelled")
			}
			// Continue waiting
		}
	}
}

// parseTaskResult parses task result
func parseTaskResult[T any](result *taskStatusResult) (*T, error) {
	var r T

	// Handle based on type
	switch any(&r).(type) {
	case *TTSAsyncResult:
		ttsResult := &TTSAsyncResult{
			AudioURL: result.AudioURL,
			Duration: result.AudioDuration,
		}
		return any(ttsResult).(*T), nil

	case *ASRResult:
		var asrResult ASRResult
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &asrResult); err != nil {
				return nil, wrapError(err, "unmarshal asr result")
			}
		}
		return any(&asrResult).(*T), nil

	case *VoiceCloneResult:
		vcResult := &VoiceCloneResult{
			SpeakerID: result.TaskID,
			Status:    VoiceCloneStatusSuccess,
		}
		return any(vcResult).(*T), nil

	case *MeetingResult:
		var meetingResult MeetingResult
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &meetingResult); err != nil {
				return nil, wrapError(err, "unmarshal meeting result")
			}
		}
		return any(&meetingResult).(*T), nil

	case *PodcastResult:
		podcastResult := &PodcastResult{
			AudioURL: result.AudioURL,
			Duration: result.AudioDuration,
		}
		return any(podcastResult).(*T), nil

	case *SubtitleResult:
		var subtitleResult SubtitleResult
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &subtitleResult); err != nil {
				return nil, wrapError(err, "unmarshal subtitle result")
			}
		}
		subtitleResult.Duration = result.AudioDuration
		return any(&subtitleResult).(*T), nil

	default:
		// Generic parsing
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &r); err != nil {
				return nil, wrapError(err, "unmarshal result")
			}
		}
		return &r, nil
	}
}
