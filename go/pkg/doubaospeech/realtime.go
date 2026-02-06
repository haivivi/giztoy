package doubaospeech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ================== Realtime Types ==================

// RealtimeEventType represents realtime event type
type RealtimeEventType int32

const (
	// Connection events
	EventConnectionStarted RealtimeEventType = 50
	EventConnectionFailed  RealtimeEventType = 51
	EventConnectionEnded   RealtimeEventType = 52

	// Session events
	EventSessionStarted RealtimeEventType = 150
	EventSessionFinished RealtimeEventType = 152
	EventSessionFailed  RealtimeEventType = 153
	EventUsageResponse  RealtimeEventType = 154

	// ASR events (per official API doc)
	EventASRInfo     RealtimeEventType = 450 // First word detected (interrupt)
	EventASRResponse RealtimeEventType = 451 // ASR text result
	EventASREnded    RealtimeEventType = 459 // User speech ended

	// TTS events
	EventTTSStarted    RealtimeEventType = 350 // TTSSentenceStart
	EventTTSSegmentEnd RealtimeEventType = 351 // TTSSentenceEnd
	EventTTSAudioData  RealtimeEventType = 352 // TTSResponse (audio)
	EventTTSFinished   RealtimeEventType = 359 // TTSEnded

	// Chat/LLM events
	EventChatResponse RealtimeEventType = 550 // Model text response
	EventChatEnded    RealtimeEventType = 559 // Model response ended

	// Legacy aliases
	EventAudioReceived = EventTTSAudioData
	EventSessionEnded  = EventSessionFinished // Alias for compatibility
	EventASRStarted    = EventASRInfo         // Alias
	EventASRFinished   = EventASREnded        // Alias
)

// RealtimeConfig represents realtime session configuration
type RealtimeConfig struct {
	ASR    RealtimeASRConfig    `json:"asr"`
	TTS    RealtimeTTSConfig    `json:"tts"`
	Dialog RealtimeDialogConfig `json:"dialog"`
}

// RealtimeASRConfig represents ASR configuration
type RealtimeASRConfig struct {
	Extra map[string]any `json:"extra,omitempty"`
}

// RealtimeTTSConfig represents TTS configuration
type RealtimeTTSConfig struct {
	Speaker     string                   `json:"speaker"`
	AudioConfig RealtimeAudioConfig      `json:"audio_config"`
	Extra       map[string]any           `json:"extra,omitempty"`
}

// RealtimeAudioConfig represents audio configuration
type RealtimeAudioConfig struct {
	Channel    int    `json:"channel"`
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
}

// RealtimeDialogConfig represents dialog configuration
type RealtimeDialogConfig struct {
	BotName           string          `json:"bot_name,omitempty"`
	SystemRole        string          `json:"system_role,omitempty"`
	SpeakingStyle     string          `json:"speaking_style,omitempty"`
	CharacterManifest string          `json:"character_manifest,omitempty"`
	Location          *LocationInfo   `json:"location,omitempty"`
	Extra             map[string]any  `json:"extra,omitempty"`
}

// RealtimeEvent represents a realtime event
type RealtimeEvent struct {
	Type      RealtimeEventType `json:"type"`
	SessionID string            `json:"session_id"`
	Text      string            `json:"text,omitempty"`
	Audio     []byte            `json:"audio,omitempty"`
	Payload   []byte            `json:"payload,omitempty"`
	ASRInfo   *RealtimeASRInfo  `json:"asr_info,omitempty"`
	TTSInfo   *RealtimeTTSInfo  `json:"tts_info,omitempty"`
	Error     *Error            `json:"error,omitempty"`
}

// RealtimeASRInfo represents ASR information in event
type RealtimeASRInfo struct {
	Text       string      `json:"text"`
	IsFinal    bool        `json:"is_final"`
	Utterances []Utterance `json:"utterances,omitempty"`
}

// RealtimeTTSInfo represents TTS information in event
type RealtimeTTSInfo struct {
	TTSType string `json:"tts_type"`
	Content string `json:"content"`
}

// ================== Implementation ==================

// realtimeService provides realtime conversation operations
// RealtimeService provides real-time speech-to-speech functionality
type RealtimeService struct {
	client *Client
}

// newRealtimeService creates a new realtime conversation service
func newRealtimeService(c *Client) *RealtimeService {
	return &RealtimeService{client: c}
}

// Dial establishes a WebSocket connection to the Realtime dialogue endpoint
//
// This uses the endpoint: WSS /api/v3/realtime/dialogue
// Resource ID: volc.speech.dialog
//
// The connection flow is:
//  1. WebSocket connect
//  2. Send StartConnection (event=1)
//  3. Wait for ConnectionStarted (event=50)
func (s *RealtimeService) Dial(ctx context.Context) (*RealtimeConnection, error) {
	url := s.client.config.wsURL + "/api/v3/realtime/dialogue"
	reqID := generateReqID()

	// Use V2 authentication headers
	headers := s.client.getV2WSHeaders(ResourceRealtime, reqID)
	headers.Set("X-Api-Request-Id", reqID)

	wsConn, _, err := websocket.DefaultDialer.DialContext(ctx, url, headers)
	if err != nil {
		return nil, wrapError(err, "connect websocket")
	}

	conn := &RealtimeConnection{
		conn:      wsConn,
		client:    s.client,
		service:   s,
		proto:     newBinaryProtocol(),
		closeChan: make(chan struct{}),
	}

	// Send StartConnection (event=1)
	startConnMsg := &message{
		msgType: msgTypeFullClient,
		flags:   msgFlagWithEvent,
		event:   1, // StartConnection
		payload: []byte("{}"),
	}
	data, err := conn.proto.marshal(startConnMsg)
	if err != nil {
		wsConn.Close()
		return nil, wrapError(err, "marshal start connection")
	}
	if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		wsConn.Close()
		return nil, wrapError(err, "send start connection")
	}

	// Wait for ConnectionStarted (event=50)
	wsConn.SetReadDeadline(timeFromContext(ctx))
	_, respData, err := wsConn.ReadMessage()
	if err != nil {
		wsConn.Close()
		return nil, wrapError(err, "read connection started")
	}

	respMsg, err := conn.proto.unmarshal(respData)
	if err != nil {
		wsConn.Close()
		return nil, wrapError(err, "parse connection started")
	}

	if respMsg.event != 50 {
		wsConn.Close()
		if respMsg.msgType == msgTypeError {
			return nil, wrapError(fmt.Errorf("event=%d payload=%s", respMsg.event, string(respMsg.payload)), "connection failed")
		}
		return nil, wrapError(fmt.Errorf("expected event=50, got %d", respMsg.event), "unexpected response")
	}

	// Store connect ID
	conn.connectID = respMsg.connectID

	// Clear read deadline
	wsConn.SetReadDeadline(time.Time{})

	// Note: receiveLoop is started after StartSession completes

	return conn, nil
}

// timeFromContext extracts deadline from context, returns zero time if no deadline
func timeFromContext(ctx context.Context) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline
	}
	return time.Now().Add(30 * time.Second) // default timeout
}

// Connect establishes connection and starts a session (convenience method)
func (s *RealtimeService) Connect(ctx context.Context, config *RealtimeConfig) (*RealtimeSession, error) {
	conn, err := s.Dial(ctx)
	if err != nil {
		return nil, err
	}

	session, err := conn.StartSession(ctx, config)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return session, nil
}

func (s *RealtimeService) buildStartRequest(config *RealtimeConfig) map[string]any {
	return map[string]any{
		"type": "start",
		"data": map[string]any{
			"session_id": generateReqID(),
			"config":     s.buildSessionConfig(config),
		},
	}
}

func (s *RealtimeService) buildSessionConfig(config *RealtimeConfig) map[string]any {
	cfg := map[string]any{
		"asr": map[string]any{
			"extra": config.ASR.Extra,
		},
		"tts": map[string]any{
			"speaker": config.TTS.Speaker,
			"audio_config": map[string]any{
				"channel":     config.TTS.AudioConfig.Channel,
				"format":      config.TTS.AudioConfig.Format,
				"sample_rate": config.TTS.AudioConfig.SampleRate,
			},
		},
		"dialog": map[string]any{
			"bot_name":           config.Dialog.BotName,
			"system_role":        config.Dialog.SystemRole,
			"speaking_style":     config.Dialog.SpeakingStyle,
			"character_manifest": config.Dialog.CharacterManifest,
			"extra":              config.Dialog.Extra,
		},
	}

	if config.Dialog.Location != nil {
		cfg["dialog"].(map[string]any)["location"] = map[string]any{
			"longitude":    config.Dialog.Location.Longitude,
			"latitude":     config.Dialog.Location.Latitude,
			"city":         config.Dialog.Location.City,
			"country":      config.Dialog.Location.Country,
			"province":     config.Dialog.Location.Province,
			"district":     config.Dialog.Location.District,
			"town":         config.Dialog.Location.Town,
			"country_code": config.Dialog.Location.CountryCode,
			"address":      config.Dialog.Location.Address,
		}
	}

	return cfg
}

// ================== Connection Implementation ==================

// RealtimeConnection represents an active WebSocket connection to the realtime service
type RealtimeConnection struct {
	conn      *websocket.Conn
	client    *Client
	service   *RealtimeService
	proto     *binaryProtocol
	closeChan chan struct{}
	closeOnce sync.Once
	connectID string // Connection ID from server

	// Current active session
	sessionMu      sync.RWMutex
	currentSession *RealtimeSession
}

// StartSession starts a new session on this connection
//
// The session flow is:
//  1. Send StartSession (event=100) with session config
//  2. Wait for SessionStarted (event=150)
func (c *RealtimeConnection) StartSession(ctx context.Context, config *RealtimeConfig) (*RealtimeSession, error) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()

	// Close active session if exists
	if c.currentSession != nil && !c.currentSession.isClosed() {
		c.currentSession.close()
	}

	// Generate session ID
	sessionID := generateReqID()

	session := &RealtimeSession{
		conn:      c,
		config:    config,
		sessionID: sessionID,
		recvChan:  make(chan *RealtimeEvent, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Build session config payload
	configPayload := c.service.buildSessionConfig(config)
	payload, err := json.Marshal(configPayload)
	if err != nil {
		return nil, wrapError(err, "marshal config")
	}

	// Send StartSession (event=100) with session_id in protocol header
	startMsg := &message{
		msgType:   msgTypeFullClient,
		flags:     msgFlagWithEvent,
		event:     100, // StartSession
		sessionID: sessionID,
		payload:   payload,
	}
	data, err := c.proto.marshal(startMsg)
	if err != nil {
		return nil, wrapError(err, "marshal start session")
	}
	if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return nil, wrapError(err, "send start session")
	}

	// Read response synchronously (before receiveLoop picks it up)
	c.conn.SetReadDeadline(timeFromContext(ctx))
	_, respData, err := c.conn.ReadMessage()
	if err != nil {
		return nil, wrapError(err, "read session response")
	}
	c.conn.SetReadDeadline(time.Time{}) // Clear deadline

	// Parse response
	respMsg, err := c.proto.unmarshal(respData)
	if err != nil {
		return nil, wrapError(err, "parse session response")
	}

	// Check for error response
	if respMsg.isError() {
		return nil, wrapError(fmt.Errorf("code=%d: %s", respMsg.errorCode, string(respMsg.payload)), "session failed")
	}

	// Check for SessionStarted (event=150)
	if respMsg.event != 150 {
		return nil, wrapError(fmt.Errorf("expected event=150, got %d", respMsg.event), "unexpected response")
	}

	// Set as current active session
	c.currentSession = session

	// Start receive loop now that session is established
	go c.receiveLoop()

	return session, nil
}

// Close closes the connection
func (c *RealtimeConnection) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeChan)

		// Close current session
		c.sessionMu.Lock()
		if c.currentSession != nil {
			c.currentSession.close()
			c.currentSession = nil
		}
		c.sessionMu.Unlock()

		c.conn.Close()
	})
	return nil
}

// receiveLoop is the connection-level message receive loop
func (c *RealtimeConnection) receiveLoop() {
	for {
		select {
		case <-c.closeChan:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				c.dispatchError(wrapError(err, "read message"))
			}
			return
		}

		// Parse and dispatch event
		event := c.parseMessage(data)
		if event != nil {
			c.dispatchEvent(event)
		}
	}
}

// dispatchEvent dispatches an event to the current session
func (c *RealtimeConnection) dispatchEvent(event *RealtimeEvent) {
	c.sessionMu.RLock()
	session := c.currentSession
	c.sessionMu.RUnlock()

	if session != nil && !session.isClosed() {
		select {
		case session.recvChan <- event:
		case <-session.closeChan:
		default:
			// Channel full, drop event to avoid blocking
		}
	}
}

// dispatchError dispatches an error to the current session
func (c *RealtimeConnection) dispatchError(err error) {
	c.sessionMu.RLock()
	session := c.currentSession
	c.sessionMu.RUnlock()

	if session != nil && !session.isClosed() {
		select {
		case session.errChan <- err:
		default:
		}
	}
}

// parseMessage parses a message
func (c *RealtimeConnection) parseMessage(data []byte) *RealtimeEvent {
	// Try to parse as binary protocol message
	msg, err := c.proto.unmarshal(data)
	if err == nil {
		// Handle error messages (msgType=15) regardless of flags
		if msg.isError() {
			return &RealtimeEvent{
				Type:      EventSessionFailed,
				SessionID: msg.sessionID,
				Error: &Error{
					Code:    int(msg.errorCode),
					Message: string(msg.payload),
				},
			}
		}
		// Handle event messages
		if msg.flags&msgFlagWithEvent != 0 {
			return c.parseProtocolEvent(msg)
		}
	}

	// Try to parse as JSON message
	return c.parseJSONEvent(data)
}

func (c *RealtimeConnection) parseProtocolEvent(msg *message) *RealtimeEvent {
	event := &RealtimeEvent{
		Type:      RealtimeEventType(msg.event),
		SessionID: msg.sessionID,
	}

	if msg.isAudioOnly() {
		event.Audio = msg.payload
		event.Type = EventAudioReceived
	} else if len(msg.payload) > 0 {
		event.Payload = msg.payload

		// Try to parse info from payload
		var payload struct {
			Text      string `json:"text"`
			Content   string `json:"content"` // ChatResponse uses this field
			SessionID string `json:"session_id"`
			DialogID  string `json:"dialog_id"`
			ASRInfo   *struct {
				Text       string      `json:"text"`
				IsFinal    bool        `json:"is_final"`
				Utterances []Utterance `json:"utterances,omitempty"`
			} `json:"asr_info,omitempty"`
			TTSInfo *struct {
				TTSType string `json:"tts_type"`
				Content string `json:"content"`
				Text    string `json:"text"` // TTSSentenceStart uses text
			} `json:"tts_info,omitempty"`
		}

		if json.Unmarshal(msg.payload, &payload) == nil {
			if payload.SessionID != "" {
				event.SessionID = payload.SessionID
			}
			// Prefer content (ChatResponse) over text
			if payload.Content != "" {
				event.Text = payload.Content
			} else {
				event.Text = payload.Text
			}
			if payload.ASRInfo != nil {
				event.ASRInfo = &RealtimeASRInfo{
					Text:       payload.ASRInfo.Text,
					IsFinal:    payload.ASRInfo.IsFinal,
					Utterances: payload.ASRInfo.Utterances,
				}
			}
			if payload.TTSInfo != nil {
				event.TTSInfo = &RealtimeTTSInfo{
					TTSType: payload.TTSInfo.TTSType,
					Content: payload.TTSInfo.Content,
				}
				// TTSSentenceStart may have text in tts_info.text
				if payload.TTSInfo.Text != "" && event.Text == "" {
					event.Text = payload.TTSInfo.Text
				}
			}
		}
	}

	if msg.isError() {
		event.Error = &Error{
			Code:    int(msg.errorCode),
			Message: string(msg.payload),
		}
	}

	return event
}

func (c *RealtimeConnection) parseJSONEvent(data []byte) *RealtimeEvent {
	var jsonMsg struct {
		Type string `json:"type"`
		Data struct {
			SessionID string `json:"session_id"`
			DialogID  string `json:"dialog_id"`
			Role      string `json:"role"`
			Content   string `json:"content"`
			Text      string `json:"text"`
			IsFinal   bool   `json:"is_final"`
			Audio     string `json:"audio"`
			Sequence  int32  `json:"sequence"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &jsonMsg); err != nil {
		return nil
	}

	event := &RealtimeEvent{
		SessionID: jsonMsg.Data.SessionID,
	}

	switch jsonMsg.Type {
	case "text":
		if jsonMsg.Data.Role == "user" {
			event.Type = EventASRFinished
			event.ASRInfo = &RealtimeASRInfo{
				Text:    jsonMsg.Data.Content,
				IsFinal: jsonMsg.Data.IsFinal,
			}
		} else {
			event.Type = EventTTSStarted
			event.Text = jsonMsg.Data.Content
		}
	case "audio":
		event.Type = EventAudioReceived
		// Audio is base64 encoded
		if jsonMsg.Data.Audio != "" {
			audioData, err := base64.StdEncoding.DecodeString(jsonMsg.Data.Audio)
			if err == nil {
				event.Audio = audioData
			}
		}
	case "status":
		// Map status to event type
		event.Type = EventSessionStarted
	case "error":
		event.Type = EventSessionFailed
		event.Error = &Error{
			Message: jsonMsg.Data.Content,
		}
	default:
		return nil
	}

	return event
}

// writeJSON writes JSON message (thread-safe via gorilla/websocket)
func (c *RealtimeConnection) writeJSON(v any) error {
	return c.conn.WriteJSON(v)
}

// sendBinaryJSON wraps JSON data in binary protocol with event ID and sends it
func (c *RealtimeConnection) sendBinaryJSON(v any) error {
	jsonData, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	// Create message with event=100 (StartSession)
	msg := &message{
		msgType: msgTypeFullClient,
		flags:   msgFlagWithEvent,
		event:   100, // StartSession event
		payload: jsonData,
	}
	data, err := c.proto.marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal binary: %w", err)
	}

	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

// writeMessage writes binary message (thread-safe via gorilla/websocket)
func (c *RealtimeConnection) writeMessage(messageType int, data []byte) error {
	return c.conn.WriteMessage(messageType, data)
}

// ================== Session Implementation ==================

// RealtimeSession represents an active realtime speech-to-speech session
type RealtimeSession struct {
	conn      *RealtimeConnection
	config    *RealtimeConfig
	sessionID string
	dialogID  string
	recvChan  chan *RealtimeEvent
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
	closed    bool
	closedMu  sync.RWMutex
}

func (s *RealtimeSession) SendAudio(ctx context.Context, audio []byte) error {
	if s.isClosed() {
		return wrapError(nil, "session closed")
	}

	// Build audio message with TaskRequest event (200)
	msg := &message{
		msgType:   msgTypeAudioOnlyClient,
		flags:     msgFlagWithEvent,
		event:     200, // TaskRequest event for audio upload
		sessionID: s.sessionID,
		payload:   audio,
	}

	data, err := s.conn.proto.marshal(msg)
	if err != nil {
		return wrapError(err, "marshal audio message")
	}

	return s.conn.writeMessage(websocket.BinaryMessage, data)
}

func (s *RealtimeSession) SendText(ctx context.Context, text string) error {
	if s.isClosed() {
		return wrapError(nil, "session closed")
	}

	payload, _ := json.Marshal(map[string]any{"content": text})
	return s.sendEvent(501, payload) // ChatTextQuery event
}

func (s *RealtimeSession) SayHello(ctx context.Context, content string) error {
	if s.isClosed() {
		return wrapError(nil, "session closed")
	}

	payload, _ := json.Marshal(map[string]any{"content": content})
	return s.sendEvent(300, payload) // SayHello event
}

// sendEvent sends a binary protocol message with the given event ID
func (s *RealtimeSession) sendEvent(eventID int32, payload []byte) error {
	msg := &message{
		msgType:   msgTypeFullClient,
		flags:     msgFlagWithEvent,
		event:     eventID,
		sessionID: s.sessionID,
		payload:   payload,
	}
	data, err := s.conn.proto.marshal(msg)
	if err != nil {
		return wrapError(err, "marshal event")
	}
	return s.conn.writeMessage(websocket.BinaryMessage, data)
}

func (s *RealtimeSession) Interrupt(ctx context.Context) error {
	if s.isClosed() {
		return wrapError(nil, "session closed")
	}

	// FinishSession event (102) to interrupt current response
	return s.sendEvent(102, []byte("{}"))
}

func (s *RealtimeSession) Recv() iter.Seq2[*RealtimeEvent, error] {
	return func(yield func(*RealtimeEvent, error) bool) {
		for {
			select {
			case event, ok := <-s.recvChan:
				if !ok {
					return
				}
				if !yield(event, nil) {
					return
				}
			case err := <-s.errChan:
				yield(nil, err)
				return
			case <-s.closeChan:
				return
			}
		}
	}
}

func (s *RealtimeSession) SessionID() string {
	return s.sessionID
}

func (s *RealtimeSession) DialogID() string {
	return s.dialogID
}

func (s *RealtimeSession) Close() error {
	s.close()
	return nil
}

// close is the internal close method
func (s *RealtimeSession) close() {
	s.closeOnce.Do(func() {
		s.closedMu.Lock()
		s.closed = true
		s.closedMu.Unlock()

		close(s.closeChan)

		// Send finish message (don't close connection)
		finishMsg := map[string]any{
			"type": "finish",
			"data": map[string]any{
				"session_id": s.sessionID,
			},
		}
		s.conn.writeJSON(finishMsg)
	})
}

func (s *RealtimeSession) isClosed() bool {
	s.closedMu.RLock()
	defer s.closedMu.RUnlock()
	return s.closed
}

func (s *RealtimeSession) Connection() *RealtimeConnection {
	return s.conn
}
