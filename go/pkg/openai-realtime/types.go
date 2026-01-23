package openairealtime

import "encoding/json"

// Models supported by OpenAI Realtime API.
const (
	// ModelGPT4oRealtimePreview is the GPT-4o realtime preview model.
	ModelGPT4oRealtimePreview = "gpt-4o-realtime-preview"
	// ModelGPT4oRealtimePreview20241217 is a specific version.
	ModelGPT4oRealtimePreview20241217 = "gpt-4o-realtime-preview-2024-12-17"
	// ModelGPT4oMiniRealtimePreview is the GPT-4o mini realtime preview model.
	ModelGPT4oMiniRealtimePreview = "gpt-4o-mini-realtime-preview"
	// ModelGPT4oMiniRealtimePreview20241217 is a specific version.
	ModelGPT4oMiniRealtimePreview20241217 = "gpt-4o-mini-realtime-preview-2024-12-17"
)

// Audio formats supported by the Realtime API.
const (
	// AudioFormatPCM16 is 16-bit PCM audio at 24kHz, mono, little-endian.
	AudioFormatPCM16 = "pcm16"
	// AudioFormatG711ULaw is G.711 μ-law audio at 8kHz.
	AudioFormatG711ULaw = "g711_ulaw"
	// AudioFormatG711ALaw is G.711 A-law audio at 8kHz.
	AudioFormatG711ALaw = "g711_alaw"
)

// Voice options for audio output.
const (
	VoiceAlloy   = "alloy"
	VoiceAsh     = "ash"
	VoiceBallad  = "ballad"
	VoiceCoral   = "coral"
	VoiceEcho    = "echo"
	VoiceSage    = "sage"
	VoiceShimmer = "shimmer"
	VoiceVerse   = "verse"
)

// VAD modes for turn detection.
const (
	// VADServerVAD enables server-side voice activity detection.
	VADServerVAD = "server_vad"
	// VADSemanticVAD enables semantic voice activity detection.
	VADSemanticVAD = "semantic_vad"
)

// Modality types.
const (
	ModalityText  = "text"
	ModalityAudio = "audio"
)

// Tool choice options.
const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
)

// ConnectConfig contains configuration for establishing a realtime connection.
type ConnectConfig struct {
	// Model is the model ID to use.
	// Default: gpt-4o-realtime-preview
	Model string `json:"model,omitempty"`
}

// SessionConfig contains configuration for updating session parameters.
type SessionConfig struct {
	// Modalities specifies the output modalities.
	// Default: ["text", "audio"]
	Modalities []string `json:"modalities,omitempty"`

	// Instructions is the system prompt.
	Instructions string `json:"instructions,omitempty"`

	// Voice is the voice ID for audio output.
	Voice string `json:"voice,omitempty"`

	// InputAudioFormat specifies the input audio format.
	// Default: pcm16
	InputAudioFormat string `json:"input_audio_format,omitempty"`

	// OutputAudioFormat specifies the output audio format.
	// Default: pcm16
	OutputAudioFormat string `json:"output_audio_format,omitempty"`

	// InputAudioTranscription configures input audio transcription.
	// Set to enable transcription of user audio.
	InputAudioTranscription *TranscriptionConfig `json:"input_audio_transcription,omitempty"`

	// TurnDetection configures voice activity detection.
	// Use TurnDetectionOff() to explicitly disable VAD (manual mode).
	// Use nil to keep current setting.
	TurnDetection *TurnDetection `json:"turn_detection,omitempty"`

	// TurnDetectionDisabled when true, sends "turn_detection": null explicitly.
	// This is needed because omitempty won't send null values.
	TurnDetectionDisabled bool `json:"-"`

	// Tools defines the available functions for the model.
	Tools []Tool `json:"tools,omitempty"`

	// ToolChoice specifies how the model should use tools.
	// Options: "auto", "none", "required", or specific function.
	ToolChoice interface{} `json:"tool_choice,omitempty"`

	// Temperature controls randomness (0.6-1.2).
	// Default: 0.8
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxResponseOutputTokens limits the output length.
	// Use "inf" for unlimited.
	MaxResponseOutputTokens interface{} `json:"max_response_output_tokens,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for SessionConfig.
// This is needed to send "turn_detection": null when TurnDetectionDisabled is true.
func (s SessionConfig) MarshalJSON() ([]byte, error) {
	type Alias SessionConfig
	aux := &struct {
		TurnDetection interface{} `json:"turn_detection,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(&s),
	}

	if s.TurnDetectionDisabled {
		// Create a map and set turn_detection to null explicitly
		m := make(map[string]interface{})
		if len(s.Modalities) > 0 {
			m["modalities"] = s.Modalities
		}
		if s.Instructions != "" {
			m["instructions"] = s.Instructions
		}
		if s.Voice != "" {
			m["voice"] = s.Voice
		}
		if s.InputAudioFormat != "" {
			m["input_audio_format"] = s.InputAudioFormat
		}
		if s.OutputAudioFormat != "" {
			m["output_audio_format"] = s.OutputAudioFormat
		}
		if s.InputAudioTranscription != nil {
			m["input_audio_transcription"] = s.InputAudioTranscription
		}
		m["turn_detection"] = nil // Explicit null
		if len(s.Tools) > 0 {
			m["tools"] = s.Tools
		}
		if s.ToolChoice != nil {
			m["tool_choice"] = s.ToolChoice
		}
		if s.Temperature != nil {
			m["temperature"] = s.Temperature
		}
		if s.MaxResponseOutputTokens != nil {
			m["max_response_output_tokens"] = s.MaxResponseOutputTokens
		}
		return json.Marshal(m)
	}

	if s.TurnDetection != nil {
		aux.TurnDetection = s.TurnDetection
	}
	return json.Marshal(aux)
}

// TranscriptionConfig configures input audio transcription.
type TranscriptionConfig struct {
	// Model is the transcription model to use.
	// Default: whisper-1
	Model string `json:"model,omitempty"`
}

// TurnDetection configures voice activity detection.
type TurnDetection struct {
	// Type is the VAD mode: "server_vad" or "semantic_vad".
	Type string `json:"type,omitempty"`

	// Threshold is the VAD sensitivity (0.0-1.0).
	// Default: 0.5
	Threshold float64 `json:"threshold,omitempty"`

	// PrefixPaddingMs is the padding before speech start (ms).
	// Default: 300
	PrefixPaddingMs int `json:"prefix_padding_ms,omitempty"`

	// SilenceDurationMs is the silence duration to detect end of speech (ms).
	// Default: 500
	SilenceDurationMs int `json:"silence_duration_ms,omitempty"`

	// CreateResponse specifies whether to automatically create a response
	// when VAD detects end of speech.
	// Default: true
	CreateResponse *bool `json:"create_response,omitempty"`

	// InterruptResponse specifies whether to interrupt the current response
	// when the user starts speaking.
	// Default: true
	InterruptResponse *bool `json:"interrupt_response,omitempty"`

	// Eagerness controls how eagerly the model responds (semantic_vad only).
	// Higher eagerness means faster responses but may interrupt the user.
	// Values: "low", "medium", "high". Default: "medium"
	Eagerness string `json:"eagerness,omitempty"`
}

// Tool defines a function tool available to the model.
type Tool struct {
	// Type is always "function".
	Type string `json:"type"`

	// Name is the function name.
	Name string `json:"name"`

	// Description describes what the function does.
	Description string `json:"description,omitempty"`

	// Parameters is the JSON Schema for the function parameters.
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ResponseCreateOptions contains options for creating a response.
type ResponseCreateOptions struct {
	// Modalities specifies the output modalities for this response.
	Modalities []string `json:"modalities,omitempty"`

	// Instructions override for this response.
	Instructions string `json:"instructions,omitempty"`

	// Voice override for this response.
	Voice string `json:"voice,omitempty"`

	// OutputAudioFormat override for this response.
	OutputAudioFormat string `json:"output_audio_format,omitempty"`

	// Tools override for this response.
	Tools []Tool `json:"tools,omitempty"`

	// ToolChoice override for this response.
	ToolChoice interface{} `json:"tool_choice,omitempty"`

	// Temperature override for this response.
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxOutputTokens limits the output length for this response.
	MaxOutputTokens interface{} `json:"max_output_tokens,omitempty"`

	// Conversation specifies conversation handling.
	// "auto" (default) uses existing conversation.
	// "none" creates response without conversation context.
	Conversation string `json:"conversation,omitempty"`

	// Input provides input items directly instead of using the buffer.
	// Use this for text-only input or to inject conversation history.
	Input []ConversationItem `json:"input,omitempty"`
}

// SessionResource represents the session state returned by the server.
type SessionResource struct {
	ID                        string               `json:"id,omitempty"`
	Object                    string               `json:"object,omitempty"`
	Model                     string               `json:"model,omitempty"`
	ExpiresAt                 int64                `json:"expires_at,omitempty"`
	Modalities                []string             `json:"modalities,omitempty"`
	Instructions              string               `json:"instructions,omitempty"`
	Voice                     string               `json:"voice,omitempty"`
	InputAudioFormat          string               `json:"input_audio_format,omitempty"`
	OutputAudioFormat         string               `json:"output_audio_format,omitempty"`
	InputAudioTranscription   *TranscriptionConfig `json:"input_audio_transcription,omitempty"`
	TurnDetection             *TurnDetection       `json:"turn_detection,omitempty"`
	Tools                     []Tool               `json:"tools,omitempty"`
	ToolChoice                interface{}          `json:"tool_choice,omitempty"`
	Temperature               float64              `json:"temperature,omitempty"`
	MaxResponseOutputTokens   interface{}          `json:"max_response_output_tokens,omitempty"`
}

// ConversationResource represents a conversation.
type ConversationResource struct {
	ID     string `json:"id,omitempty"`
	Object string `json:"object,omitempty"`
}

// ConversationItem represents an item in the conversation.
type ConversationItem struct {
	ID       string        `json:"id,omitempty"`
	Object   string        `json:"object,omitempty"`
	Type     string        `json:"type,omitempty"` // "message", "function_call", "function_call_output"
	Status   string        `json:"status,omitempty"`
	Role     string        `json:"role,omitempty"` // "user", "assistant", "system"
	Content  []ContentPart `json:"content,omitempty"`
	CallID   string        `json:"call_id,omitempty"`   // for function_call_output
	Name     string        `json:"name,omitempty"`      // for function_call
	Arguments string       `json:"arguments,omitempty"` // for function_call
	Output   string        `json:"output,omitempty"`    // for function_call_output
}

// ContentPart represents a part of message content.
type ContentPart struct {
	Type       string `json:"type,omitempty"` // "input_text", "input_audio", "item_reference", "text", "audio"
	Text       string `json:"text,omitempty"`
	Audio      string `json:"audio,omitempty"`      // base64 encoded
	Transcript string `json:"transcript,omitempty"` // for audio parts
	ID         string `json:"id,omitempty"`         // for item_reference
}

// ResponseResource represents a response from the model.
type ResponseResource struct {
	ID                 string             `json:"id,omitempty"`
	Object             string             `json:"object,omitempty"`
	Status             string             `json:"status,omitempty"` // "in_progress", "completed", "cancelled", "incomplete", "failed"
	StatusDetails      *StatusDetails     `json:"status_details,omitempty"`
	Output             []ConversationItem `json:"output,omitempty"`
	Usage              *Usage             `json:"usage,omitempty"`
}

// StatusDetails contains details about the response status.
type StatusDetails struct {
	Type   string `json:"type,omitempty"`
	Reason string `json:"reason,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

// Usage contains token usage information.
type Usage struct {
	TotalTokens              int          `json:"total_tokens,omitempty"`
	InputTokens              int          `json:"input_tokens,omitempty"`
	OutputTokens             int          `json:"output_tokens,omitempty"`
	InputTokenDetails        *TokenDetails `json:"input_token_details,omitempty"`
	OutputTokenDetails       *TokenDetails `json:"output_token_details,omitempty"`
}

// TokenDetails contains detailed token breakdown.
type TokenDetails struct {
	CachedTokens     int `json:"cached_tokens,omitempty"`
	TextTokens       int `json:"text_tokens,omitempty"`
	AudioTokens      int `json:"audio_tokens,omitempty"`
	CachedTokensDetails *CachedTokensDetails `json:"cached_tokens_details,omitempty"`
}

// CachedTokensDetails contains details about cached tokens.
type CachedTokensDetails struct {
	TextTokens  int `json:"text_tokens,omitempty"`
	AudioTokens int `json:"audio_tokens,omitempty"`
}
