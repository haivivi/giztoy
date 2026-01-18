package doubao_speech_interface

import (
	"context"
	"io"
	"iter"
)

// ASRService 语音识别服务接口
type ASRService interface {
	// RecognizeOneSentence 一句话识别（ASR 1.0）
	//
	// 适用于 60 秒以内的短音频
	RecognizeOneSentence(ctx context.Context, req *OneSentenceRequest) (*ASRResult, error)

	// OpenStreamSession 打开流式识别会话（ASR 2.0）
	//
	// 适用于实时音频流
	OpenStreamSession(ctx context.Context, config *StreamASRConfig) (ASRStreamSession, error)

	// RecognizeFile 文件识别（ASR 2.0）
	//
	// 适用于较长的音频文件，返回异步任务
	RecognizeFile(ctx context.Context, req *FileASRRequest) (*Task[ASRResult], error)
}

// ================== 一句话识别 ==================

// OneSentenceRequest 一句话识别请求
type OneSentenceRequest struct {
	// Audio 音频数据（与 AudioURL、AudioReader 三选一）
	Audio []byte `json:"-"`

	// AudioURL 音频文件 URL（与 Audio、AudioReader 三选一）
	AudioURL string `json:"audio_url,omitempty"`

	// AudioReader 音频数据流（与 Audio、AudioURL 三选一）
	AudioReader io.Reader `json:"-"`

	// Format 音频格式
	Format AudioFormat `json:"format"`

	// SampleRate 采样率
	SampleRate SampleRate `json:"sample_rate,omitempty"`

	// Language 语言
	Language Language `json:"language,omitempty"`

	// EnableITN 启用逆文本正则化（数字、日期等转换）
	EnableITN bool `json:"enable_itn,omitempty"`

	// EnablePunc 启用标点预测
	EnablePunc bool `json:"enable_punc,omitempty"`

	// EnableDDC 启用顺滑（去除口语化重复）
	EnableDDC bool `json:"enable_ddc,omitempty"`
}

// ASRResult 识别结果
type ASRResult struct {
	// Text 识别文本
	Text string `json:"text"`

	// Duration 音频时长（毫秒）
	Duration int `json:"duration"`

	// Utterances 分句详情
	Utterances []Utterance `json:"utterances,omitempty"`
}

// Utterance 分句信息
type Utterance struct {
	// Text 分句文本
	Text string `json:"text"`

	// StartTime 开始时间（毫秒）
	StartTime int `json:"start_time"`

	// EndTime 结束时间（毫秒）
	EndTime int `json:"end_time"`

	// Definite 是否为确定结果（流式识别中使用）
	Definite bool `json:"definite"`

	// Words 词级别详情
	Words []Word `json:"words,omitempty"`
}

// Word 词信息
type Word struct {
	// Text 词文本
	Text string `json:"text"`

	// StartTime 开始时间（毫秒）
	StartTime int `json:"start_time"`

	// EndTime 结束时间（毫秒）
	EndTime int `json:"end_time"`
}

// ================== 流式识别 ==================

// StreamASRConfig 流式识别配置
type StreamASRConfig struct {
	// Format 音频格式
	Format AudioFormat `json:"format"`

	// SampleRate 采样率
	SampleRate SampleRate `json:"sample_rate"`

	// Bits 位深度，通常为 16
	Bits int `json:"bits"`

	// Channel 声道数，通常为 1
	Channel int `json:"channel"`

	// Language 语言
	Language Language `json:"language,omitempty"`

	// ModelName 模型名称
	//
	// 如 "bigmodel" 表示使用大模型
	ModelName string `json:"model_name,omitempty"`

	// EnableITN 启用逆文本正则化
	EnableITN bool `json:"enable_itn,omitempty"`

	// EnablePunc 启用标点预测
	EnablePunc bool `json:"enable_punc,omitempty"`

	// EnableDDC 启用顺滑
	EnableDDC bool `json:"enable_ddc,omitempty"`

	// ShowUtterances 是否返回分句详情
	ShowUtterances bool `json:"show_utterances,omitempty"`

	// EnableNonstream 是否使用非流式模式（等待完整结果）
	EnableNonstream bool `json:"enable_nonstream,omitempty"`
}

// ASRStreamSession 流式识别会话
type ASRStreamSession interface {
	// SendAudio 发送音频数据
	//
	// isLast 为 true 时表示音频输入结束
	SendAudio(ctx context.Context, audio []byte, isLast bool) error

	// Recv 接收识别结果流
	Recv() iter.Seq2[*ASRChunk, error]

	// Close 关闭会话
	Close() error
}

// ASRChunk 流式识别结果块
type ASRChunk struct {
	// Text 当前识别文本
	Text string `json:"text"`

	// IsDefinite 是否为确定结果（非确定结果可能会被后续修正）
	IsDefinite bool `json:"is_definite"`

	// IsFinal 是否为最终结果
	IsFinal bool `json:"is_final"`

	// Utterances 分句详情
	Utterances []Utterance `json:"utterances,omitempty"`

	// AudioInfo 音频信息（最终结果中返回）
	AudioInfo *AudioInfo `json:"audio_info,omitempty"`

	// Sequence 序列号
	Sequence int32 `json:"sequence"`
}

// ================== 文件识别 ==================

// FileASRRequest 文件识别请求
type FileASRRequest struct {
	// AudioURL 音频文件 URL
	AudioURL string `json:"audio_url"`

	// Format 音频格式
	Format AudioFormat `json:"format,omitempty"`

	// Language 语言
	Language Language `json:"language,omitempty"`

	// EnableITN 启用逆文本正则化
	EnableITN bool `json:"enable_itn,omitempty"`

	// EnablePunc 启用标点预测
	EnablePunc bool `json:"enable_punc,omitempty"`

	// EnableDDC 启用顺滑
	EnableDDC bool `json:"enable_ddc,omitempty"`

	// EnableTimestamp 启用时间戳
	EnableTimestamp bool `json:"enable_timestamp,omitempty"`

	// CallbackURL 回调地址（可选）
	CallbackURL string `json:"callback_url,omitempty"`
}
