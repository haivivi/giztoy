package doubao_speech_interface

import (
	"context"
	"iter"
)

// RealtimeService 端到端实时对话服务接口
type RealtimeService interface {
	// Connect 建立实时对话连接
	Connect(ctx context.Context, config *RealtimeConfig) (RealtimeSession, error)
}

// ================== 配置结构 ==================

// RealtimeConfig 实时对话配置
type RealtimeConfig struct {
	// ASR 语音识别配置
	ASR RealtimeASRConfig `json:"asr"`

	// TTS 语音合成配置
	TTS RealtimeTTSConfig `json:"tts"`

	// Dialog 对话配置
	Dialog RealtimeDialogConfig `json:"dialog"`
}

// RealtimeASRConfig ASR 配置
type RealtimeASRConfig struct {
	// Extra 扩展参数
	//
	// 如 end_smooth_window_ms 用于控制端点检测平滑窗口
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// RealtimeTTSConfig TTS 配置
type RealtimeTTSConfig struct {
	// Speaker 音色 ID
	//
	// 可使用预置音色如 zh_female_vv_jupiter_bigtts
	// 或自定义复刻音色如 S_XXXXXX
	Speaker string `json:"speaker"`

	// AudioConfig 音频配置
	AudioConfig RealtimeAudioConfig `json:"audio_config"`
}

// RealtimeAudioConfig 音频配置
type RealtimeAudioConfig struct {
	// Channel 声道数，通常为 1
	Channel int `json:"channel"`

	// Format 音频格式
	//
	// pcm 对应 f32le 格式，pcm_s16le 对应 s16le 格式
	Format string `json:"format"`

	// SampleRate 采样率，通常为 24000
	SampleRate int `json:"sample_rate"`
}

// RealtimeDialogConfig 对话配置
type RealtimeDialogConfig struct {
	// BotName 机器人名称
	BotName string `json:"bot_name"`

	// SystemRole 系统角色设定
	//
	// 描述 AI 的身份、性格等
	SystemRole string `json:"system_role"`

	// SpeakingStyle 说话风格
	//
	// 描述说话的方式、语气等
	SpeakingStyle string `json:"speaking_style"`

	// CharacterManifest 角色详细设定（用于复刻音色）
	//
	// 包含角色的外貌、性格、口头禅等详细描述
	CharacterManifest string `json:"character_manifest,omitempty"`

	// Location 位置信息
	Location *LocationInfo `json:"location,omitempty"`

	// Extra 扩展参数
	//
	// 如 strict_audit, audit_response, recv_timeout, input_mod 等
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// ================== 会话接口 ==================

// RealtimeSession 实时对话会话
type RealtimeSession interface {
	// SendAudio 发送音频数据
	//
	// 音频格式应为 16kHz, 16bit, 单声道 PCM
	SendAudio(ctx context.Context, audio []byte) error

	// SendText 发送文本（纯文本输入模式）
	SendText(ctx context.Context, text string) error

	// SayHello 发送问候语，让 AI 主动说话
	SayHello(ctx context.Context, content string) error

	// Interrupt 中断当前 AI 说话
	Interrupt(ctx context.Context) error

	// Recv 接收事件流
	Recv() iter.Seq2[*RealtimeEvent, error]

	// SessionID 获取会话 ID
	SessionID() string

	// DialogID 获取对话 ID
	DialogID() string

	// Close 关闭会话
	Close() error
}

// ================== 事件类型 ==================

// RealtimeEventType 事件类型
type RealtimeEventType int

const (
	// 连接事件
	EventConnectionStarted  RealtimeEventType = 50
	EventConnectionFailed   RealtimeEventType = 51
	EventConnectionFinished RealtimeEventType = 52

	// 会话事件
	EventSessionStarted  RealtimeEventType = 150
	EventSessionFinished RealtimeEventType = 152
	EventSessionFailed   RealtimeEventType = 153

	// 音频事件
	EventAudioReceived RealtimeEventType = 200

	// SayHello 事件
	EventSayHelloStarted  RealtimeEventType = 300
	EventSayHelloFinished RealtimeEventType = 359

	// TTS 事件
	EventTTSStarted  RealtimeEventType = 350
	EventTTSFinished RealtimeEventType = 359

	// ASR 事件
	EventASRStarted  RealtimeEventType = 450
	EventASRFinished RealtimeEventType = 459

	// 文本输入事件
	EventChatTextQuery RealtimeEventType = 501
	EventChatRAGText   RealtimeEventType = 502
)

// ================== 事件结构 ==================

// RealtimeEvent 实时对话事件
type RealtimeEvent struct {
	// Type 事件类型
	Type RealtimeEventType `json:"type"`

	// SessionID 会话 ID
	SessionID string `json:"session_id"`

	// Audio 音频数据（TTS 输出）
	Audio []byte `json:"audio,omitempty"`

	// Text 文本内容（ASR 识别结果或 LLM 响应）
	Text string `json:"text,omitempty"`

	// ASRInfo ASR 详细信息
	ASRInfo *RealtimeASRInfo `json:"asr_info,omitempty"`

	// TTSInfo TTS 详细信息
	TTSInfo *RealtimeTTSInfo `json:"tts_info,omitempty"`

	// Error 错误信息
	Error *Error `json:"error,omitempty"`

	// Payload 原始 payload（JSON）
	Payload []byte `json:"payload,omitempty"`
}

// RealtimeASRInfo ASR 详细信息
type RealtimeASRInfo struct {
	// Text 识别文本
	Text string `json:"text"`

	// Utterances 分句详情
	Utterances []Utterance `json:"utterances,omitempty"`

	// IsFinal 是否为最终结果
	IsFinal bool `json:"is_final"`
}

// RealtimeTTSInfo TTS 详细信息
type RealtimeTTSInfo struct {
	// TTSType TTS 类型
	//
	// say_hello: 问候语
	// chat_tts_text: 插入式 TTS
	// external_rag: 外部 RAG 回复
	TTSType string `json:"tts_type"`

	// Content 文本内容
	Content string `json:"content,omitempty"`
}
