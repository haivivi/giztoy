package dashscope

// Event types for realtime communication.
const (
	// Client events
	EventTypeSessionUpdate       = "session.update"
	EventTypeInputAudioAppend    = "input_audio_buffer.append"
	EventTypeInputAudioCommit    = "input_audio_buffer.commit"
	EventTypeInputAudioClear     = "input_audio_buffer.clear"
	EventTypeResponseCreate      = "response.create"
	EventTypeResponseCancel      = "response.cancel"
	EventTypeTranscriptionUpdate = "transcription.update"

	// Server events
	EventTypeSessionCreated          = "session.created"
	EventTypeSessionUpdated          = "session.updated"
	EventTypeInputAudioCommitted     = "input_audio_buffer.committed"
	EventTypeInputAudioCleared       = "input_audio_buffer.cleared"
	EventTypeInputSpeechStarted      = "input_audio_buffer.speech_started"
	EventTypeInputSpeechStopped      = "input_audio_buffer.speech_stopped"
	EventTypeResponseCreated         = "response.created"
	EventTypeResponseDone            = "response.done"
	EventTypeResponseOutputAdded     = "response.output_item.added"
	EventTypeResponseOutputDone      = "response.output_item.done"
	EventTypeResponseContentAdded    = "response.content_part.added"
	EventTypeResponseContentDone     = "response.content_part.done"
	EventTypeResponseTextDelta       = "response.text.delta"
	EventTypeResponseTextDone        = "response.text.done"
	EventTypeResponseAudioDelta      = "response.audio.delta"
	EventTypeResponseAudioDone       = "response.audio.done"
	EventTypeResponseTranscriptDelta = "response.audio_transcript.delta"
	EventTypeResponseTranscriptDone  = "response.audio_transcript.done"
	EventTypeError                   = "error"

	// DashScope-specific: "choices" format response (different from OpenAI Realtime)
	EventTypeChoicesResponse = "choices"
)

// RealtimeEvent represents an event in the realtime session.
type RealtimeEvent struct {
	// Type is the event type.
	// For DashScope "choices" format responses, this will be EventTypeChoicesResponse.
	Type string `json:"type"`

	// EventID is the unique identifier for this event.
	EventID string `json:"event_id,omitempty"`

	// Session contains session information (for session.* events).
	Session *SessionInfo `json:"session,omitempty"`

	// Response contains response information (for response.* events).
	Response *ResponseInfo `json:"response,omitempty"`

	// ResponseID is the response identifier (for response.created).
	ResponseID string `json:"response_id,omitempty"`

	// Delta contains incremental text content (for *.delta events and choices responses).
	Delta string `json:"delta,omitempty"`

	// Audio contains decoded audio data (for response.audio.delta and choices responses).
	Audio []byte `json:"-"`

	// AudioBase64 is the raw base64 audio from JSON.
	AudioBase64 string `json:"audio,omitempty"`

	// Transcript contains transcript text (for transcript completion events).
	Transcript string `json:"transcript,omitempty"`

	// FinishReason indicates why the response ended (for choices responses).
	// Values: "stop", "length", etc. Empty string means still generating.
	FinishReason string `json:"finish_reason,omitempty"`

	// ItemID is the item identifier (for item events).
	ItemID string `json:"item_id,omitempty"`

	// OutputIndex is the output index (for content events).
	OutputIndex int `json:"output_index,omitempty"`

	// ContentIndex is the content index (for content events).
	ContentIndex int `json:"content_index,omitempty"`

	// Error contains error information (for error events).
	Error *EventError `json:"error,omitempty"`

	// Usage contains usage statistics (for response.done).
	Usage *UsageStats `json:"usage,omitempty"`
}

// ResponseInfo contains response state information.
type ResponseInfo struct {
	ID           string        `json:"id,omitempty"`
	Status       string        `json:"status,omitempty"`
	StatusDetail *StatusDetail `json:"status_detail,omitempty"`
	Output       []OutputItem  `json:"output,omitempty"`
	Usage        *UsageStats   `json:"usage,omitempty"`
}

// StatusDetail contains detailed status information.
type StatusDetail struct {
	Type   string `json:"type,omitempty"`
	Reason string `json:"reason,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

// OutputItem represents an output item in a response.
type OutputItem struct {
	ID      string        `json:"id,omitempty"`
	Type    string        `json:"type,omitempty"`
	Role    string        `json:"role,omitempty"`
	Status  string        `json:"status,omitempty"`
	Content []ContentPart `json:"content,omitempty"`
}

// ContentPart represents a part of content.
type ContentPart struct {
	Type       string `json:"type,omitempty"`
	Text       string `json:"text,omitempty"`
	Audio      string `json:"audio,omitempty"`
	Transcript string `json:"transcript,omitempty"`
}

// EventError contains error information from error events.
type EventError struct {
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Param   string `json:"param,omitempty"`
}
