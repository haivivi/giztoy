package minimax_interface

import (
	"context"
)

// MusicService 音乐生成服务接口
type MusicService interface {
	// Generate 生成音乐
	//
	// 根据歌曲描述和歌词生成带人声的歌曲。
	Generate(ctx context.Context, req *MusicRequest) (*MusicResponse, error)
}

// MusicRequest 音乐生成请求
type MusicRequest struct {
	// Model 模型名称
	Model string `json:"model,omitempty"`

	// Prompt 音乐创作灵感，描述风格、情绪、场景等 (10-300 字符)
	Prompt string `json:"prompt"`

	// Lyrics 歌词内容 (10-600 字符)
	// 使用 \n 分隔每行，支持结构标签: [Intro], [Verse], [Chorus], [Bridge], [Outro]
	Lyrics string `json:"lyrics"`

	// SampleRate 采样率: 16000, 24000, 32000, 44100
	SampleRate int `json:"sample_rate,omitempty"`

	// Bitrate 比特率: 32000, 64000, 128000, 256000
	Bitrate int `json:"bitrate,omitempty"`

	// Format 音频格式: mp3, wav, pcm
	Format string `json:"format,omitempty"`
}

// MusicResponse 音乐生成响应
type MusicResponse struct {
	// Audio 音频数据（已从 hex 解码）
	Audio []byte `json:"-"`

	// Duration 音频时长（毫秒）
	Duration int `json:"duration"`

	// ExtraInfo 音频信息
	ExtraInfo *AudioInfo `json:"extra_info"`
}
