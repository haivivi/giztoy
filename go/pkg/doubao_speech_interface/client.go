package doubao_speech_interface

import (
	"net/http"
	"time"
)

// Client 豆包语音 API 客户端
type Client struct {
	TTS         TTSService
	ASR         ASRService
	VoiceClone  VoiceCloneService
	Realtime    RealtimeService
	Meeting     MeetingService
	Podcast     PodcastService
	Translation TranslationService
	Media       MediaService
	Console     ConsoleService // 控制台管理服务

	config *clientConfig
}

// clientConfig 客户端配置
type clientConfig struct {
	appID       string
	accessToken string // Bearer Token 认证
	accessKey   string // API Key 认证
	appKey      string
	cluster     string // 集群名称，如 volcano_tts
	baseURL     string
	wsURL       string
	httpClient  *http.Client
	timeout     time.Duration
}

// Option 配置选项函数
type Option func(*clientConfig)

// NewClient 创建豆包语音客户端
//
// appID 为应用 ID，可在火山引擎控制台获取
func NewClient(appID string, opts ...Option) *Client {
	// 实现略
	return nil
}

// WithBearerToken 使用 Bearer Token 认证
//
// token 格式为从控制台获取的 access_token
// 认证头格式: Authorization: Bearer;{token}
func WithBearerToken(token string) Option {
	return func(c *clientConfig) {
		c.accessToken = token
	}
}

// WithAPIKey 使用 API Key 认证
//
// 认证头格式:
//   - X-Api-Access-Key: {accessKey}
//   - X-Api-App-Key: {appKey}
func WithAPIKey(accessKey, appKey string) Option {
	return func(c *clientConfig) {
		c.accessKey = accessKey
		c.appKey = appKey
	}
}

// WithCluster 设置集群名称
//
// 常用集群：
//   - volcano_tts: TTS 服务
//   - volcano_asr: ASR 服务
func WithCluster(cluster string) Option {
	return func(c *clientConfig) {
		c.cluster = cluster
	}
}

// WithBaseURL 设置 HTTP API 基础 URL
//
// 默认为 https://openspeech.bytedance.com
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.baseURL = url
	}
}

// WithWebSocketURL 设置 WebSocket URL
//
// 默认为 wss://openspeech.bytedance.com
func WithWebSocketURL(url string) Option {
	return func(c *clientConfig) {
		c.wsURL = url
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// WithTimeout 设置请求超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		c.timeout = timeout
	}
}
