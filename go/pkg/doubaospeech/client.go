package doubaospeech

import (
	"net/http"
	"time"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

const (
	defaultBaseURL = "https://openspeech.bytedance.com"
	defaultWSURL   = "wss://openspeech.bytedance.com"
	defaultTimeout = 30 * time.Second
)

// Client 豆包语音 API 客户端实现
type Client struct {
	TTS         iface.TTSService
	ASR         iface.ASRService
	VoiceClone  iface.VoiceCloneService
	Realtime    iface.RealtimeService
	Meeting     iface.MeetingService
	Podcast     iface.PodcastService
	Translation iface.TranslationService
	Media       iface.MediaService
	Console     iface.ConsoleService

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
	userID      string // 用户标识
}

// Option 配置选项函数
type Option func(*clientConfig)

// NewClient 创建豆包语音客户端
//
// appID 为应用 ID，可在火山引擎控制台获取
func NewClient(appID string, opts ...Option) *Client {
	config := &clientConfig{
		appID:   appID,
		baseURL: defaultBaseURL,
		wsURL:   defaultWSURL,
		timeout: defaultTimeout,
		userID:  "default_user",
	}

	for _, opt := range opts {
		opt(config)
	}

	if config.httpClient == nil {
		config.httpClient = &http.Client{
			Timeout: config.timeout,
		}
	}

	c := &Client{
		config: config,
	}

	// 初始化各服务
	c.TTS = newTTSService(c)
	c.ASR = newASRService(c)
	c.VoiceClone = newVoiceCloneService(c)
	c.Realtime = newRealtimeService(c)
	c.Meeting = newMeetingService(c)
	c.Podcast = newPodcastService(c)
	c.Translation = newTranslationService(c)
	c.Media = newMediaService(c)
	c.Console = newConsoleService(c)

	return c
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

// WithUserID 设置用户标识
func WithUserID(userID string) Option {
	return func(c *clientConfig) {
		c.userID = userID
	}
}

// setAuthHeaders 设置认证请求头
func (c *Client) setAuthHeaders(req *http.Request) {
	if c.config.accessToken != "" {
		req.Header.Set("Authorization", "Bearer;"+c.config.accessToken)
	} else if c.config.accessKey != "" {
		req.Header.Set("X-Api-Access-Key", c.config.accessKey)
		req.Header.Set("X-Api-App-Key", c.config.appKey)
	}
}

// getWSAuthParams 获取 WebSocket 认证参数
func (c *Client) getWSAuthParams() string {
	params := "appid=" + c.config.appID
	if c.config.accessToken != "" {
		params += "&token=" + c.config.accessToken
	}
	if c.config.cluster != "" {
		params += "&cluster=" + c.config.cluster
	}
	return params
}
