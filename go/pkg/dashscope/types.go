package dashscope

// Common models for Qwen-Omni-Realtime.
const (
	// ModelQwenOmniTurboRealtime is the Qwen-Omni-Turbo model for Realtime API.
	ModelQwenOmniTurboRealtime = "qwen-omni-turbo-realtime"
	// ModelQwenOmniTurboRealtimeLatest is the latest version of Qwen-Omni-Turbo for Realtime API.
	ModelQwenOmniTurboRealtimeLatest = "qwen-omni-turbo-realtime-latest"
)

// Audio formats supported by DashScope.
const (
	AudioFormatPCM16 = "pcm16" // 16-bit PCM, 24kHz, mono (for input and Turbo output)
	AudioFormatPCM24 = "pcm24" // 24-bit PCM (for Flash output)
	AudioFormatWAV   = "wav"   // WAV format
	AudioFormatMP3   = "mp3"   // MP3 format
)

// Voice IDs for TTS output.
const (
	VoiceChelsie = "Chelsie" // Default voice
	VoiceCherry  = "Cherry"
	VoiceSerena  = "Serena"
	VoiceEthan   = "Ethan"
)

// VAD modes for voice activity detection.
const (
	VADModeServerVAD = "server_vad" // Server-side VAD (default)
	VADModeDisabled  = "disabled"   // Manual mode, no VAD
)

// Modalities for output.
const (
	ModalityText  = "text"
	ModalityAudio = "audio"
)

// RealtimeConfig is the configuration for establishing a realtime session.
type RealtimeConfig struct {
	// Model is the model ID to use.
	// Default: qwen-omni-turbo-latest
	Model string `json:"model,omitempty"`
}

// SessionConfig is the configuration for updating session parameters.
type SessionConfig struct {
	// TurnDetection configures voice activity detection.
	TurnDetection *TurnDetection `json:"turn_detection,omitempty"`

	// InputAudioFormat specifies the input audio format.
	// Default: pcm16 (16-bit, 16kHz, mono)
	InputAudioFormat string `json:"input_audio_format,omitempty"`

	// OutputAudioFormat specifies the output audio format.
	// Default: pcm16 (16-bit, 24kHz, mono)
	OutputAudioFormat string `json:"output_audio_format,omitempty"`

	// Voice is the voice ID for TTS output.
	Voice string `json:"voice,omitempty"`

	// Modalities specifies the output modalities.
	// Default: ["text", "audio"]
	Modalities []string `json:"modalities,omitempty"`

	// Instructions is the system prompt.
	Instructions string `json:"instructions,omitempty"`

	// Temperature controls randomness (0.0-2.0).
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxOutputTokens limits the output length.
	// Use -1 for unlimited (default).
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`

	// EnableInputAudioTranscription enables transcription of input audio.
	EnableInputAudioTranscription bool `json:"enable_input_audio_transcription,omitempty"`

	// InputAudioTranscriptionModel specifies the model for input transcription.
	InputAudioTranscriptionModel string `json:"input_audio_transcription_model,omitempty"`
}

// TurnDetection configures voice activity detection.
type TurnDetection struct {
	// Type is the VAD mode: "server_vad" or "disabled".
	Type string `json:"type,omitempty"`

	// PrefixPaddingMs is the padding before speech start (ms).
	// Default: 300
	PrefixPaddingMs int `json:"prefix_padding_ms,omitempty"`

	// SilenceDurationMs is the silence duration to detect end of speech (ms).
	// Default: 500
	SilenceDurationMs int `json:"silence_duration_ms,omitempty"`

	// Threshold is the VAD sensitivity (0.0-1.0).
	// Default: 0.5
	Threshold float64 `json:"threshold,omitempty"`
}

// SessionInfo contains session state information.
type SessionInfo struct {
	ID                string         `json:"id,omitempty"`
	Model             string         `json:"model,omitempty"`
	Modalities        []string       `json:"modalities,omitempty"`
	Voice             string         `json:"voice,omitempty"`
	InputAudioFormat  string         `json:"input_audio_format,omitempty"`
	OutputAudioFormat string         `json:"output_audio_format,omitempty"`
	TurnDetection     *TurnDetection `json:"turn_detection,omitempty"`
	Instructions      string         `json:"instructions,omitempty"`
	Temperature       float64        `json:"temperature,omitempty"`
	MaxOutputTokens   interface{}    `json:"max_output_tokens,omitempty"` // int or "inf"
}

// TranscriptItem represents a transcript segment.
type TranscriptItem struct {
	// ItemID is the unique identifier for this item.
	ItemID string `json:"item_id,omitempty"`

	// OutputIndex is the index in the output array.
	OutputIndex int `json:"output_index,omitempty"`

	// ContentIndex is the index within the content array.
	ContentIndex int `json:"content_index,omitempty"`

	// Transcript is the text content.
	Transcript string `json:"transcript,omitempty"`
}

// UsageStats contains token usage information.
type UsageStats struct {
	// TotalTokens is the total number of tokens.
	TotalTokens int `json:"total_tokens,omitempty"`

	// InputTokens is the number of input tokens.
	InputTokens int `json:"input_tokens,omitempty"`

	// OutputTokens is the number of output tokens.
	OutputTokens int `json:"output_tokens,omitempty"`

	// InputTokenDetails contains detailed input token breakdown.
	InputTokenDetails *TokenDetails `json:"input_token_details,omitempty"`

	// OutputTokenDetails contains detailed output token breakdown.
	OutputTokenDetails *TokenDetails `json:"output_token_details,omitempty"`
}

// TokenDetails contains detailed token breakdown.
type TokenDetails struct {
	TextTokens  int `json:"text_tokens,omitempty"`
	AudioTokens int `json:"audio_tokens,omitempty"`
	ImageTokens int `json:"image_tokens,omitempty"`
}
