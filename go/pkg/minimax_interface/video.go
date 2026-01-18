package minimax_interface

import (
	"context"
)

// VideoService 视频生成服务接口
type VideoService interface {
	// CreateTextToVideo 创建文生视频任务
	CreateTextToVideo(ctx context.Context, req *TextToVideoRequest) (*Task[VideoResult], error)

	// CreateImageToVideo 创建图生视频任务
	CreateImageToVideo(ctx context.Context, req *ImageToVideoRequest) (*Task[VideoResult], error)

	// CreateFrameToVideo 创建首尾帧生成视频任务
	CreateFrameToVideo(ctx context.Context, req *FrameToVideoRequest) (*Task[VideoResult], error)

	// CreateSubjectRefVideo 创建主体参考生成视频任务
	CreateSubjectRefVideo(ctx context.Context, req *SubjectRefVideoRequest) (*Task[VideoResult], error)

	// CreateAgentTask 创建视频 Agent 任务（基于模板）
	CreateAgentTask(ctx context.Context, req *VideoAgentRequest) (*Task[VideoResult], error)
}

// TextToVideoRequest 文生视频请求
type TextToVideoRequest struct {
	// Model 模型名称
	Model string `json:"model"`

	// Prompt 视频描述文本
	Prompt string `json:"prompt"`

	// Duration 视频时长（秒），可选 6 或 10
	Duration int `json:"duration,omitempty"`

	// Resolution 分辨率: 768P 或 1080P
	Resolution string `json:"resolution,omitempty"`
}

// ImageToVideoRequest 图生视频请求
type ImageToVideoRequest struct {
	// Model 模型名称（I2V 系列）
	Model string `json:"model"`

	// Prompt 视频描述文本
	Prompt string `json:"prompt,omitempty"`

	// FirstFrameImage 首帧图片 URL 或 base64
	FirstFrameImage string `json:"first_frame_image"`

	// Duration 视频时长（秒）
	Duration int `json:"duration,omitempty"`

	// Resolution 分辨率
	Resolution string `json:"resolution,omitempty"`
}

// FrameToVideoRequest 首尾帧生成视频请求
type FrameToVideoRequest struct {
	// Model 模型名称
	Model string `json:"model"`

	// Prompt 视频描述文本
	Prompt string `json:"prompt,omitempty"`

	// FirstFrameImage 首帧图片
	FirstFrameImage string `json:"first_frame_image"`

	// LastFrameImage 尾帧图片
	LastFrameImage string `json:"last_frame_image"`
}

// SubjectRefVideoRequest 主体参考生成视频请求
type SubjectRefVideoRequest struct {
	// Model 模型名称
	Model string `json:"model"`

	// Prompt 视频描述文本
	Prompt string `json:"prompt"`

	// SubjectReference 主体参考图片
	SubjectReference string `json:"subject_reference"`
}

// VideoAgentRequest 视频 Agent 请求
type VideoAgentRequest struct {
	// TemplateID 模板 ID
	TemplateID string `json:"template_id"`

	// MediaInputs 媒体输入
	MediaInputs []MediaInput `json:"media_inputs,omitempty"`

	// TextInputs 文本输入
	TextInputs []TextInput `json:"text_inputs,omitempty"`
}

// MediaInput 媒体输入
type MediaInput struct {
	// Type 媒体类型: image 或 video
	Type string `json:"type"`

	// URL 媒体文件 URL
	URL string `json:"url,omitempty"`

	// FileID 媒体文件 ID
	FileID string `json:"file_id,omitempty"`
}

// TextInput 文本输入
type TextInput struct {
	// Key 输入键名
	Key string `json:"key"`

	// Value 输入值
	Value string `json:"value"`
}
