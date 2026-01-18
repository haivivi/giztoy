package minimax_interface

import (
	"net/http"
	"time"
)

// Client 是 MiniMax API 的主客户端。
type Client struct {
	// Text 文本生成服务
	Text TextService

	// Speech 语音合成服务
	Speech SpeechService

	// Voice 音色管理服务
	Voice VoiceService

	// Video 视频生成服务
	Video VideoService

	// Image 图片生成服务
	Image ImageService

	// Music 音乐生成服务
	Music MusicService

	// File 文件管理服务
	File FileService
}

// ClientConfig 客户端配置
type ClientConfig struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
	MaxRetries int
}

// Option 客户端配置选项
type Option func(*ClientConfig)

// WithBaseURL 设置 API 基础 URL
func WithBaseURL(url string) Option {
	return func(c *ClientConfig) {
		c.BaseURL = url
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(client *http.Client) Option {
	return func(c *ClientConfig) {
		c.HTTPClient = client
	}
}

// WithTimeout 设置请求超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(c *ClientConfig) {
		c.Timeout = timeout
	}
}

// WithRetry 设置最大重试次数
func WithRetry(maxRetries int) Option {
	return func(c *ClientConfig) {
		c.MaxRetries = maxRetries
	}
}

// NewClient 创建新的 MiniMax 客户端
//
// 示例:
//
//	client := minimax.NewClient("your-api-key")
//	client := minimax.NewClient("your-api-key", minimax.WithTimeout(30*time.Second))
func NewClient(apiKey string, opts ...Option) *Client {
	// TODO: 实现
	panic("not implemented")
}
