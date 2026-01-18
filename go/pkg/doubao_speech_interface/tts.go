package doubao_speech_interface

import (
	"context"
	"io"
	"iter"
)

// TTSService 语音合成服务接口
type TTSService interface {
	// Synthesize 同步语音合成
	//
	// 适用于短文本（最大 4096 字符），返回完整音频
	Synthesize(ctx context.Context, req *TTSRequest) (*TTSResponse, error)

	// SynthesizeStream 流式语音合成（HTTP）
	//
	// 适用于需要低延迟首包的场景
	SynthesizeStream(ctx context.Context, req *TTSRequest) iter.Seq2[*TTSChunk, error]

	// SynthesizeStreamWS 流式语音合成（WebSocket）
	//
	// 适用于需要持续交互的场景
	SynthesizeStreamWS(ctx context.Context, req *TTSRequest) iter.Seq2[*TTSChunk, error]

	// OpenDuplexSession 打开双工流式会话
	//
	// 支持边输入文本边合成音频
	OpenDuplexSession(ctx context.Context, config *TTSDuplexConfig) (TTSDuplexSession, error)

	// CreateAsyncTask 创建异步合成任务
	//
	// 适用于长文本（超过 4096 字符）
	CreateAsyncTask(ctx context.Context, req *AsyncTTSRequest) (*Task[TTSAsyncResult], error)
}

// ================== 文本类型 ==================

// TTSTextType 文本类型
type TTSTextType string

const (
	TTSTextTypePlain TTSTextType = "plain" // 纯文本
	TTSTextTypeSSML  TTSTextType = "ssml"  // SSML 格式
)

// ================== 请求结构 ==================

// TTSRequest 语音合成请求
type TTSRequest struct {
	// Text 要合成的文本，最大 4096 字符
	Text string `json:"text"`

	// TextType 文本类型：plain 或 ssml
	TextType TTSTextType `json:"text_type,omitempty"`

	// VoiceType 音色 ID
	//
	// 可在火山引擎控制台查看可用音色列表
	VoiceType string `json:"voice_type"`

	// Encoding 音频编码格式，默认 mp3
	Encoding AudioEncoding `json:"encoding,omitempty"`

	// SampleRate 采样率，默认 24000
	SampleRate SampleRate `json:"sample_rate,omitempty"`

	// SpeedRatio 语速，范围 0.2-3.0，默认 1.0
	SpeedRatio float64 `json:"speed_ratio,omitempty"`

	// VolumeRatio 音量，范围 0.1-3.0，默认 1.0
	VolumeRatio float64 `json:"volume_ratio,omitempty"`

	// PitchRatio 音高，范围 0.1-3.0，默认 1.0
	PitchRatio float64 `json:"pitch_ratio,omitempty"`

	// Emotion 情感（部分音色支持）
	//
	// 如：happy, sad, angry 等
	Emotion string `json:"emotion,omitempty"`

	// Language 语言（多语言音色需指定）
	Language Language `json:"language,omitempty"`

	// EnableSubtitle 是否返回字幕时间戳
	EnableSubtitle bool `json:"enable_subtitle,omitempty"`

	// SilenceDuration 句尾静音时长（毫秒）
	SilenceDuration int `json:"silence_duration,omitempty"`
}

// ================== 响应结构 ==================

// TTSResponse 同步合成响应
type TTSResponse struct {
	// Audio 音频数据
	Audio []byte `json:"-"`

	// Duration 音频时长（毫秒）
	Duration int `json:"duration"`

	// Subtitles 字幕列表（如果启用）
	Subtitles []SubtitleSegment `json:"subtitles,omitempty"`

	// ReqID 请求 ID
	ReqID string `json:"reqid"`
}

// ToReader 将音频数据转换为 io.Reader
func (r *TTSResponse) ToReader() io.Reader {
	// 实现略
	return nil
}

// TTSChunk 流式合成数据块
type TTSChunk struct {
	// Audio 音频数据
	Audio []byte `json:"-"`

	// Sequence 序列号，负数表示最后一个
	Sequence int32 `json:"sequence"`

	// IsLast 是否为最后一个 chunk
	IsLast bool `json:"is_last"`

	// Subtitle 当前 chunk 对应的字幕（如果启用）
	Subtitle *SubtitleSegment `json:"subtitle,omitempty"`

	// Duration 当前 chunk 音频时长（毫秒）
	Duration int `json:"duration,omitempty"`
}

// ================== 双工会话 ==================

// TTSDuplexConfig 双工会话配置
type TTSDuplexConfig struct {
	// VoiceType 音色 ID
	VoiceType string `json:"voice_type"`

	// Encoding 音频编码格式
	Encoding AudioEncoding `json:"encoding,omitempty"`

	// SampleRate 采样率
	SampleRate SampleRate `json:"sample_rate,omitempty"`

	// SpeedRatio 语速
	SpeedRatio float64 `json:"speed_ratio,omitempty"`

	// VolumeRatio 音量
	VolumeRatio float64 `json:"volume_ratio,omitempty"`

	// PitchRatio 音高
	PitchRatio float64 `json:"pitch_ratio,omitempty"`
}

// TTSDuplexSession 双工流式会话
type TTSDuplexSession interface {
	// SendText 发送文本片段
	//
	// isLast 为 true 时表示文本输入结束
	SendText(ctx context.Context, text string, isLast bool) error

	// Recv 接收音频数据流
	Recv() iter.Seq2[*TTSChunk, error]

	// Close 关闭会话
	Close() error
}

// ================== 异步合成 ==================

// AsyncTTSRequest 异步合成请求
type AsyncTTSRequest struct {
	// Text 要合成的文本，支持长文本
	Text string `json:"text"`

	// TextType 文本类型
	TextType TTSTextType `json:"text_type,omitempty"`

	// VoiceType 音色 ID
	VoiceType string `json:"voice_type"`

	// Encoding 音频编码格式
	Encoding AudioEncoding `json:"encoding,omitempty"`

	// SampleRate 采样率
	SampleRate SampleRate `json:"sample_rate,omitempty"`

	// SpeedRatio 语速
	SpeedRatio float64 `json:"speed_ratio,omitempty"`

	// VolumeRatio 音量
	VolumeRatio float64 `json:"volume_ratio,omitempty"`

	// PitchRatio 音高
	PitchRatio float64 `json:"pitch_ratio,omitempty"`

	// CallbackURL 回调地址（可选）
	CallbackURL string `json:"callback_url,omitempty"`
}

// TTSAsyncResult 异步合成结果
type TTSAsyncResult struct {
	// AudioURL 音频文件下载地址
	AudioURL string `json:"audio_url"`

	// Duration 音频时长（毫秒）
	Duration int `json:"duration"`

	// Subtitles 字幕列表
	Subtitles []SubtitleSegment `json:"subtitles,omitempty"`
}
