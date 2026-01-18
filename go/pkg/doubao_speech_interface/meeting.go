package doubao_speech_interface

import (
	"context"
)

// MeetingService 会议场景服务接口
type MeetingService interface {
	// CreateTask 创建会议转写任务
	CreateTask(ctx context.Context, req *MeetingTaskRequest) (*Task[MeetingResult], error)

	// GetTask 查询任务状态
	GetTask(ctx context.Context, taskID string) (*MeetingTaskStatus, error)
}

// MeetingTaskRequest 会议转写请求
type MeetingTaskRequest struct {
	// AudioURL 音频文件 URL
	AudioURL string `json:"audio_url"`

	// Format 音频格式
	Format AudioFormat `json:"format,omitempty"`

	// Language 语言
	Language Language `json:"language,omitempty"`

	// SpeakerCount 说话人数量（用于说话人分离）
	SpeakerCount int `json:"speaker_count,omitempty"`

	// EnableSpeakerDiarization 启用说话人分离
	EnableSpeakerDiarization bool `json:"enable_speaker_diarization,omitempty"`

	// EnableTimestamp 启用时间戳
	EnableTimestamp bool `json:"enable_timestamp,omitempty"`

	// CallbackURL 回调地址
	CallbackURL string `json:"callback_url,omitempty"`
}

// MeetingResult 会议转写结果
type MeetingResult struct {
	// Text 完整转写文本
	Text string `json:"text"`

	// Duration 音频时长（毫秒）
	Duration int `json:"duration"`

	// Segments 分段详情
	Segments []MeetingSegment `json:"segments,omitempty"`
}

// MeetingSegment 会议分段
type MeetingSegment struct {
	// Text 分段文本
	Text string `json:"text"`

	// StartTime 开始时间（毫秒）
	StartTime int `json:"start_time"`

	// EndTime 结束时间（毫秒）
	EndTime int `json:"end_time"`

	// SpeakerID 说话人 ID
	SpeakerID string `json:"speaker_id,omitempty"`
}

// MeetingTaskStatus 任务状态
type MeetingTaskStatus struct {
	// TaskID 任务 ID
	TaskID string `json:"task_id"`

	// Status 状态
	Status TaskStatus `json:"status"`

	// Progress 进度（0-100）
	Progress int `json:"progress,omitempty"`

	// Result 结果（完成时有值）
	Result *MeetingResult `json:"result,omitempty"`

	// Error 错误信息（失败时有值）
	Error *Error `json:"error,omitempty"`
}
