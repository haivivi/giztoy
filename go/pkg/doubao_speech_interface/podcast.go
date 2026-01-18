package doubao_speech_interface

import (
	"context"
)

// PodcastService 播客场景服务接口
type PodcastService interface {
	// CreateTask 创建播客合成任务
	CreateTask(ctx context.Context, req *PodcastTaskRequest) (*Task[PodcastResult], error)

	// GetTask 查询任务状态
	GetTask(ctx context.Context, taskID string) (*PodcastTaskStatus, error)
}

// PodcastTaskRequest 播客合成请求
type PodcastTaskRequest struct {
	// Script 播客脚本
	Script []PodcastLine `json:"script"`

	// Encoding 输出音频格式
	Encoding AudioEncoding `json:"encoding,omitempty"`

	// SampleRate 采样率
	SampleRate SampleRate `json:"sample_rate,omitempty"`

	// CallbackURL 回调地址
	CallbackURL string `json:"callback_url,omitempty"`
}

// PodcastLine 播客台词
type PodcastLine struct {
	// SpeakerID 说话人音色 ID
	SpeakerID string `json:"speaker_id"`

	// Text 台词文本
	Text string `json:"text"`

	// Emotion 情感（可选）
	Emotion string `json:"emotion,omitempty"`

	// SpeedRatio 语速（可选）
	SpeedRatio float64 `json:"speed_ratio,omitempty"`
}

// PodcastResult 播客合成结果
type PodcastResult struct {
	// AudioURL 音频下载地址
	AudioURL string `json:"audio_url"`

	// Duration 音频时长（毫秒）
	Duration int `json:"duration"`

	// Subtitles 字幕列表
	Subtitles []SubtitleSegment `json:"subtitles,omitempty"`
}

// PodcastTaskStatus 任务状态
type PodcastTaskStatus struct {
	// TaskID 任务 ID
	TaskID string `json:"task_id"`

	// Status 状态
	Status TaskStatus `json:"status"`

	// Progress 进度（0-100）
	Progress int `json:"progress,omitempty"`

	// Result 结果
	Result *PodcastResult `json:"result,omitempty"`

	// Error 错误信息
	Error *Error `json:"error,omitempty"`
}
