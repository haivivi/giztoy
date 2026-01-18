package minimax_interface

import (
	"context"
	"io"
)

// VoiceService 音色管理服务接口
type VoiceService interface {
	// List 查询可用音色列表
	List(ctx context.Context, voiceType VoiceType) (*VoiceListResponse, error)

	// Delete 删除自定义音色
	Delete(ctx context.Context, voiceID string) error

	// UploadCloneAudio 上传待复刻的音频文件
	UploadCloneAudio(ctx context.Context, file io.Reader, filename string) (*UploadResponse, error)

	// UploadDemoAudio 上传示例音频文件（可选，增强复刻效果）
	UploadDemoAudio(ctx context.Context, file io.Reader, filename string) (*UploadResponse, error)

	// Clone 音色快速复刻
	//
	// 复刻的音色为临时音色，若 7 天内未被用于语音合成将被自动删除。
	Clone(ctx context.Context, req *VoiceCloneRequest) (*VoiceCloneResponse, error)

	// Design 音色设计
	//
	// 基于文本描述生成个性化音色。生成的音色为临时音色。
	Design(ctx context.Context, req *VoiceDesignRequest) (*VoiceDesignResponse, error)
}

// VoiceType 音色类型
type VoiceType string

const (
	VoiceTypeAll     VoiceType = "all"
	VoiceTypeSystem  VoiceType = "system"
	VoiceTypeCloning VoiceType = "voice_cloning"
)

// VoiceListResponse 音色列表响应
type VoiceListResponse struct {
	Voices []VoiceInfo `json:"voices"`
}

// VoiceInfo 音色信息
type VoiceInfo struct {
	// VoiceID 音色 ID
	VoiceID string `json:"voice_id"`

	// Name 音色名称
	Name string `json:"name"`

	// Type 音色类型: system, voice_cloning, voice_design
	Type string `json:"type"`

	// Language 支持的语言列表
	Language []string `json:"language,omitempty"`

	// Description 音色描述
	Description string `json:"description,omitempty"`

	// CreatedAt 创建时间（仅自定义音色）
	CreatedAt string `json:"created_at,omitempty"`
}

// UploadResponse 文件上传响应
type UploadResponse struct {
	FileID string `json:"file_id"`
}

// VoiceCloneRequest 音色复刻请求
type VoiceCloneRequest struct {
	// FileID 复刻音频的 file_id
	FileID string `json:"file_id"`

	// DemoFileID 示例音频的 file_id（可选）
	DemoFileID string `json:"demo_file_id,omitempty"`

	// VoiceID 自定义的音色 ID
	VoiceID string `json:"voice_id"`

	// Model 模型版本
	Model string `json:"model,omitempty"`

	// Text 试听文本
	Text string `json:"text,omitempty"`
}

// VoiceCloneResponse 音色复刻响应
type VoiceCloneResponse struct {
	// VoiceID 复刻后的音色 ID
	VoiceID string `json:"voice_id"`

	// DemoAudio 试听音频（已解码）
	DemoAudio []byte `json:"-"`
}

// VoiceDesignRequest 音色设计请求
type VoiceDesignRequest struct {
	// Prompt 音色描述
	Prompt string `json:"prompt"`

	// PreviewText 试听文本
	PreviewText string `json:"preview_text"`

	// VoiceID 自定义的音色 ID（可选）
	VoiceID string `json:"voice_id,omitempty"`

	// Model 模型版本
	Model string `json:"model,omitempty"`
}

// VoiceDesignResponse 音色设计响应
type VoiceDesignResponse struct {
	// VoiceID 设计后的音色 ID
	VoiceID string `json:"voice_id"`

	// DemoAudio 试听音频（已解码）
	DemoAudio []byte `json:"-"`
}
