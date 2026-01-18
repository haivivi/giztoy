package minimax_interface

import (
	"context"
	"iter"
)

// SpeechService 语音合成服务接口
type SpeechService interface {
	// Synthesize 同步语音合成
	//
	// 单次最长处理 10,000 字符。返回的音频数据已自动从 hex 解码。
	Synthesize(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error)

	// SynthesizeStream 流式语音合成
	//
	// 返回一个迭代器，每次迭代返回一个 SpeechChunk。
	// 迭代结束或 break 时会自动关闭连接。
	//
	// 示例:
	//
	//	var buf bytes.Buffer
	//	for chunk, err := range client.Speech.SynthesizeStream(ctx, req) {
	//	    if err != nil {
	//	        return err
	//	    }
	//	    if chunk.Audio != nil {
	//	        buf.Write(chunk.Audio)
	//	    }
	//	}
	SynthesizeStream(ctx context.Context, req *SpeechRequest) iter.Seq2[*SpeechChunk, error]

	// CreateAsyncTask 创建异步长文本语音合成任务
	//
	// 支持最长 1,000,000 字符。返回 Task，可调用 Wait() 等待完成。
	CreateAsyncTask(ctx context.Context, req *AsyncSpeechRequest) (*Task[SpeechAsyncResult], error)
}

// SpeechRequest 语音合成请求
type SpeechRequest struct {
	// Model 模型版本
	Model string `json:"model"`

	// Text 需要合成的文本，限制 10,000 字符
	Text string `json:"text"`

	// VoiceSetting 语音设置
	VoiceSetting *VoiceSetting `json:"voice_setting,omitempty"`

	// AudioSetting 音频设置
	AudioSetting *AudioSetting `json:"audio_setting,omitempty"`

	// PronunciationDict 发音词典
	PronunciationDict *PronunciationDict `json:"pronunciation_dict,omitempty"`

	// LanguageBoost 语言增强
	LanguageBoost string `json:"language_boost,omitempty"`

	// SubtitleEnable 是否开启字幕
	SubtitleEnable bool `json:"subtitle_enable,omitempty"`

	// OutputFormat 输出格式: hex（默认）或 url
	OutputFormat OutputFormat `json:"output_format,omitempty"`
}

// AsyncSpeechRequest 异步语音合成请求
type AsyncSpeechRequest struct {
	// Model 模型版本
	Model string `json:"model"`

	// Text 需要合成的文本，最长 1,000,000 字符（与 FileID 二选一）
	Text string `json:"text,omitempty"`

	// FileID 文本文件的 file_id（与 Text 二选一）
	FileID string `json:"file_id,omitempty"`

	// VoiceSetting 语音设置
	VoiceSetting *VoiceSetting `json:"voice_setting,omitempty"`

	// AudioSetting 音频设置
	AudioSetting *AudioSetting `json:"audio_setting,omitempty"`

	// PronunciationDict 发音词典
	PronunciationDict *PronunciationDict `json:"pronunciation_dict,omitempty"`

	// LanguageBoost 语言增强
	LanguageBoost string `json:"language_boost,omitempty"`

	// SubtitleEnable 是否开启字幕
	SubtitleEnable bool `json:"subtitle_enable,omitempty"`
}

// VoiceSetting 语音设置
type VoiceSetting struct {
	// VoiceID 音色 ID
	VoiceID string `json:"voice_id"`

	// Speed 语速，范围 0.5-2.0，默认 1.0
	Speed float64 `json:"speed,omitempty"`

	// Vol 音量，范围 0-10，默认 1.0
	Vol float64 `json:"vol,omitempty"`

	// Pitch 音调，范围 -12 到 12，默认 0
	Pitch int `json:"pitch,omitempty"`

	// Emotion 情绪: happy, sad, angry, fearful, disgusted, surprised, neutral
	Emotion string `json:"emotion,omitempty"`
}

// AudioSetting 音频设置
type AudioSetting struct {
	// SampleRate 采样率: 8000, 16000, 22050, 24000, 32000, 44100
	SampleRate int `json:"sample_rate,omitempty"`

	// Bitrate 比特率: 32000, 64000, 128000, 256000
	Bitrate int `json:"bitrate,omitempty"`

	// Format 音频格式: mp3, pcm, flac, wav（wav 仅非流式支持）
	Format AudioFormat `json:"format,omitempty"`

	// Channel 声道数: 1 或 2
	Channel int `json:"channel,omitempty"`
}

// PronunciationDict 发音词典
type PronunciationDict struct {
	// Tone 发音规则列表，如 ["处理/(chu3)(li3)", "危险/dangerous"]
	Tone []string `json:"tone"`
}

// OutputFormat 输出格式
type OutputFormat string

const (
	OutputFormatHex OutputFormat = "hex"
	OutputFormatURL OutputFormat = "url"
)

// AudioFormat 音频格式
type AudioFormat string

const (
	AudioFormatMP3  AudioFormat = "mp3"
	AudioFormatPCM  AudioFormat = "pcm"
	AudioFormatFLAC AudioFormat = "flac"
	AudioFormatWAV  AudioFormat = "wav"
)

// SpeechResponse 语音合成响应
type SpeechResponse struct {
	// Audio 音频数据（已从 hex 解码）
	Audio []byte `json:"-"`

	// AudioURL 音频 URL（当 OutputFormat 为 url 时）
	AudioURL string `json:"audio_url,omitempty"`

	// ExtraInfo 音频信息
	ExtraInfo *AudioInfo `json:"extra_info"`

	// TraceID 请求追踪 ID
	TraceID string `json:"trace_id"`
}

// SpeechChunk 流式语音合成的数据块
type SpeechChunk struct {
	// Audio 音频数据（已从 hex 解码），可能为 nil
	Audio []byte

	// Status 状态码: 1=生成中, 2=完成
	Status int `json:"status"`

	// Subtitle 字幕片段（如果启用字幕且当前 chunk 包含字幕）
	Subtitle *SubtitleSegment `json:"subtitle,omitempty"`

	// ExtraInfo 音频信息（通常在最后一个 chunk 返回）
	ExtraInfo *AudioInfo `json:"extra_info,omitempty"`

	// TraceID 请求追踪 ID（通常在最后一个 chunk 返回）
	TraceID string `json:"trace_id,omitempty"`
}

// AudioInfo 音频信息
type AudioInfo struct {
	// AudioLength 音频时长（毫秒）
	AudioLength int `json:"audio_length"`

	// AudioSampleRate 采样率
	AudioSampleRate int `json:"audio_sample_rate"`

	// AudioSize 音频大小（字节）
	AudioSize int `json:"audio_size"`

	// Bitrate 比特率
	Bitrate int `json:"bitrate"`

	// WordCount 字数
	WordCount int `json:"word_count"`

	// UsageCharacters 计费字符数
	UsageCharacters int `json:"usage_characters"`

	// AudioFormat 音频格式
	AudioFormat string `json:"audio_format"`

	// AudioChannel 声道数
	AudioChannel int `json:"audio_channel"`
}
