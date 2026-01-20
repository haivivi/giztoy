package doubaospeech

import (
	"io"
)

// ================== Audio Encoding ==================

// AudioEncoding represents audio encoding format (TTS output)
type AudioEncoding string

const (
	EncodingPCM      AudioEncoding = "pcm"
	EncodingWAV      AudioEncoding = "wav"
	EncodingMP3      AudioEncoding = "mp3"
	EncodingOGG      AudioEncoding = "ogg_opus"
	EncodingAAC      AudioEncoding = "aac"
	EncodingM4A      AudioEncoding = "m4a"
	EncodingPCMS16LE AudioEncoding = "pcm_s16le" // For realtime, s16le format
	EncodingPCMF32LE AudioEncoding = "pcm"       // For realtime, f32le format
)

// AudioFormat represents audio format (ASR input)
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

// ================== Sample Rate ==================

// SampleRate represents audio sample rate
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

// ================== Language ==================

// Language represents language code
type Language string

const (
	LanguageZhCN Language = "zh-CN" // Chinese (Mandarin)
	LanguageEnUS Language = "en-US" // English (US)
	LanguageEnGB Language = "en-GB" // English (UK)
	LanguageJaJP Language = "ja-JP" // Japanese
	LanguageKoKR Language = "ko-KR" // Korean
	LanguageEsES Language = "es-ES" // Spanish
	LanguageFrFR Language = "fr-FR" // French
	LanguageDeDE Language = "de-DE" // German
	LanguageItIT Language = "it-IT" // Italian
	LanguagePtBR Language = "pt-BR" // Portuguese (Brazil)
	LanguageRuRU Language = "ru-RU" // Russian
	LanguageArSA Language = "ar-SA" // Arabic
	LanguageThTH Language = "th-TH" // Thai
	LanguageViVN Language = "vi-VN" // Vietnamese
	LanguageIdID Language = "id-ID" // Indonesian
	LanguageMsMS Language = "ms-MS" // Malay
)

// ================== Common Structures ==================

// AudioInfo represents audio information
type AudioInfo struct {
	Duration   int `json:"duration"`              // Duration in milliseconds
	SampleRate int `json:"sample_rate,omitempty"` // Sample rate
	Channels   int `json:"channels,omitempty"`    // Number of channels
	Bits       int `json:"bits,omitempty"`        // Bit depth
}

// SubtitleSegment represents a subtitle segment
type SubtitleSegment struct {
	Text      string `json:"text"`       // Subtitle text
	StartTime int    `json:"start_time"` // Start time in milliseconds
	EndTime   int    `json:"end_time"`   // End time in milliseconds
}

// LocationInfo represents location information (for realtime conversation)
type LocationInfo struct {
	Longitude   float64 `json:"longitude,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	City        string  `json:"city,omitempty"`
	Country     string  `json:"country,omitempty"`
	Province    string  `json:"province,omitempty"`
	District    string  `json:"district,omitempty"`
	Town        string  `json:"town,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	Address     string  `json:"address,omitempty"`
}

// ================== Task ==================

// TaskStatus represents task status
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusSuccess    TaskStatus = "success"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// Task represents an async task
type Task[T any] struct {
	ID string
}

// Note: Error type is defined in error.go

// ================== TTS Types ==================

// TTSTextType represents text type
type TTSTextType string

const (
	TTSTextTypePlain TTSTextType = "plain" // Plain text
	TTSTextTypeSSML  TTSTextType = "ssml"  // SSML format
)

// TTSRequest represents TTS synthesis request
type TTSRequest struct {
	Text            string        `json:"text" yaml:"text"`
	TextType        TTSTextType   `json:"text_type,omitempty" yaml:"text_type,omitempty"`
	VoiceType       string        `json:"voice_type" yaml:"voice_type"`
	Cluster         string        `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	Encoding        AudioEncoding `json:"encoding,omitempty" yaml:"encoding,omitempty"`
	SampleRate      SampleRate    `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`
	SpeedRatio      float64       `json:"speed_ratio,omitempty" yaml:"speed_ratio,omitempty"`
	VolumeRatio     float64       `json:"volume_ratio,omitempty" yaml:"volume_ratio,omitempty"`
	PitchRatio      float64       `json:"pitch_ratio,omitempty" yaml:"pitch_ratio,omitempty"`
	Emotion         string        `json:"emotion,omitempty" yaml:"emotion,omitempty"`
	Language        Language      `json:"language,omitempty" yaml:"language,omitempty"`
	EnableSubtitle  bool          `json:"enable_subtitle,omitempty" yaml:"enable_subtitle,omitempty"`
	SilenceDuration int           `json:"silence_duration,omitempty" yaml:"silence_duration,omitempty"`
}

// TTSResponse represents TTS synthesis response
type TTSResponse struct {
	Audio     []byte            `json:"-"`
	Duration  int               `json:"duration"`
	Subtitles []SubtitleSegment `json:"subtitles,omitempty"`
	ReqID     string            `json:"reqid"`
}

// ToReader converts audio data to io.Reader
func (r *TTSResponse) ToReader() io.Reader {
	return nil // Implementation in tts.go
}

// TTSChunk represents streaming TTS chunk
type TTSChunk struct {
	Audio    []byte           `json:"-"`
	Sequence int32            `json:"sequence"`
	IsLast   bool             `json:"is_last"`
	Subtitle *SubtitleSegment `json:"subtitle,omitempty"`
	Duration int              `json:"duration,omitempty"`
}

// TTSDuplexConfig represents duplex session config
type TTSDuplexConfig struct {
	VoiceType   string        `json:"voice_type"`
	Encoding    AudioEncoding `json:"encoding,omitempty"`
	SampleRate  SampleRate    `json:"sample_rate,omitempty"`
	SpeedRatio  float64       `json:"speed_ratio,omitempty"`
	VolumeRatio float64       `json:"volume_ratio,omitempty"`
	PitchRatio  float64       `json:"pitch_ratio,omitempty"`
}

// AsyncTTSRequest represents async TTS request
type AsyncTTSRequest struct {
	Text        string        `json:"text"`
	TextType    TTSTextType   `json:"text_type,omitempty"`
	VoiceType   string        `json:"voice_type"`
	Encoding    AudioEncoding `json:"encoding,omitempty"`
	SampleRate  SampleRate    `json:"sample_rate,omitempty"`
	SpeedRatio  float64       `json:"speed_ratio,omitempty"`
	VolumeRatio float64       `json:"volume_ratio,omitempty"`
	PitchRatio  float64       `json:"pitch_ratio,omitempty"`
	CallbackURL string        `json:"callback_url,omitempty"`
}

// TTSAsyncResult represents async TTS result
type TTSAsyncResult struct {
	AudioURL  string            `json:"audio_url"`
	Duration  int               `json:"duration"`
	Subtitles []SubtitleSegment `json:"subtitles,omitempty"`
}

// ================== ASR Types ==================

// OneSentenceRequest represents one-sentence ASR request
type OneSentenceRequest struct {
	Audio       []byte      `json:"-"`
	AudioURL    string      `json:"audio_url,omitempty"`
	AudioReader io.Reader   `json:"-"`
	Format      AudioFormat `json:"format"`
	SampleRate  SampleRate  `json:"sample_rate,omitempty"`
	Language    Language    `json:"language,omitempty"`
	EnableITN   bool        `json:"enable_itn,omitempty"`
	EnablePunc  bool        `json:"enable_punc,omitempty"`
	EnableDDC   bool        `json:"enable_ddc,omitempty"`
}

// ASRResult represents ASR result
type ASRResult struct {
	Text       string      `json:"text"`
	Duration   int         `json:"duration"`
	Utterances []Utterance `json:"utterances,omitempty"`
}

// Utterance represents sentence segment
type Utterance struct {
	Text      string `json:"text"`
	StartTime int    `json:"start_time"`
	EndTime   int    `json:"end_time"`
	Definite  bool   `json:"definite"`
	Words     []Word `json:"words,omitempty"`
}

// Word represents word information
type Word struct {
	Text      string `json:"text"`
	StartTime int    `json:"start_time"`
	EndTime   int    `json:"end_time"`
}

// StreamASRConfig represents streaming ASR config
type StreamASRConfig struct {
	Format          AudioFormat `json:"format"`
	SampleRate      SampleRate  `json:"sample_rate"`
	Bits            int         `json:"bits"`
	Channel         int         `json:"channel"`
	Language        Language    `json:"language,omitempty"`
	ModelName       string      `json:"model_name,omitempty"`
	EnableITN       bool        `json:"enable_itn,omitempty"`
	EnablePunc      bool        `json:"enable_punc,omitempty"`
	EnableDDC       bool        `json:"enable_ddc,omitempty"`
	ShowUtterances  bool        `json:"show_utterances,omitempty"`
	EnableNonstream bool        `json:"enable_nonstream,omitempty"`
}

// ASRChunk represents streaming ASR chunk
type ASRChunk struct {
	Text       string      `json:"text"`
	IsDefinite bool        `json:"is_definite"`
	IsFinal    bool        `json:"is_final"`
	Utterances []Utterance `json:"utterances,omitempty"`
	AudioInfo  *AudioInfo  `json:"audio_info,omitempty"`
	Sequence   int32       `json:"sequence"`
}

// FileASRRequest represents file ASR request
type FileASRRequest struct {
	AudioURL        string      `json:"audio_url"`
	Format          AudioFormat `json:"format,omitempty"`
	Language        Language    `json:"language,omitempty"`
	EnableITN       bool        `json:"enable_itn,omitempty"`
	EnablePunc      bool        `json:"enable_punc,omitempty"`
	EnableDDC       bool        `json:"enable_ddc,omitempty"`
	EnableTimestamp bool        `json:"enable_timestamp,omitempty"`
	CallbackURL     string      `json:"callback_url,omitempty"`
}

// ================== Voice Clone Types ==================

// VoiceCloneModelType represents voice clone model type
type VoiceCloneModelType string

const (
	VoiceCloneModelStandard VoiceCloneModelType = "standard"
	VoiceCloneModelPro      VoiceCloneModelType = "pro"
)

// VoiceCloneStatusType represents voice clone status
type VoiceCloneStatusType string

const (
	VoiceCloneStatusPending    VoiceCloneStatusType = "pending"
	VoiceCloneStatusProcessing VoiceCloneStatusType = "processing"
	VoiceCloneStatusSuccess    VoiceCloneStatusType = "success"
	VoiceCloneStatusFailed     VoiceCloneStatusType = "failed"
)

// VoiceCloneTrainRequest represents voice clone training request
type VoiceCloneTrainRequest struct {
	SpeakerID string              `json:"speaker_id"`
	AudioURLs []string            `json:"audio_urls,omitempty"`
	AudioData [][]byte            `json:"-"`
	Text      string              `json:"text,omitempty"`
	Language  Language            `json:"language"`
	ModelType VoiceCloneModelType `json:"model_type,omitempty"`
}

// VoiceCloneResult represents voice clone result
type VoiceCloneResult struct {
	SpeakerID string               `json:"speaker_id"`
	Status    VoiceCloneStatusType `json:"status"`
	Message   string               `json:"message,omitempty"`
}

// VoiceCloneStatus represents voice clone training status
type VoiceCloneStatus struct {
	SpeakerID string               `json:"speaker_id"`
	Status    VoiceCloneStatusType `json:"status"`
	Progress  int                  `json:"progress,omitempty"`
	Message   string               `json:"message,omitempty"`
	DemoAudio string               `json:"demo_audio,omitempty"`
	CreatedAt int64                `json:"created_at"`
	UpdatedAt int64                `json:"updated_at"`
}

// VoiceCloneInfo represents trained voice info
type VoiceCloneInfo struct {
	SpeakerID string               `json:"speaker_id"`
	Status    VoiceCloneStatusType `json:"status"`
	Language  Language             `json:"language"`
	ModelType VoiceCloneModelType  `json:"model_type"`
	CreatedAt int64                `json:"created_at"`
}

// ================== Meeting Types ==================

// MeetingTaskRequest represents meeting transcription request
type MeetingTaskRequest struct {
	AudioURL                 string      `json:"audio_url"`
	Format                   AudioFormat `json:"format,omitempty"`
	Language                 Language    `json:"language,omitempty"`
	SpeakerCount             int         `json:"speaker_count,omitempty"`
	EnableSpeakerDiarization bool        `json:"enable_speaker_diarization,omitempty"`
	EnableTimestamp          bool        `json:"enable_timestamp,omitempty"`
	CallbackURL              string      `json:"callback_url,omitempty"`
}

// MeetingResult represents meeting transcription result
type MeetingResult struct {
	Text     string           `json:"text"`
	Duration int              `json:"duration"`
	Segments []MeetingSegment `json:"segments,omitempty"`
}

// MeetingSegment represents meeting segment
type MeetingSegment struct {
	Text      string `json:"text"`
	StartTime int    `json:"start_time"`
	EndTime   int    `json:"end_time"`
	SpeakerID string `json:"speaker_id,omitempty"`
}

// MeetingTaskStatus represents meeting task status
type MeetingTaskStatus struct {
	TaskID   string         `json:"task_id"`
	Status   TaskStatus     `json:"status"`
	Progress int            `json:"progress,omitempty"`
	Result   *MeetingResult `json:"result,omitempty"`
	Error    *Error         `json:"error,omitempty"`
}

// ================== Podcast Types ==================

// PodcastTaskRequest represents podcast synthesis request
type PodcastTaskRequest struct {
	Script      []PodcastLine `json:"script"`
	Encoding    AudioEncoding `json:"encoding,omitempty"`
	SampleRate  SampleRate    `json:"sample_rate,omitempty"`
	CallbackURL string        `json:"callback_url,omitempty"`
}

// PodcastLine represents podcast line
type PodcastLine struct {
	SpeakerID  string  `json:"speaker_id"`
	Text       string  `json:"text"`
	Emotion    string  `json:"emotion,omitempty"`
	SpeedRatio float64 `json:"speed_ratio,omitempty"`
}

// PodcastResult represents podcast result
type PodcastResult struct {
	AudioURL  string            `json:"audio_url"`
	Duration  int               `json:"duration"`
	Subtitles []SubtitleSegment `json:"subtitles,omitempty"`
}

// PodcastTaskStatus represents podcast task status
type PodcastTaskStatus struct {
	TaskID   string         `json:"task_id"`
	Status   TaskStatus     `json:"status"`
	Progress int            `json:"progress,omitempty"`
	Result   *PodcastResult `json:"result,omitempty"`
	Error    *Error         `json:"error,omitempty"`
}

// ================== Media Types ==================

// SubtitleFormat represents subtitle format
type SubtitleFormat string

const (
	SubtitleFormatSRT  SubtitleFormat = "srt"
	SubtitleFormatVTT  SubtitleFormat = "vtt"
	SubtitleFormatJSON SubtitleFormat = "json"
)

// SubtitleRequest represents subtitle extraction request
type SubtitleRequest struct {
	MediaURL          string         `json:"media_url"`
	Language          Language       `json:"language,omitempty"`
	Format            SubtitleFormat `json:"format,omitempty"`
	EnableTranslation bool           `json:"enable_translation,omitempty"`
	TargetLanguage    Language       `json:"target_language,omitempty"`
	CallbackURL       string         `json:"callback_url,omitempty"`
}

// SubtitleResult represents subtitle extraction result
type SubtitleResult struct {
	SubtitleURL string            `json:"subtitle_url"`
	Subtitles   []SubtitleSegment `json:"subtitles,omitempty"`
	Duration    int               `json:"duration"`
}

// SubtitleTaskStatus represents subtitle task status
type SubtitleTaskStatus struct {
	TaskID   string          `json:"task_id"`
	Status   TaskStatus      `json:"status"`
	Progress int             `json:"progress,omitempty"`
	Result   *SubtitleResult `json:"result,omitempty"`
	Error    *Error          `json:"error,omitempty"`
}

// ================== Translation Types ==================

// TranslationConfig represents translation config
type TranslationConfig struct {
	SourceLanguage Language        `json:"source_language"`
	TargetLanguage Language        `json:"target_language"`
	AudioConfig    StreamASRConfig `json:"audio_config"`
	EnableTTS      bool            `json:"enable_tts,omitempty"`
	TTSVoice       string          `json:"tts_voice,omitempty"`
}

// TranslationSession represents translation session
// TranslationChunk represents translation chunk
type TranslationChunk struct {
	SourceText string `json:"source_text"`
	TargetText string `json:"target_text"`
	Audio      []byte `json:"audio,omitempty"`
	IsDefinite bool   `json:"is_definite"`
	IsFinal    bool   `json:"is_final"`
	Sequence   int32  `json:"sequence"`
}

