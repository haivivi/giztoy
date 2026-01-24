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
	Model string `json:"model,omitzero"`

	// Voice is the voice ID for audio output (WebRTC only).
	// Used when creating the ephemeral token.
	// Default: alloy
	Voice string `json:"voice,omitzero"`
}

// SessionConfig contains configuration for updating session parameters.
type SessionConfig struct {
	// Modalities specifies the output modalities.
	// Default: ["text", "audio"]
	Modalities []string `json:"modalities,omitzero"`

	// Instructions is the system prompt.
	Instructions string `json:"instructions,omitzero"`

	// Voice is the voice ID for audio output.
	Voice string `json:"voice,omitzero"`

	// InputAudioFormat specifies the input audio format.
	// Default: pcm16
	InputAudioFormat string `json:"input_audio_format,omitzero"`

	// OutputAudioFormat specifies the output audio format.
	// Default: pcm16
	OutputAudioFormat string `json:"output_audio_format,omitzero"`

	// InputAudioTranscription configures input audio transcription.
	// Set to enable transcription of user audio.
	InputAudioTranscription *TranscriptionConfig `json:"input_audio_transcription,omitzero"`

	// TurnDetection configures voice activity detection.
	// Set TurnDetectionDisabled=true to explicitly disable VAD (manual mode).
	// Use nil to keep current setting.
	TurnDetection *TurnDetection `json:"turn_detection,omitzero"`

	// TurnDetectionDisabled when true, sends "turn_detection": null explicitly.
	// This disables server-side VAD and enables manual mode.
	TurnDetectionDisabled bool `json:"-"`

	// Tools defines the available functions for the model.
	Tools []Tool `json:"tools,omitzero"`

	// ToolChoice specifies how the model should use tools.
	// Can be a string ("auto", "none", "required") or an object:
	//   {"type": "function", "function": {"name": "my_function"}}
	// Use interface{} to support both string and object forms.
	ToolChoice interface{} `json:"tool_choice,omitzero"`

	// Temperature controls randomness (0.6-1.2).
	// Default: 0.8
	Temperature *float64 `json:"temperature,omitzero"`

	// MaxResponseOutputTokens limits the output length.
	MaxResponseOutputTokens *int `json:"max_response_output_tokens,omitzero"`
}

// MarshalJSON implements custom JSON marshaling for SessionConfig.
// This is needed to send "turn_detection": null when TurnDetectionDisabled is true.
func (s SessionConfig) MarshalJSON() ([]byte, error) {
	type Alias SessionConfig
	aux := &struct {
		TurnDetection interface{} `json:"turn_detection,omitzero"`
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
	Model string `json:"model,omitzero"`
}

// TurnDetection configures voice activity detection.
type TurnDetection struct {
	// Type is the VAD mode: "server_vad" or "semantic_vad".
	Type string `json:"type,omitzero"`

	// Threshold is the VAD sensitivity (0.0-1.0).
	// Default: 0.5
	Threshold float64 `json:"threshold,omitzero"`

	// PrefixPaddingMs is the padding before speech start (ms).
	// Default: 300
	PrefixPaddingMs int `json:"prefix_padding_ms,omitzero"`

	// SilenceDurationMs is the silence duration to detect end of speech (ms).
	// Default: 500
	SilenceDurationMs int `json:"silence_duration_ms,omitzero"`

	// CreateResponse specifies whether to automatically create a response
	// when VAD detects end of speech.
	// Default: true
	CreateResponse *bool `json:"create_response,omitzero"`

	// InterruptResponse specifies whether to interrupt the current response
	// when the user starts speaking.
	// Default: true
	InterruptResponse *bool `json:"interrupt_response,omitzero"`

	// Eagerness controls how eagerly the model responds (semantic_vad only).
	// Higher eagerness means faster responses but may interrupt the user.
	// Values: "low", "medium", "high". Default: "medium"
	Eagerness string `json:"eagerness,omitzero"`
}

// Tool defines a function tool available to the model.
type Tool struct {
	// Type is always "function".
	Type string `json:"type"`

	// Name is the function name.
	Name string `json:"name"`

	// Description describes what the function does.
	Description string `json:"description,omitzero"`

	// Parameters is the JSON Schema for the function parameters.
	Parameters map[string]interface{} `json:"parameters,omitzero"`
}

// ResponseCreateOptions contains options for creating a response.
type ResponseCreateOptions struct {
	// Modalities specifies the output modalities for this response.
	Modalities []string `json:"modalities,omitzero"`

	// Instructions override for this response.
	Instructions string `json:"instructions,omitzero"`

	// Voice override for this response.
	Voice string `json:"voice,omitzero"`

	// OutputAudioFormat override for this response.
	OutputAudioFormat string `json:"output_audio_format,omitzero"`

	// Tools override for this response.
	Tools []Tool `json:"tools,omitzero"`

	// ToolChoice override for this response.
	// Can be a string ("auto", "none", "required") or an object:
	//   {"type": "function", "function": {"name": "my_function"}}
	ToolChoice interface{} `json:"tool_choice,omitzero"`

	// Temperature override for this response.
	Temperature *float64 `json:"temperature,omitzero"`

	// MaxOutputTokens limits the output length for this response.
	MaxOutputTokens *int `json:"max_output_tokens,omitzero"`

	// Conversation specifies conversation handling.
	// "auto" (default) uses existing conversation.
	// "none" creates response without conversation context.
	Conversation string `json:"conversation,omitzero"`

	// Input provides input items directly instead of using the buffer.
	// Use this for text-only input or to inject conversation history.
	Input []ConversationItem `json:"input,omitzero"`
}

// SessionResource represents the session state returned by the server.
type SessionResource struct {
	ID                        string               `json:"id,omitzero"`
	Object                    string               `json:"object,omitzero"`
	Model                     string               `json:"model,omitzero"`
	ExpiresAt                 int64                `json:"expires_at,omitzero"`
	Modalities                []string             `json:"modalities,omitzero"`
	Instructions              string               `json:"instructions,omitzero"`
	Voice                     string               `json:"voice,omitzero"`
	InputAudioFormat          string               `json:"input_audio_format,omitzero"`
	OutputAudioFormat         string               `json:"output_audio_format,omitzero"`
	InputAudioTranscription   *TranscriptionConfig `json:"input_audio_transcription,omitzero"`
	TurnDetection             *TurnDetection       `json:"turn_detection,omitzero"`
	Tools                     []Tool               `json:"tools,omitzero"`
	ToolChoice                interface{}          `json:"tool_choice,omitzero"`
	Temperature               float64              `json:"temperature,omitzero"`
	MaxResponseOutputTokens   interface{}          `json:"max_response_output_tokens,omitzero"`
}

// ConversationResource represents a conversation.
type ConversationResource struct {
	ID     string `json:"id,omitzero"`
	Object string `json:"object,omitzero"`
}

// ConversationItem represents an item in the conversation.
type ConversationItem struct {
	ID       string        `json:"id,omitzero"`
	Object   string        `json:"object,omitzero"`
	Type     string        `json:"type,omitzero"` // "message", "function_call", "function_call_output"
	Status   string        `json:"status,omitzero"`
	Role     string        `json:"role,omitzero"` // "user", "assistant", "system"
	Content  []ContentPart `json:"content,omitzero"`
	CallID   string        `json:"call_id,omitzero"`   // for function_call_output
	Name     string        `json:"name,omitzero"`      // for function_call
	Arguments string       `json:"arguments,omitzero"` // for function_call
	Output   string        `json:"output,omitzero"`    // for function_call_output
}

// ContentPart represents a part of message content.
type ContentPart struct {
	Type       string `json:"type,omitzero"` // "input_text", "input_audio", "item_reference", "text", "audio"
	Text       string `json:"text,omitzero"`
	Audio      string `json:"audio,omitzero"`      // base64 encoded
	Transcript string `json:"transcript,omitzero"` // for audio parts
	ID         string `json:"id,omitzero"`         // for item_reference
}

// ResponseResource represents a response from the model.
type ResponseResource struct {
	ID                 string             `json:"id,omitzero"`
	Object             string             `json:"object,omitzero"`
	Status             string             `json:"status,omitzero"` // "in_progress", "completed", "cancelled", "incomplete", "failed"
	StatusDetails      *StatusDetails     `json:"status_details,omitzero"`
	Output             []ConversationItem `json:"output,omitzero"`
	Usage              *Usage             `json:"usage,omitzero"`
}

// StatusDetails contains details about the response status.
type StatusDetails struct {
	Type   string `json:"type,omitzero"`
	Reason string `json:"reason,omitzero"`
	Error  *Error `json:"error,omitzero"`
}

// Usage contains token usage information.
type Usage struct {
	TotalTokens              int          `json:"total_tokens,omitzero"`
	InputTokens              int          `json:"input_tokens,omitzero"`
	OutputTokens             int          `json:"output_tokens,omitzero"`
	InputTokenDetails        *TokenDetails `json:"input_token_details,omitzero"`
	OutputTokenDetails       *TokenDetails `json:"output_token_details,omitzero"`
}

// TokenDetails contains detailed token breakdown.
type TokenDetails struct {
	CachedTokens     int `json:"cached_tokens,omitzero"`
	TextTokens       int `json:"text_tokens,omitzero"`
	AudioTokens      int `json:"audio_tokens,omitzero"`
	CachedTokensDetails *CachedTokensDetails `json:"cached_tokens_details,omitzero"`
}

// CachedTokensDetails contains details about cached tokens.
type CachedTokensDetails struct {
	TextTokens  int `json:"text_tokens,omitzero"`
	AudioTokens int `json:"audio_tokens,omitzero"`
}
