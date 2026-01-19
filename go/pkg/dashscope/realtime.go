package dashscope

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// RealtimeService provides access to the Qwen-Omni-Realtime API.
type RealtimeService struct {
	client *Client
}

// Connect establishes a realtime session with the specified configuration.
// The session uses standard JSON messages similar to OpenAI Realtime API format.
func (s *RealtimeService) Connect(ctx context.Context, config *RealtimeConfig) (*RealtimeSession, error) {
	if config == nil {
		config = &RealtimeConfig{}
	}
	if config.Model == "" {
		config.Model = ModelQwenOmniTurboRealtimeLatest
	}

	// Build WebSocket URL: wss://dashscope.aliyuncs.com/api-ws/v1/realtime?model={model}
	url := fmt.Sprintf("%s?model=%s", s.client.config.baseURL, config.Model)

	// Build headers
	headers := http.Header{}
	headers.Set("Authorization", "bearer "+s.client.config.apiKey)
	if s.client.config.workspaceID != "" {
		headers.Set("X-DashScope-WorkSpace", s.client.config.workspaceID)
	}

	// Dial WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: s.client.config.httpClient.Timeout,
	}

	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		if resp != nil {
			return nil, &Error{
				Code:       "ConnectionFailed",
				Message:    fmt.Sprintf("failed to connect: %v", err),
				HTTPStatus: resp.StatusCode,
			}
		}
		return nil, fmt.Errorf("dashscope: failed to connect: %w", err)
	}

	session := &RealtimeSession{
		conn:     conn,
		config:   config,
		client:   s.client,
		closeCh:  make(chan struct{}),
		eventsCh: make(chan eventOrError, 100),
	}

	// Start background reader
	go session.readLoop()

	return session, nil
}

// RealtimeSession represents an active realtime session.
type RealtimeSession struct {
	conn      *websocket.Conn
	config    *RealtimeConfig
	client    *Client
	sessionID string
	closeCh   chan struct{}
	eventsCh  chan eventOrError
	closeOnce sync.Once
	mu        sync.Mutex
}

type eventOrError struct {
	event *RealtimeEvent
	err   error
}

// generateEventID generates a unique event ID.
func generateEventID() string {
	return "event_" + uuid.New().String()[:8]
}

// UpdateSession updates the session configuration.
// This should be called after receiving session.created event.
func (s *RealtimeSession) UpdateSession(config *SessionConfig) error {
	sessionConfig := map[string]interface{}{}

	if len(config.Modalities) > 0 {
		sessionConfig["modalities"] = config.Modalities
	}
	if config.Voice != "" {
		sessionConfig["voice"] = config.Voice
	}
	if config.InputAudioFormat != "" {
		sessionConfig["input_audio_format"] = config.InputAudioFormat
	}
	if config.OutputAudioFormat != "" {
		sessionConfig["output_audio_format"] = config.OutputAudioFormat
	}
	if config.Instructions != "" {
		sessionConfig["instructions"] = config.Instructions
	}
	if config.EnableInputAudioTranscription {
		// Only send if model is specified, otherwise server uses default
		if config.InputAudioTranscriptionModel != "" {
			sessionConfig["input_audio_transcription"] = map[string]interface{}{
				"model": config.InputAudioTranscriptionModel,
			}
		}
	}
	if config.TurnDetection != nil {
		turnDetection := map[string]interface{}{
			"type": config.TurnDetection.Type,
		}
		if config.TurnDetection.Threshold > 0 {
			turnDetection["threshold"] = config.TurnDetection.Threshold
		}
		if config.TurnDetection.PrefixPaddingMs > 0 {
			turnDetection["prefix_padding_ms"] = config.TurnDetection.PrefixPaddingMs
		}
		if config.TurnDetection.SilenceDurationMs > 0 {
			turnDetection["silence_duration_ms"] = config.TurnDetection.SilenceDurationMs
		}
		sessionConfig["turn_detection"] = turnDetection
	}

	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "session.update",
		"session":  sessionConfig,
	})
}

// AppendAudio sends audio data to the input audio buffer.
// Audio should be base64 encoded PCM data.
func (s *RealtimeSession) AppendAudio(audio []byte) error {
	encoded := base64.StdEncoding.EncodeToString(audio)
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "input_audio_buffer.append",
		"audio":    encoded,
	})
}

// AppendAudioBase64 sends base64-encoded audio data to the input audio buffer.
func (s *RealtimeSession) AppendAudioBase64(audioBase64 string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "input_audio_buffer.append",
		"audio":    audioBase64,
	})
}

// AppendImage sends an image frame for video input.
// Image should be base64 encoded.
func (s *RealtimeSession) AppendImage(image []byte) error {
	encoded := base64.StdEncoding.EncodeToString(image)
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "input_image_buffer.append",
		"image":    encoded,
	})
}

// CommitInput commits the audio buffer.
// In server_vad mode, this is called automatically after VAD detects end of speech.
// In manual mode, call this to indicate end of user input.
func (s *RealtimeSession) CommitInput() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "input_audio_buffer.commit",
	})
}

// ClearInput clears the input audio buffer.
func (s *RealtimeSession) ClearInput() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "input_audio_buffer.clear",
	})
}

// CreateResponse requests the model to generate a response.
// In server_vad mode, this is called automatically by the server.
// In manual mode, call this after CommitInput to trigger response generation.
func (s *RealtimeSession) CreateResponse(opts *ResponseCreateOptions) error {
	event := map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "response.create",
		"response": map[string]interface{}{},
	}

	if opts != nil {
		response := event["response"].(map[string]interface{})
		if opts.Instructions != "" {
			response["instructions"] = opts.Instructions
		}
		if len(opts.Modalities) > 0 {
			response["modalities"] = opts.Modalities
		}
	}

	return s.sendEvent(event)
}

// CancelResponse cancels the current response generation.
func (s *RealtimeSession) CancelResponse() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "response.cancel",
	})
}

// FinishSession sends a session.finish event to gracefully end the session.
func (s *RealtimeSession) FinishSession() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     "session.finish",
	})
}

// SendRaw sends a raw JSON event to the server.
// Use this for events not covered by helper methods.
func (s *RealtimeSession) SendRaw(event map[string]interface{}) error {
	return s.sendEvent(event)
}

// Events returns an iterator over session events.
func (s *RealtimeSession) Events() iter.Seq2[*RealtimeEvent, error] {
	return func(yield func(*RealtimeEvent, error) bool) {
		for {
			select {
			case <-s.closeCh:
				return
			case item, ok := <-s.eventsCh:
				if !ok {
					return
				}
				if !yield(item.event, item.err) {
					return
				}
				if item.err != nil {
					return
				}
			}
		}
	}
}

// Close closes the session.
func (s *RealtimeSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closeCh)
		err = s.conn.Close()
	})
	return err
}

// SessionID returns the session ID assigned by the server.
func (s *RealtimeSession) SessionID() string {
	return s.sessionID
}

// sendEvent sends a JSON event to the server.
func (s *RealtimeSession) sendEvent(event map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Debug: log the event being sent
	if s.client.config.debug {
		if jsonBytes, err := json.MarshalIndent(event, "", "  "); err == nil {
			// Truncate for readability
			str := string(jsonBytes)
			if len(str) > 500 {
				str = str[:500] + "..."
			}
			fmt.Printf("[DEBUG] Sending: %s\n", str)
		}
	}

	return s.conn.WriteJSON(event)
}

// readLoop reads events from the WebSocket connection.
func (s *RealtimeSession) readLoop() {
	defer close(s.eventsCh)

	for {
		select {
		case <-s.closeCh:
			return
		default:
		}

		_, message, err := s.conn.ReadMessage()
		if err != nil {
			select {
			case <-s.closeCh:
				return
			case s.eventsCh <- eventOrError{err: fmt.Errorf("read error: %w", err)}:
			}
			return
		}

		// Debug: log received message
		if s.client.config.debug {
			msgStr := string(message)
			if len(msgStr) > 1000 {
				msgStr = msgStr[:1000] + "..."
			}
			fmt.Printf("[DEBUG] Received (len=%d): %s\n", len(message), msgStr)
		}

		// Parse JSON event
		var rawEvent map[string]json.RawMessage
		if err := json.Unmarshal(message, &rawEvent); err != nil {
			select {
			case <-s.closeCh:
				return
			case s.eventsCh <- eventOrError{err: fmt.Errorf("parse error: %w", err)}:
			}
			continue
		}

		// Extract event type
		var eventType string
		if typeRaw, ok := rawEvent["type"]; ok {
			json.Unmarshal(typeRaw, &eventType)
		}

		// Check for error event
		if eventType == "error" {
			var errorData struct {
				Type  string `json:"type"`
				Error struct {
					Type    string `json:"type"`
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(message, &errorData); err == nil {
				select {
				case <-s.closeCh:
					return
				case s.eventsCh <- eventOrError{err: &Error{
					Code:    errorData.Error.Code,
					Message: errorData.Error.Message,
				}}:
				}
				continue
			}
		}

		// Convert to RealtimeEvent
		event := s.parseEvent(eventType, message)
		if event != nil {
			// Track session ID
			if eventType == "session.created" && event.Session != nil {
				s.sessionID = event.Session.ID
			}

			select {
			case <-s.closeCh:
				return
			case s.eventsCh <- eventOrError{event: event}:
			}
		}
	}
}

// parseEvent parses a raw JSON message into a RealtimeEvent.
func (s *RealtimeSession) parseEvent(eventType string, message []byte) *RealtimeEvent {
	event := &RealtimeEvent{
		Type: eventType,
	}

	// Check for DashScope "choices" format (different from OpenAI Realtime events)
	var choicesData struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role    string `json:"role"`
				Content []struct {
					Text  string `json:"text,omitempty"`
					Audio *struct {
						Data string `json:"data"`
					} `json:"audio,omitempty"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(message, &choicesData); err == nil && len(choicesData.Choices) > 0 {
		// DashScope "choices" format response
		event.Type = EventTypeChoicesResponse
		choice := choicesData.Choices[0]

		for _, content := range choice.Message.Content {
			if content.Text != "" {
				event.Delta = content.Text
			}
			if content.Audio != nil && content.Audio.Data != "" {
				event.AudioBase64 = content.Audio.Data
				if decoded, err := base64.StdEncoding.DecodeString(content.Audio.Data); err == nil {
					event.Audio = decoded
				}
			}
		}

		if choice.FinishReason != "" && choice.FinishReason != "null" {
			event.FinishReason = choice.FinishReason
		}

		return event
	}

	// Standard event format
	switch eventType {
	case EventTypeSessionCreated, EventTypeSessionUpdated:
		var data struct {
			Session struct {
				ID string `json:"id"`
			} `json:"session"`
		}
		if err := json.Unmarshal(message, &data); err == nil && data.Session.ID != "" {
			event.Session = &SessionInfo{ID: data.Session.ID}
		}

	case EventTypeResponseCreated:
		var data struct {
			Response struct {
				ID string `json:"id"`
			} `json:"response"`
		}
		if err := json.Unmarshal(message, &data); err == nil {
			event.ResponseID = data.Response.ID
		}

	case EventTypeResponseAudioDelta:
		var data struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(message, &data); err == nil && data.Delta != "" {
			event.AudioBase64 = data.Delta
			if decoded, err := base64.StdEncoding.DecodeString(data.Delta); err == nil {
				event.Audio = decoded
			}
		}

	case EventTypeResponseTranscriptDelta:
		var data struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(message, &data); err == nil {
			event.Delta = data.Delta
		}

	case EventTypeResponseTextDelta:
		var data struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(message, &data); err == nil {
			event.Delta = data.Delta
		}

	case "conversation.item.input_audio_transcription.completed":
		var data struct {
			Transcript string `json:"transcript"`
		}
		if err := json.Unmarshal(message, &data); err == nil {
			event.Transcript = data.Transcript
		}

	case EventTypeResponseDone:
		var data struct {
			Response struct {
				Usage struct {
					TotalTokens  int `json:"total_tokens"`
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal(message, &data); err == nil {
			event.Usage = &UsageStats{
				TotalTokens:  data.Response.Usage.TotalTokens,
				InputTokens:  data.Response.Usage.InputTokens,
				OutputTokens: data.Response.Usage.OutputTokens,
			}
		}
	}

	return event
}

// ResponseCreateOptions contains options for creating a response.
type ResponseCreateOptions struct {
	// Instructions override for this response (optional)
	Instructions string
	// Output modalities for this response (optional)
	Modalities []string
}
