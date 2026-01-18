package doubao_speech_interface

import (
	"context"
)

// MediaService 音视频处理服务接口
type MediaService interface {
	// ExtractSubtitle 提取字幕
	ExtractSubtitle(ctx context.Context, req *SubtitleRequest) (*Task[SubtitleResult], error)

	// GetSubtitleTask 查询字幕任务状态
	GetSubtitleTask(ctx context.Context, taskID string) (*SubtitleTaskStatus, error)
}

// SubtitleFormat 字幕格式
type SubtitleFormat string

const (
	SubtitleFormatSRT  SubtitleFormat = "srt"
	SubtitleFormatVTT  SubtitleFormat = "vtt"
	SubtitleFormatJSON SubtitleFormat = "json"
)

// SubtitleRequest 字幕提取请求
type SubtitleRequest struct {
	// MediaURL 音视频文件 URL
	MediaURL string `json:"media_url"`

	// Language 语言
	Language Language `json:"language,omitempty"`

	// Format 输出字幕格式
	Format SubtitleFormat `json:"format,omitempty"`

	// EnableTranslation 是否翻译字幕
	EnableTranslation bool `json:"enable_translation,omitempty"`

	// TargetLanguage 翻译目标语言
	TargetLanguage Language `json:"target_language,omitempty"`

	// CallbackURL 回调地址
	CallbackURL string `json:"callback_url,omitempty"`
}

// SubtitleResult 字幕提取结果
type SubtitleResult struct {
	// SubtitleURL 字幕文件下载地址
	SubtitleURL string `json:"subtitle_url"`

	// Subtitles 字幕内容（JSON 格式时）
	Subtitles []SubtitleSegment `json:"subtitles,omitempty"`

	// Duration 媒体时长（毫秒）
	Duration int `json:"duration"`
}

// SubtitleTaskStatus 任务状态
type SubtitleTaskStatus struct {
	// TaskID 任务 ID
	TaskID string `json:"task_id"`

	// Status 状态
	Status TaskStatus `json:"status"`

	// Progress 进度（0-100）
	Progress int `json:"progress,omitempty"`

	// Result 结果
	Result *SubtitleResult `json:"result,omitempty"`

	// Error 错误信息
	Error *Error `json:"error,omitempty"`
}
