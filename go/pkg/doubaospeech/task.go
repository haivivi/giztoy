package doubaospeech

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

// taskType 任务类型
type taskType string

const (
	taskTypeTTSAsync   taskType = "tts_async"
	taskTypeASRFile    taskType = "asr_file"
	taskTypeVoiceClone taskType = "voice_clone"
	taskTypeMeeting    taskType = "meeting"
	taskTypePodcast    taskType = "podcast"
	taskTypeSubtitle   taskType = "subtitle"
)

// task 异步任务实现
type task[T any] struct {
	id       string
	client   *Client
	taskType taskType
	reqID    string
}

// newTask 创建异步任务
func newTask[T any](id string, client *Client, tt taskType, reqID string) *iface.Task[T] {
	// 创建一个 iface.Task 包装
	// 注意：实际的轮询逻辑需要在调用方通过 WaitTask 函数实现
	_ = &task[T]{
		id:       id,
		client:   client,
		taskType: tt,
		reqID:    reqID,
	}
	
	return &iface.Task[T]{
		ID: id,
	}
}

// queryTaskStatus 查询任务状态
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

	queryReq := map[string]interface{}{
		"appid": c.config.appID,
		"reqid": reqID,
	}

	var resp taskStatusResult
	if err := c.doJSONRequest(ctx, http.MethodPost, path, queryReq, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// taskStatusResult 任务状态结果
type taskStatusResult struct {
	ReqID         string `json:"reqid"`
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	Progress      int    `json:"progress,omitempty"`
	Code          int    `json:"code"`
	Message       string `json:"message"`
	AudioURL      string `json:"audio_url,omitempty"`
	AudioDuration int    `json:"audio_duration,omitempty"`
	AudioSize     int64  `json:"audio_size,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
}

// toTaskStatus 转换为 iface.TaskStatus
func (r *taskStatusResult) toTaskStatus() iface.TaskStatus {
	switch r.Status {
	case "submitted", "pending":
		return iface.TaskStatusPending
	case "running", "processing":
		return iface.TaskStatusProcessing
	case "success":
		return iface.TaskStatusSuccess
	case "failed":
		return iface.TaskStatusFailed
	case "cancelled":
		return iface.TaskStatusCancelled
	default:
		return iface.TaskStatusPending
	}
}

// WaitTask 等待任务完成的通用函数
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
			case iface.TaskStatusSuccess:
				return parseTaskResult[T](result)
			case iface.TaskStatusFailed:
				return nil, &Error{
					Code:    result.Code,
					Message: result.Message,
					ReqID:   result.ReqID,
				}
			case iface.TaskStatusCancelled:
				return nil, newAPIError(0, "task cancelled")
			}
			// 继续等待
		}
	}
}

// parseTaskResult 解析任务结果
func parseTaskResult[T any](result *taskStatusResult) (*T, error) {
	var r T

	// 根据类型处理
	switch any(&r).(type) {
	case *iface.TTSAsyncResult:
		ttsResult := &iface.TTSAsyncResult{
			AudioURL: result.AudioURL,
			Duration: result.AudioDuration,
		}
		return any(ttsResult).(*T), nil

	case *iface.ASRResult:
		var asrResult iface.ASRResult
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &asrResult); err != nil {
				return nil, wrapError(err, "unmarshal asr result")
			}
		}
		return any(&asrResult).(*T), nil

	case *iface.VoiceCloneResult:
		vcResult := &iface.VoiceCloneResult{
			SpeakerID: result.TaskID,
			Status:    iface.VoiceCloneStatusSuccess,
		}
		return any(vcResult).(*T), nil

	case *iface.MeetingResult:
		var meetingResult iface.MeetingResult
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &meetingResult); err != nil {
				return nil, wrapError(err, "unmarshal meeting result")
			}
		}
		return any(&meetingResult).(*T), nil

	case *iface.PodcastResult:
		podcastResult := &iface.PodcastResult{
			AudioURL: result.AudioURL,
			Duration: result.AudioDuration,
		}
		return any(podcastResult).(*T), nil

	case *iface.SubtitleResult:
		var subtitleResult iface.SubtitleResult
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &subtitleResult); err != nil {
				return nil, wrapError(err, "unmarshal subtitle result")
			}
		}
		subtitleResult.Duration = result.AudioDuration
		return any(&subtitleResult).(*T), nil

	default:
		// 通用解析
		if result.Result != nil {
			if err := json.Unmarshal(result.Result, &r); err != nil {
				return nil, wrapError(err, "unmarshal result")
			}
		}
		return &r, nil
	}
}
