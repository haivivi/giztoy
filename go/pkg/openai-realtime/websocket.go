package openairealtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WebSocketSession is a WebSocket-based realtime session.
type WebSocketSession struct {
	conn      *websocket.Conn
	config    *ConnectConfig
	client    *Client
	sessionID string
	closeCh   chan struct{}
	eventsCh  chan eventOrError
	closeOnce sync.Once
	mu        sync.Mutex
}

type eventOrError struct {
	event *ServerEvent
	err   error
}

// connectWebSocket establishes a WebSocket connection.
func (c *Client) connectWebSocket(ctx context.Context, config *ConnectConfig) (*WebSocketSession, error) {
	if config == nil {
		config = &ConnectConfig{}
	}
	if config.Model == "" {
		config.Model = ModelGPT4oRealtimePreview
	}

	// Build WebSocket URL with model query parameter
	url := fmt.Sprintf("%s?model=%s", c.config.wsURL, config.Model)

	// Build headers
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.config.apiKey)
	headers.Set("OpenAI-Beta", "realtime=v1")
	if c.config.organization != "" {
		headers.Set("OpenAI-Organization", c.config.organization)
	}
	if c.config.project != "" {
		headers.Set("OpenAI-Project", c.config.project)
	}

	// Dial WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: c.config.httpClient.Timeout,
	}

	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		if resp != nil {
			return nil, &Error{
				Code:       "connection_failed",
				Message:    fmt.Sprintf("failed to connect: %v", err),
				HTTPStatus: resp.StatusCode,
			}
		}
		return nil, fmt.Errorf("openai-realtime: failed to connect: %w", err)
	}

	session := &WebSocketSession{
		conn:     conn,
		config:   config,
		client:   c,
		closeCh:  make(chan struct{}),
		eventsCh: make(chan eventOrError, 100),
	}

	// Start background reader
	go session.readLoop()

	return session, nil
}

// generateEventID generates a unique event ID.
func generateEventID() string {
	return "evt_" + uuid.New().String()[:12]
}

// UpdateSession updates the session configuration.
func (s *WebSocketSession) UpdateSession(config *SessionConfig) error {
	event := map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeSessionUpdate,
		"session":  config,
	}
	return s.sendEvent(event)
}

// AppendAudio appends PCM audio data to the input audio buffer.
func (s *WebSocketSession) AppendAudio(audio []byte) error {
	encoded := base64.StdEncoding.EncodeToString(audio)
	return s.AppendAudioBase64(encoded)
}

// AppendAudioBase64 appends base64-encoded audio data to the input buffer.
func (s *WebSocketSession) AppendAudioBase64(audioBase64 string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeInputAudioBufferAppend,
		"audio":    audioBase64,
	})
}

// CommitInput commits the audio buffer.
func (s *WebSocketSession) CommitInput() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeInputAudioBufferCommit,
	})
}

// ClearInput clears the input audio buffer.
func (s *WebSocketSession) ClearInput() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeInputAudioBufferClear,
	})
}

// AddUserMessage adds a user text message to the conversation.
func (s *WebSocketSession) AddUserMessage(text string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeConversationItemCreate,
		"item": map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []map[string]interface{}{
				{
					"type": "input_text",
					"text": text,
				},
			},
		},
	})
}

// AddUserAudio adds a user audio message to the conversation.
func (s *WebSocketSession) AddUserAudio(audioBase64 string, transcript string) error {
	content := map[string]interface{}{
		"type":  "input_audio",
		"audio": audioBase64,
	}
	if transcript != "" {
		content["transcript"] = transcript
	}
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeConversationItemCreate,
		"item": map[string]interface{}{
			"type":    "message",
			"role":    "user",
			"content": []map[string]interface{}{content},
		},
	})
}

// AddAssistantMessage adds an assistant text message to the conversation.
func (s *WebSocketSession) AddAssistantMessage(text string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeConversationItemCreate,
		"item": map[string]interface{}{
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": text,
				},
			},
		},
	})
}

// AddFunctionCallOutput adds a function call output to the conversation.
func (s *WebSocketSession) AddFunctionCallOutput(callID string, output string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeConversationItemCreate,
		"item": map[string]interface{}{
			"type":    "function_call_output",
			"call_id": callID,
			"output":  output,
		},
	})
}

// TruncateItem truncates a conversation item.
func (s *WebSocketSession) TruncateItem(itemID string, contentIndex int, audioEndMs int) error {
	return s.sendEvent(map[string]interface{}{
		"event_id":      generateEventID(),
		"type":          EventTypeConversationItemTruncate,
		"item_id":       itemID,
		"content_index": contentIndex,
		"audio_end_ms":  audioEndMs,
	})
}

// DeleteItem deletes a conversation item.
func (s *WebSocketSession) DeleteItem(itemID string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeConversationItemDelete,
		"item_id":  itemID,
	})
}

// CreateResponse requests the model to generate a response.
func (s *WebSocketSession) CreateResponse(opts *ResponseCreateOptions) error {
	event := map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeResponseCreate,
	}

	if opts != nil {
		response := map[string]interface{}{}
		if len(opts.Modalities) > 0 {
			response["modalities"] = opts.Modalities
		}
		if opts.Instructions != "" {
			response["instructions"] = opts.Instructions
		}
		if opts.Voice != "" {
			response["voice"] = opts.Voice
		}
		if opts.OutputAudioFormat != "" {
			response["output_audio_format"] = opts.OutputAudioFormat
		}
		if len(opts.Tools) > 0 {
			response["tools"] = opts.Tools
		}
		if opts.ToolChoice != nil {
			response["tool_choice"] = opts.ToolChoice
		}
		if opts.Temperature != nil {
			response["temperature"] = *opts.Temperature
		}
		if opts.MaxOutputTokens != nil {
			response["max_output_tokens"] = opts.MaxOutputTokens
		}
		if opts.Conversation != "" {
			response["conversation"] = opts.Conversation
		}
		if len(opts.Input) > 0 {
			response["input"] = opts.Input
		}
		if len(response) > 0 {
			event["response"] = response
		}
	}

	return s.sendEvent(event)
}

// CancelResponse cancels the current response generation.
func (s *WebSocketSession) CancelResponse() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeResponseCancel,
	})
}

// Events returns an iterator over server events.
func (s *WebSocketSession) Events() iter.Seq2[*ServerEvent, error] {
	return func(yield func(*ServerEvent, error) bool) {
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

// SendRaw sends a raw JSON event to the server.
func (s *WebSocketSession) SendRaw(event map[string]interface{}) error {
	return s.sendEvent(event)
}

// Close closes the session.
func (s *WebSocketSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closeCh)
		err = s.conn.Close()
	})
	return err
}

// SessionID returns the session ID.
func (s *WebSocketSession) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// sendEvent sends a JSON event to the server.
func (s *WebSocketSession) sendEvent(event map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		if jsonBytes, err := json.MarshalIndent(event, "", "  "); err == nil {
			str := string(jsonBytes)
			if len(str) > 500 {
				str = str[:500] + "..."
			}
			slog.Debug("sending event", "content", str)
		}
	}

	return s.conn.WriteJSON(event)
}

// readLoop reads events from the WebSocket connection.
func (s *WebSocketSession) readLoop() {
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

		if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
			msgStr := string(message)
			if len(msgStr) > 1000 {
				msgStr = msgStr[:1000] + "..."
			}
			slog.Debug("received message", "len", len(message), "content", msgStr)
		}

		event, err := s.parseEvent(message)
		if err != nil {
			select {
			case <-s.closeCh:
				return
			case s.eventsCh <- eventOrError{err: err}:
			}
			continue
		}

		// Track session ID
		if event.Type == EventTypeSessionCreated && event.Session != nil {
			s.mu.Lock()
			s.sessionID = event.Session.ID
			s.mu.Unlock()
		}

		// Check for error event
		if event.Type == EventTypeError && event.TranscriptionError != nil {
			select {
			case <-s.closeCh:
				return
			case s.eventsCh <- eventOrError{err: event.TranscriptionError.ToError()}:
			}
			continue
		}

		select {
		case <-s.closeCh:
			return
		case s.eventsCh <- eventOrError{event: event}:
		}
	}
}

// parseEvent parses a raw JSON message into a ServerEvent.
func (s *WebSocketSession) parseEvent(message []byte) (*ServerEvent, error) {
	var event ServerEvent
	if err := json.Unmarshal(message, &event); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	event.Raw = message

	// Handle audio delta - the "delta" field contains base64 audio
	if event.Type == EventTypeResponseAudioDelta && event.Delta != "" {
		event.AudioBase64 = event.Delta
		if decoded, err := base64.StdEncoding.DecodeString(event.Delta); err == nil {
			event.Audio = decoded
		}
	}

	return &event, nil
}

// Ensure WebSocketSession implements Session interface.
var _ Session = (*WebSocketSession)(nil)
