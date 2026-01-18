package doubao_speech_interface

import (
	"context"
)

// VoiceCloneService 声音复刻服务接口
type VoiceCloneService interface {
	// Train 训练自定义音色
	Train(ctx context.Context, req *VoiceCloneTrainRequest) (*Task[VoiceCloneResult], error)

	// GetStatus 查询训练状态
	GetStatus(ctx context.Context, speakerID string) (*VoiceCloneStatus, error)

	// List 列出已训练的音色
	List(ctx context.Context) ([]*VoiceCloneInfo, error)

	// Delete 删除已训练的音色
	Delete(ctx context.Context, speakerID string) error
}

// VoiceCloneModelType 声音复刻模型类型
type VoiceCloneModelType string

const (
	VoiceCloneModelStandard VoiceCloneModelType = "standard" // 标准版
	VoiceCloneModelPro      VoiceCloneModelType = "pro"      // 专业版
)

// VoiceCloneStatusType 训练状态类型
type VoiceCloneStatusType string

const (
	VoiceCloneStatusPending    VoiceCloneStatusType = "pending"
	VoiceCloneStatusProcessing VoiceCloneStatusType = "processing"
	VoiceCloneStatusSuccess    VoiceCloneStatusType = "success"
	VoiceCloneStatusFailed     VoiceCloneStatusType = "failed"
)

// VoiceCloneTrainRequest 声音复刻训练请求
type VoiceCloneTrainRequest struct {
	// SpeakerID 自定义音色 ID
	//
	// 格式通常为 S_XXXXXX
	SpeakerID string `json:"speaker_id"`

	// AudioURLs 训练音频 URL 列表
	AudioURLs []string `json:"audio_urls,omitempty"`

	// AudioData 训练音频数据（与 AudioURLs 二选一）
	AudioData [][]byte `json:"-"`

	// Text 参考文本（可选，用于提高训练质量）
	Text string `json:"text,omitempty"`

	// Language 语言
	Language Language `json:"language"`

	// ModelType 模型类型
	ModelType VoiceCloneModelType `json:"model_type,omitempty"`
}

// VoiceCloneResult 训练结果
type VoiceCloneResult struct {
	// SpeakerID 音色 ID
	SpeakerID string `json:"speaker_id"`

	// Status 状态
	Status VoiceCloneStatusType `json:"status"`

	// Message 状态消息
	Message string `json:"message,omitempty"`
}

// VoiceCloneStatus 训练状态
type VoiceCloneStatus struct {
	// SpeakerID 音色 ID
	SpeakerID string `json:"speaker_id"`

	// Status 状态
	Status VoiceCloneStatusType `json:"status"`

	// Progress 进度（0-100）
	Progress int `json:"progress,omitempty"`

	// Message 状态消息
	Message string `json:"message,omitempty"`

	// CreatedAt 创建时间（Unix 时间戳）
	CreatedAt int64 `json:"created_at"`

	// UpdatedAt 更新时间（Unix 时间戳）
	UpdatedAt int64 `json:"updated_at"`
}

// VoiceCloneInfo 已训练音色信息
type VoiceCloneInfo struct {
	// SpeakerID 音色 ID
	SpeakerID string `json:"speaker_id"`

	// Status 状态
	Status VoiceCloneStatusType `json:"status"`

	// Language 语言
	Language Language `json:"language"`

	// ModelType 模型类型
	ModelType VoiceCloneModelType `json:"model_type"`

	// CreatedAt 创建时间（Unix 时间戳）
	CreatedAt int64 `json:"created_at"`
}
