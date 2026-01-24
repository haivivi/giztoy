package openairealtime

import "iter"

// Session is the common interface for OpenAI Realtime sessions.
// Both WebSocket and WebRTC implementations satisfy this interface.
type Session interface {
	// === Session Management ===

	// UpdateSession updates the session configuration.
	// This should be called after receiving session.created event.
	UpdateSession(config *SessionConfig) error

	// Close closes the session connection.
	Close() error

	// SessionID returns the session ID assigned by the server.
	// Returns empty string if session.created has not been received yet.
	SessionID() string

	// === Audio Input ===

	// AppendAudio appends PCM audio data to the input audio buffer.
	// Audio format requirements:
	//   - Sample rate: 24kHz
	//   - Bit depth: 16-bit signed integers
	//   - Channels: Mono (1 channel)
	//   - Encoding: Little-endian PCM
	// The audio is automatically base64 encoded before sending.
	AppendAudio(audio []byte) error

	// AppendAudioBase64 appends base64-encoded audio data to the input buffer.
	AppendAudioBase64(audioBase64 string) error

	// CommitInput commits the audio buffer and creates a user message.
	// In server_vad mode, this is called automatically after VAD detects end of speech.
	// In manual mode (turn_detection: null), call this to indicate end of user input.
	CommitInput() error

	// ClearInput clears the input audio buffer without creating a message.
	ClearInput() error

	// === Conversation Management ===

	// AddUserMessage adds a user text message to the conversation.
	AddUserMessage(text string) error

	// AddUserAudio adds a user audio message to the conversation.
	// Audio should be base64 encoded. Transcript is optional.
	AddUserAudio(audioBase64 string, transcript string) error

	// AddAssistantMessage adds an assistant text message to the conversation.
	AddAssistantMessage(text string) error

	// AddFunctionCallOutput adds a function call output to the conversation.
	AddFunctionCallOutput(callID string, output string) error

	// TruncateItem truncates a conversation item (assistant audio).
	// contentIndex is the index of the content part to truncate.
	// audioEndMs is the audio end time in milliseconds.
	TruncateItem(itemID string, contentIndex int, audioEndMs int) error

	// DeleteItem deletes a conversation item.
	DeleteItem(itemID string) error

	// === Response Control ===

	// CreateResponse requests the model to generate a response.
	// In server_vad mode, this is called automatically by the server.
	// In manual mode, call this after CommitInput to trigger response generation.
	// Pass nil for default options.
	CreateResponse(opts *ResponseCreateOptions) error

	// CancelResponse cancels the current response generation.
	CancelResponse() error

	// === Event Reception ===

	// Events returns an iterator over server events.
	// The iterator yields events until the session is closed or an error occurs.
	// After an error is yielded, iteration stops.
	Events() iter.Seq2[*ServerEvent, error]

	// === Raw Operations ===

	// SendRaw sends a raw JSON event to the server.
	// Use this for events not covered by helper methods.
	SendRaw(event map[string]interface{}) error
}
