package doubao_speech_interface

import (
	"context"
	"iter"
)

// TranslationService 同声传译服务接口
type TranslationService interface {
	// OpenSession 打开同传会话
	OpenSession(ctx context.Context, config *TranslationConfig) (TranslationSession, error)
}

// TranslationConfig 同传配置
type TranslationConfig struct {
	// SourceLanguage 源语言
	SourceLanguage Language `json:"source_language"`

	// TargetLanguage 目标语言
	TargetLanguage Language `json:"target_language"`

	// AudioConfig 音频配置
	AudioConfig StreamASRConfig `json:"audio_config"`

	// EnableTTS 是否同时输出翻译后的语音
	EnableTTS bool `json:"enable_tts,omitempty"`

	// TTSVoice TTS 音色（启用 TTS 时）
	TTSVoice string `json:"tts_voice,omitempty"`
}

// TranslationSession 同传会话
type TranslationSession interface {
	// SendAudio 发送音频数据
	//
	// isLast 为 true 时表示音频输入结束
	SendAudio(ctx context.Context, audio []byte, isLast bool) error

	// Recv 接收翻译结果流
	Recv() iter.Seq2[*TranslationChunk, error]

	// Close 关闭会话
	Close() error
}

// TranslationChunk 翻译结果块
type TranslationChunk struct {
	// SourceText 源语言识别文本
	SourceText string `json:"source_text"`

	// TargetText 目标语言翻译文本
	TargetText string `json:"target_text"`

	// Audio 翻译后的音频（如果启用 TTS）
	Audio []byte `json:"audio,omitempty"`

	// IsDefinite 是否为确定结果
	IsDefinite bool `json:"is_definite"`

	// IsFinal 是否为最终结果
	IsFinal bool `json:"is_final"`

	// Sequence 序列号
	Sequence int32 `json:"sequence"`
}
