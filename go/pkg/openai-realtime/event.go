package openairealtime

// Client event types (sent from client to server).
const (
	// Session events
	EventTypeSessionUpdate = "session.update"

	// Input audio buffer events
	EventTypeInputAudioBufferAppend = "input_audio_buffer.append"
	EventTypeInputAudioBufferCommit = "input_audio_buffer.commit"
	EventTypeInputAudioBufferClear  = "input_audio_buffer.clear"

	// Conversation item events
	EventTypeConversationItemCreate   = "conversation.item.create"
	EventTypeConversationItemTruncate = "conversation.item.truncate"
	EventTypeConversationItemDelete   = "conversation.item.delete"

	// Response events
	EventTypeResponseCreate = "response.create"
	EventTypeResponseCancel = "response.cancel"
)

// Server event types (sent from server to client).
const (
	// Error event
	EventTypeError = "error"

	// Session events
	EventTypeSessionCreated = "session.created"
	EventTypeSessionUpdated = "session.updated"

	// Conversation events
	EventTypeConversationCreated                              = "conversation.created"
	EventTypeConversationItemCreated                          = "conversation.item.created"
	EventTypeConversationItemInputAudioTranscriptionCompleted = "conversation.item.input_audio_transcription.completed"
	EventTypeConversationItemInputAudioTranscriptionFailed    = "conversation.item.input_audio_transcription.failed"
	EventTypeConversationItemTruncated                        = "conversation.item.truncated"
	EventTypeConversationItemDeleted                          = "conversation.item.deleted"

	// Input audio buffer events
	EventTypeInputAudioBufferCommitted    = "input_audio_buffer.committed"
	EventTypeInputAudioBufferCleared      = "input_audio_buffer.cleared"
	EventTypeInputAudioBufferSpeechStarted = "input_audio_buffer.speech_started"
	EventTypeInputAudioBufferSpeechStopped = "input_audio_buffer.speech_stopped"

	// Response events
	EventTypeResponseCreated         = "response.created"
	EventTypeResponseDone            = "response.done"
	EventTypeResponseOutputItemAdded = "response.output_item.added"
	EventTypeResponseOutputItemDone  = "response.output_item.done"
	EventTypeResponseContentPartAdded = "response.content_part.added"
	EventTypeResponseContentPartDone  = "response.content_part.done"

	// Response text events
	EventTypeResponseTextDelta = "response.text.delta"
	EventTypeResponseTextDone  = "response.text.done"

	// Response audio events
	EventTypeResponseAudioDelta = "response.audio.delta"
	EventTypeResponseAudioDone  = "response.audio.done"

	// Response audio transcript events
	EventTypeResponseAudioTranscriptDelta = "response.audio_transcript.delta"
	EventTypeResponseAudioTranscriptDone  = "response.audio_transcript.done"

	// Response function call events
	EventTypeResponseFunctionCallArgumentsDelta = "response.function_call_arguments.delta"
	EventTypeResponseFunctionCallArgumentsDone  = "response.function_call_arguments.done"

	// Rate limits event
	EventTypeRateLimitsUpdated = "rate_limits.updated"
)

// ServerEvent represents a server event received from the Realtime API.
type ServerEvent struct {
	// Type is the event type.
	Type string `json:"type"`

	// EventID is the unique identifier for this event.
	EventID string `json:"event_id,omitzero"`

	// === Session events ===

	// Session contains session information (for session.created, session.updated).
	Session *SessionResource `json:"session,omitzero"`

	// === Conversation events ===

	// Conversation contains conversation information (for conversation.created).
	Conversation *ConversationResource `json:"conversation,omitzero"`

	// Item contains conversation item (for conversation.item.* events).
	Item *ConversationItem `json:"item,omitzero"`

	// === Input audio buffer events ===

	// PreviousItemID is the ID of the previous item (for input_audio_buffer.committed).
	PreviousItemID string `json:"previous_item_id,omitzero"`

	// ItemID is the ID of the item (for various events).
	ItemID string `json:"item_id,omitzero"`

	// AudioStartMs is the start time in milliseconds (for speech_started).
	AudioStartMs int `json:"audio_start_ms,omitzero"`

	// AudioEndMs is the end time in milliseconds (for speech_stopped, truncated).
	AudioEndMs int `json:"audio_end_ms,omitzero"`

	// === Transcription events ===

	// Transcript is the transcription text.
	Transcript string `json:"transcript,omitzero"`

	// ContentIndex is the index of the content part.
	ContentIndex int `json:"content_index,omitzero"`

	// TranscriptionError contains error info for transcription failure.
	TranscriptionError *EventError `json:"error,omitzero"`

	// === Response events ===

	// Response contains response information (for response.* events).
	Response *ResponseResource `json:"response,omitzero"`

	// ResponseID is the response identifier.
	ResponseID string `json:"response_id,omitzero"`

	// OutputIndex is the index of the output item.
	OutputIndex int `json:"output_index,omitzero"`

	// Part contains content part information.
	Part *ContentPart `json:"part,omitzero"`

	// === Delta events ===

	// Delta contains incremental text/arguments (for *.delta events).
	Delta string `json:"delta,omitzero"`

	// Audio contains decoded audio data (populated after parsing).
	Audio []byte `json:"-"`

	// AudioBase64 contains base64 audio directly from JSON.
	// Note: In the API, this field is named "delta" for audio events.
	AudioBase64 string `json:"-"`

	// === Function call events ===

	// CallID is the function call ID.
	CallID string `json:"call_id,omitzero"`

	// Name is the function name.
	Name string `json:"name,omitzero"`

	// Arguments is the function arguments (complete, for done event).
	Arguments string `json:"arguments,omitzero"`

	// === Rate limits event ===

	// RateLimits contains rate limit information.
	RateLimits []RateLimit `json:"rate_limits,omitzero"`

	// === Raw data ===

	// Raw contains the original JSON message.
	Raw []byte `json:"-"`
}

// RateLimit represents rate limit information.
type RateLimit struct {
	Name         string  `json:"name"`
	Limit        int     `json:"limit"`
	Remaining    int     `json:"remaining"`
	ResetSeconds float64 `json:"reset_seconds"`
}
