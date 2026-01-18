package minimax_interface

import (
	"context"
)

// ImageService 图片生成服务接口
type ImageService interface {
	// Generate 文生图
	Generate(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error)

	// GenerateWithReference 图生图（带参考图）
	GenerateWithReference(ctx context.Context, req *ImageReferenceRequest) (*ImageResponse, error)
}

// ImageGenerateRequest 图片生成请求
type ImageGenerateRequest struct {
	// Model 模型名称
	Model string `json:"model"`

	// Prompt 图片描述文本
	Prompt string `json:"prompt"`

	// AspectRatio 图片比例: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3, 21:9, 9:21
	AspectRatio string `json:"aspect_ratio,omitempty"`

	// N 生成数量 (1-9)
	N int `json:"n,omitempty"`

	// PromptOptimizer 是否优化 prompt
	PromptOptimizer *bool `json:"prompt_optimizer,omitempty"`
}

// ImageReferenceRequest 带参考图的图片生成请求
type ImageReferenceRequest struct {
	ImageGenerateRequest

	// ImagePrompt 参考图片 URL
	ImagePrompt string `json:"image_prompt"`

	// ImagePromptStrength 参考图片影响强度 (0-1)
	ImagePromptStrength float64 `json:"image_prompt_strength,omitempty"`
}

// ImageResponse 图片生成响应
type ImageResponse struct {
	Images []ImageData `json:"images"`
}

// ImageData 图片数据
type ImageData struct {
	// URL 图片 URL
	URL string `json:"url"`
}
