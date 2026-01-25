// Package doubaospeech provides a Go SDK for Volcengine Doubao Speech APIs.
//
// # Authentication
//
// V1 API (Classic):
//
//	client := NewClient(appID, WithBearerToken(token))
//	// Header: Authorization: Bearer {token}
//
// V2/V3 API (BigModel):
//
//	client := NewClient(appID, WithBearerToken(token))
//	// Headers:
//	//   X-Api-App-Id: {app_id}
//	//   X-Api-Access-Key: {token}
//	//   X-Api-Resource-Id: {resource_id}  // varies by service
//
// # Resource ID and Speaker Matching (IMPORTANT!)
//
// The most common error is "resource ID is mismatched with speaker related resource".
// This means the speaker voice suffix doesn't match the resource ID, NOT that the service
// is not enabled.
//
//	| Resource ID    | Required Speaker Suffix | Example Speaker                      |
//	|----------------|-------------------------|--------------------------------------|
//	| seed-tts-2.0   | *_uranus_bigtts         | zh_female_xiaohe_uranus_bigtts       |
//	| seed-tts-1.0   | *_moon_bigtts           | zh_female_shuangkuaisisi_moon_bigtts |
//	| seed-icl-*     | *_saturn_bigtts         | (voice clone results)                |
//	| V1 cluster     | no suffix               | zh_female_cancan                     |
//
// # Services
//
// The client provides access to both V1 (classic) and V2/V3 (BigModel) services:
//
//   - client.TTS: TTS V1 (classic, /api/v1/tts)
//   - client.TTSV2: TTS V2 (BigModel, /api/v3/tts/*)
//   - client.ASR: ASR V1 (classic, /api/v1/asr)
//   - client.ASRV2: ASR V2 (BigModel, /api/v3/sauc/*)
//   - client.Podcast: Podcast synthesis (V1 HTTP + V3 SAMI WebSocket)
//   - client.Realtime: Real-time dialogue (V3 WebSocket)
//   - client.Voice: Voice cloning
//
// See AGENTS.md in this package for more detailed development notes.
package doubaospeech

import (
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://openspeech.bytedance.com"
	defaultWSURL   = "wss://openspeech.bytedance.com"
	defaultTimeout = 30 * time.Second
)

// Fixed App Keys for V3 APIs (documented constants)
// These are fixed values provided in official documentation, not user credentials.
const (
	// AppKeyRealtime is the fixed X-Api-App-Key for Realtime dialogue API
	// Doc: https://www.volcengine.com/docs/6561/1354629
	AppKeyRealtime = "PlgvMymc7f3tQnJ6"

	// AppKeyPodcast is the fixed X-Api-App-Key for Podcast TTS API
	// Doc: https://www.volcengine.com/docs/6561/1668014
	AppKeyPodcast = "aGjiRDfUWi"
)

// V2/V3 API Resource IDs
const (
	// TTS Resource IDs
	ResourceTTSV1       = "seed-tts-1.0"         // 大模型 TTS 1.0 (字符版)
	ResourceTTSV1Concur = "seed-tts-1.0-concurr" // 大模型 TTS 1.0 (并发版)
	ResourceTTSV2       = "seed-tts-2.0"         // 大模型 TTS 2.0 (字符版)
	ResourceTTSV2Concur = "seed-tts-2.0-concurr" // 大模型 TTS 2.0 (并发版)
	ResourceVoiceCloneV1 = "seed-icl-1.0"        // 声音复刻 1.0
	ResourceVoiceCloneV2 = "seed-icl-2.0"        // 声音复刻 2.0

	// ASR Resource IDs
	ResourceASRStream   = "volc.bigasr.sauc.duration"  // 大模型流式语音识别 (时长版)
	ResourceASRStreamV2 = "volc.seedasr.sauc.duration" // 大模型流式语音识别 2.0
	ResourceASRFile     = "volc.bigasr.auc.duration"   // 大模型录音文件识别

	// Other Resource IDs
	ResourceRealtime    = "volc.speech.dialog"        // 端到端实时语音大模型
	ResourcePodcast     = "volc.service_type.10050"   // 播客语音合成
	ResourceTranslation = "volc.megatts.simt"         // 同声传译
)

// Client represents Doubao Speech API client
type Client struct {
	// V1 Services (Classic, /api/v1/*)
	TTS *TTSService // TTS 经典版 (/api/v1/tts)
	ASR *ASRService // ASR 经典版 (/api/v1/asr)

	// V2 Services (BigModel, /api/v3/*)
	TTSV2 *TTSServiceV2 // TTS 大模型版 (/api/v3/tts/*)
	ASRV2 *ASRServiceV2 // ASR 大模型版 (/api/v3/sauc/*, /api/v3/asr/*)

	// Shared Services (same API for both versions)
	VoiceClone  *VoiceCloneService  // 声音复刻
	Realtime    *RealtimeService    // 端到端实时语音大模型
	Meeting     *MeetingService     // 会议纪要
	Podcast     *PodcastService     // 播客合成
	Translation *TranslationService // 同声传译
	Media       *MediaService       // 音视频字幕提取

	config *clientConfig
}

// clientConfig represents client configuration
type clientConfig struct {
	appID       string
	accessToken string // Bearer Token auth (for V1 APIs)
	accessKey   string // X-Api-Access-Key auth (for V2/V3 APIs)
	appKey      string // X-Api-App-Key (for V2/V3 APIs, same as appID)
	apiKey      string // x-api-key auth (simple API Key, for all APIs)
	cluster     string // Cluster name, e.g. volcano_tts (V1 only)
	resourceID  string // Resource ID for V2 APIs (e.g. seed-tts-2.0)
	baseURL     string
	wsURL       string
	httpClient  *http.Client
	timeout     time.Duration
	userID      string // User identifier
}

// Option represents configuration option function
type Option func(*clientConfig)

// NewClient creates Doubao Speech client
//
// appID is the application ID from Volcano Engine console
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

	// Initialize V1 services (classic)
	c.TTS = newTTSService(c)
	c.ASR = newASRService(c)

	// Initialize V2 services (BigModel)
	c.TTSV2 = newTTSServiceV2(c)
	c.ASRV2 = newASRServiceV2(c)

	// Initialize shared services
	c.VoiceClone = newVoiceCloneService(c)
	c.Realtime = newRealtimeService(c)
	c.Meeting = newMeetingService(c)
	c.Podcast = newPodcastService(c)
	c.Translation = newTranslationService(c)
	c.Media = newMediaService(c)

	return c
}

// WithBearerToken uses Bearer Token authentication
//
// token is the access_token from console
// Header format: Authorization: Bearer {token}
func WithBearerToken(token string) Option {
	return func(c *clientConfig) {
		c.accessToken = token
	}
}

// WithAPIKey uses simple API Key authentication (recommended)
//
// apiKey is from: https://console.volcengine.com/speech/new/setting/apikeys
// Header format: x-api-key: {apiKey}
//
// This is the simplest authentication method for TTS/ASR APIs.
// No appid required in requests when using this method.
func WithAPIKey(apiKey string) Option {
	return func(c *clientConfig) {
		c.apiKey = apiKey
	}
}

// WithV2APIKey uses V2/V3 API Key authentication
//
// Header format:
//   - X-Api-Access-Key: {accessKey}
//   - X-Api-App-Key: {appKey}
//
// This is required for V2/V3 API endpoints (BigModel TTS, ASR, Realtime, etc.).
// accessKey is the Bearer Token, appKey is the AppID.
func WithV2APIKey(accessKey, appKey string) Option {
	return func(c *clientConfig) {
		c.accessKey = accessKey
		c.appKey = appKey
	}
}

// WithRealtimeAPIKey is an alias for WithV2APIKey for backward compatibility
func WithRealtimeAPIKey(accessKey, appKey string) Option {
	return WithV2APIKey(accessKey, appKey)
}

// WithResourceID sets the default resource ID for V2 APIs
func WithResourceID(resourceID string) Option {
	return func(c *clientConfig) {
		c.resourceID = resourceID
	}
}

// WithCluster sets cluster name
//
// Common clusters:
//   - volcano_tts: TTS service
//   - volcano_asr: ASR service
func WithCluster(cluster string) Option {
	return func(c *clientConfig) {
		c.cluster = cluster
	}
}

// WithBaseURL sets HTTP API base URL
//
// Default: https://openspeech.bytedance.com
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.baseURL = url
	}
}

// WithWebSocketURL sets WebSocket URL
//
// Default: wss://openspeech.bytedance.com
func WithWebSocketURL(url string) Option {
	return func(c *clientConfig) {
		c.wsURL = url
	}
}

// WithHTTPClient sets custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// WithTimeout sets request timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		c.timeout = timeout
	}
}

// WithUserID sets user identifier
func WithUserID(userID string) Option {
	return func(c *clientConfig) {
		c.userID = userID
	}
}

// setAuthHeaders sets authentication headers for V1 APIs
func (c *Client) setAuthHeaders(req *http.Request) {
	if c.config.apiKey != "" {
		// Simple API Key (recommended)
		req.Header.Set("x-api-key", c.config.apiKey)
	} else if c.config.accessToken != "" {
		// Bearer Token (note: format is "Bearer;{token}" not "Bearer {token}")
		req.Header.Set("Authorization", "Bearer;"+c.config.accessToken)
	} else if c.config.accessKey != "" {
		// V2/V3 API Key (fallback for V1)
		req.Header.Set("X-Api-Access-Key", c.config.accessKey)
		req.Header.Set("X-Api-App-Key", c.config.appKey)
	}
}

// setV2AuthHeaders sets authentication headers for V2/V3 APIs
//
// V2 APIs use X-Api-* headers:
//   - X-Api-App-Key: AppID
//   - X-Api-Access-Key: Bearer Token
//   - X-Api-Resource-Id: Resource ID (e.g. seed-tts-2.0)
//   - X-Api-Connect-Id: Connection ID (for WebSocket)
func (c *Client) setV2AuthHeaders(req *http.Request, resourceID string) {
	// Set App Key (AppID)
	req.Header.Set("X-Api-App-Key", c.config.appID)

	// Set Access Key (Bearer Token)
	if c.config.accessKey != "" {
		req.Header.Set("X-Api-Access-Key", c.config.accessKey)
	} else if c.config.accessToken != "" {
		req.Header.Set("X-Api-Access-Key", c.config.accessToken)
	} else if c.config.apiKey != "" {
		// x-api-key also works for V2 APIs
		req.Header.Set("x-api-key", c.config.apiKey)
	}

	// Set Resource ID
	if resourceID != "" {
		req.Header.Set("X-Api-Resource-Id", resourceID)
	} else if c.config.resourceID != "" {
		req.Header.Set("X-Api-Resource-Id", c.config.resourceID)
	}
}

// getV2WSHeaders returns WebSocket headers for V2/V3 APIs
func (c *Client) getV2WSHeaders(resourceID, connectID string) http.Header {
	headers := http.Header{}

	// Set X-Api-App-Key based on resource type (some APIs use fixed app keys)
	switch resourceID {
	case ResourceRealtime:
		headers.Set("X-Api-App-Key", AppKeyRealtime) // Fixed value from documentation
	case ResourcePodcast:
		headers.Set("X-Api-App-Key", AppKeyPodcast) // Fixed value from documentation
	default:
		headers.Set("X-Api-App-Key", c.config.appID)
	}

	// Set X-Api-App-Id for all V3 APIs
	headers.Set("X-Api-App-Id", c.config.appID)

	if c.config.accessKey != "" {
		headers.Set("X-Api-Access-Key", c.config.accessKey)
	} else if c.config.accessToken != "" {
		headers.Set("X-Api-Access-Key", c.config.accessToken)
	} else if c.config.apiKey != "" {
		headers.Set("x-api-key", c.config.apiKey)
	}

	if resourceID != "" {
		headers.Set("X-Api-Resource-Id", resourceID)
	}
	if connectID != "" {
		headers.Set("X-Api-Connect-Id", connectID)
	}

	return headers
}

// getWSAuthParams gets WebSocket authentication parameters
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
