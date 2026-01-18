package doubao_speech_interface

// ================== 音频编码格式 ==================

// AudioEncoding 音频编码格式（TTS 输出）
type AudioEncoding string

const (
	EncodingPCM      AudioEncoding = "pcm"
	EncodingWAV      AudioEncoding = "wav"
	EncodingMP3      AudioEncoding = "mp3"
	EncodingOGG      AudioEncoding = "ogg_opus"
	EncodingAAC      AudioEncoding = "aac"
	EncodingM4A      AudioEncoding = "m4a"
	EncodingPCMS16LE AudioEncoding = "pcm_s16le" // 用于实时对话，s16le 格式
	EncodingPCMF32LE AudioEncoding = "pcm"       // 用于实时对话，f32le 格式
)

// AudioFormat 音频格式（ASR 输入）
type AudioFormat string

const (
	FormatWAV AudioFormat = "wav"
	FormatMP3 AudioFormat = "mp3"
	FormatOGG AudioFormat = "ogg"
	FormatM4A AudioFormat = "m4a"
	FormatAAC AudioFormat = "aac"
	FormatPCM AudioFormat = "pcm"
	FormatRaw AudioFormat = "raw"
)

// ================== 采样率 ==================

// SampleRate 采样率
type SampleRate int

const (
	SampleRate8000  SampleRate = 8000
	SampleRate16000 SampleRate = 16000
	SampleRate22050 SampleRate = 22050
	SampleRate24000 SampleRate = 24000
	SampleRate32000 SampleRate = 32000
	SampleRate44100 SampleRate = 44100
	SampleRate48000 SampleRate = 48000
)

// ================== 语言 ==================

// Language 语言代码
type Language string

const (
	LanguageZhCN Language = "zh-CN" // 中文（普通话）
	LanguageEnUS Language = "en-US" // 英语（美式）
	LanguageEnGB Language = "en-GB" // 英语（英式）
	LanguageJaJP Language = "ja-JP" // 日语
	LanguageKoKR Language = "ko-KR" // 韩语
	LanguageEsES Language = "es-ES" // 西班牙语
	LanguageFrFR Language = "fr-FR" // 法语
	LanguageDeDE Language = "de-DE" // 德语
	LanguageItIT Language = "it-IT" // 意大利语
	LanguagePtBR Language = "pt-BR" // 葡萄牙语（巴西）
	LanguageRuRU Language = "ru-RU" // 俄语
	LanguageArSA Language = "ar-SA" // 阿拉伯语
	LanguageThTH Language = "th-TH" // 泰语
	LanguageViVN Language = "vi-VN" // 越南语
	LanguageIdID Language = "id-ID" // 印尼语
	LanguageMsMS Language = "ms-MS" // 马来语
)

// ================== 公共结构 ==================

// AudioInfo 音频信息
type AudioInfo struct {
	// Duration 音频时长（毫秒）
	Duration int `json:"duration"`

	// SampleRate 采样率
	SampleRate int `json:"sample_rate,omitempty"`

	// Channels 声道数
	Channels int `json:"channels,omitempty"`

	// Bits 位深度
	Bits int `json:"bits,omitempty"`
}

// SubtitleSegment 字幕片段
type SubtitleSegment struct {
	// Text 字幕文本
	Text string `json:"text"`

	// StartTime 开始时间（毫秒）
	StartTime int `json:"start_time"`

	// EndTime 结束时间（毫秒）
	EndTime int `json:"end_time"`
}

// LocationInfo 位置信息（用于实时对话）
type LocationInfo struct {
	// Longitude 经度
	Longitude float64 `json:"longitude,omitempty"`

	// Latitude 纬度
	Latitude float64 `json:"latitude,omitempty"`

	// City 城市
	City string `json:"city,omitempty"`

	// Country 国家
	Country string `json:"country,omitempty"`

	// Province 省份
	Province string `json:"province,omitempty"`

	// District 区县
	District string `json:"district,omitempty"`

	// Town 乡镇
	Town string `json:"town,omitempty"`

	// CountryCode 国家代码
	CountryCode string `json:"country_code,omitempty"`

	// Address 详细地址
	Address string `json:"address,omitempty"`
}
