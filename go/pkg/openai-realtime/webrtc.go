package openairealtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"sync"

	"github.com/pion/webrtc/v3"
)

// WebRTCSession is a WebRTC-based realtime session.
// It provides additional methods for accessing audio tracks.
type WebRTCSession struct {
	pc          *webrtc.PeerConnection
	dc          *webrtc.DataChannel
	config      *ConnectConfig
	client      *Client
	sessionID   string
	closeCh     chan struct{}
	eventsCh    chan eventOrError
	closeOnce   sync.Once
	mu          sync.Mutex
	remoteTrack *webrtc.TrackRemote
	localTrack  *webrtc.TrackLocalStaticSample
}

// ephemeralTokenResponse is the response from the session creation API.
type ephemeralTokenResponse struct {
	ID           string `json:"id"`
	Object       string `json:"object"`
	Model        string `json:"model"`
	ExpiresAt    int64  `json:"expires_at"`
	ClientSecret struct {
		Value     string `json:"value"`
		ExpiresAt int64  `json:"expires_at"`
	} `json:"client_secret"`
}

// connectWebRTC establishes a WebRTC connection.
func (c *Client) connectWebRTC(ctx context.Context, config *ConnectConfig) (*WebRTCSession, error) {
	if config == nil {
		config = &ConnectConfig{}
	}
	if config.Model == "" {
		config.Model = ModelGPT4oRealtimePreview
	}

	// Step 1: Get ephemeral token from OpenAI API
	token, err := c.getEphemeralToken(ctx, config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get ephemeral token: %w", err)
	}

	// Step 2: Create WebRTC peer connection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	session := &WebRTCSession{
		pc:       peerConnection,
		config:   config,
		client:   c,
		closeCh:  make(chan struct{}),
		eventsCh: make(chan eventOrError, 100),
	}

	// Step 3: Add audio transceiver for receiving audio
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		peerConnection.Close()
		return nil, fmt.Errorf("failed to add audio transceiver: %w", err)
	}

	// Step 4: Create data channel for events
	dataChannel, err := peerConnection.CreateDataChannel("oai-events", nil)
	if err != nil {
		peerConnection.Close()
		return nil, fmt.Errorf("failed to create data channel: %w", err)
	}
	session.dc = dataChannel

	// Set up data channel handlers
	dataChannel.OnOpen(func() {
		slog.Debug("data channel opened")
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		event, err := session.parseEvent(msg.Data)
		if err != nil {
			select {
			case <-session.closeCh:
				return
			case session.eventsCh <- eventOrError{err: err}:
			}
			return
		}

		// Track session ID
		if event.Type == EventTypeSessionCreated && event.Session != nil {
			session.mu.Lock()
			session.sessionID = event.Session.ID
			session.mu.Unlock()
		}

		// Check for error event
		if event.Type == EventTypeError && event.TranscriptionError != nil {
			select {
			case <-session.closeCh:
				return
			case session.eventsCh <- eventOrError{err: event.TranscriptionError.ToError()}:
			}
			return
		}

		select {
		case <-session.closeCh:
			return
		case session.eventsCh <- eventOrError{event: event}:
		}
	})

	dataChannel.OnClose(func() {
		slog.Debug("data channel closed")
		close(session.eventsCh)
	})

	// Set up track handler for remote audio
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		slog.Debug("received remote track", "kind", track.Kind(), "codec", track.Codec().MimeType)
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			session.mu.Lock()
			session.remoteTrack = track
			session.mu.Unlock()
		}
	})

	// Step 5: Create offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		peerConnection.Close()
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	// Set local description
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		peerConnection.Close()
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	// Wait for ICE gathering to complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	// Step 6: Send offer to OpenAI and get answer
	answer, err := c.sendOffer(ctx, token, config.Model, peerConnection.LocalDescription().SDP)
	if err != nil {
		peerConnection.Close()
		return nil, fmt.Errorf("failed to send offer: %w", err)
	}

	// Set remote description
	err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer,
	})
	if err != nil {
		peerConnection.Close()
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	return session, nil
}

// getEphemeralToken gets an ephemeral token for WebRTC session.
func (c *Client) getEphemeralToken(ctx context.Context, model string) (string, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"voice": VoiceAlloy, // Default voice
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.httpURL+"/sessions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if c.config.organization != "" {
		req.Header.Set("OpenAI-Organization", c.config.organization)
	}
	if c.config.project != "" {
		req.Header.Set("OpenAI-Project", c.config.project)
	}

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &Error{
			Code:       "session_creation_failed",
			Message:    fmt.Sprintf("failed to create session: %s", string(body)),
			HTTPStatus: resp.StatusCode,
		}
	}

	var tokenResp ephemeralTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.ClientSecret.Value, nil
}

// sendOffer sends the SDP offer to OpenAI and returns the answer.
func (c *Client) sendOffer(ctx context.Context, token, model, sdp string) (string, error) {
	url := fmt.Sprintf("%s?model=%s", c.config.httpURL, model)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte(sdp)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/sdp")

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &Error{
			Code:       "sdp_exchange_failed",
			Message:    fmt.Sprintf("failed to exchange SDP: %s", string(body)),
			HTTPStatus: resp.StatusCode,
		}
	}

	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(answer), nil
}

// UpdateSession updates the session configuration.
func (s *WebRTCSession) UpdateSession(config *SessionConfig) error {
	event := map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeSessionUpdate,
		"session":  config,
	}
	return s.sendEvent(event)
}

// AppendAudio appends PCM audio data to the input audio buffer.
// Note: For WebRTC, prefer using AddAudioTrack for real-time audio streaming.
func (s *WebRTCSession) AppendAudio(audio []byte) error {
	encoded := base64.StdEncoding.EncodeToString(audio)
	return s.AppendAudioBase64(encoded)
}

// AppendAudioBase64 appends base64-encoded audio data to the input buffer.
func (s *WebRTCSession) AppendAudioBase64(audioBase64 string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeInputAudioBufferAppend,
		"audio":    audioBase64,
	})
}

// CommitInput commits the audio buffer.
func (s *WebRTCSession) CommitInput() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeInputAudioBufferCommit,
	})
}

// ClearInput clears the input audio buffer.
func (s *WebRTCSession) ClearInput() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeInputAudioBufferClear,
	})
}

// AddUserMessage adds a user text message to the conversation.
func (s *WebRTCSession) AddUserMessage(text string) error {
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
func (s *WebRTCSession) AddUserAudio(audioBase64 string, transcript string) error {
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
func (s *WebRTCSession) AddAssistantMessage(text string) error {
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
func (s *WebRTCSession) AddFunctionCallOutput(callID string, output string) error {
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
func (s *WebRTCSession) TruncateItem(itemID string, contentIndex int, audioEndMs int) error {
	return s.sendEvent(map[string]interface{}{
		"event_id":      generateEventID(),
		"type":          EventTypeConversationItemTruncate,
		"item_id":       itemID,
		"content_index": contentIndex,
		"audio_end_ms":  audioEndMs,
	})
}

// DeleteItem deletes a conversation item.
func (s *WebRTCSession) DeleteItem(itemID string) error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeConversationItemDelete,
		"item_id":  itemID,
	})
}

// CreateResponse requests the model to generate a response.
func (s *WebRTCSession) CreateResponse(opts *ResponseCreateOptions) error {
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
func (s *WebRTCSession) CancelResponse() error {
	return s.sendEvent(map[string]interface{}{
		"event_id": generateEventID(),
		"type":     EventTypeResponseCancel,
	})
}

// Events returns an iterator over server events.
func (s *WebRTCSession) Events() iter.Seq2[*ServerEvent, error] {
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
func (s *WebRTCSession) SendRaw(event map[string]interface{}) error {
	return s.sendEvent(event)
}

// Close closes the session.
func (s *WebRTCSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closeCh)
		if s.dc != nil {
			s.dc.Close()
		}
		if s.pc != nil {
			err = s.pc.Close()
		}
	})
	return err
}

// SessionID returns the session ID.
func (s *WebRTCSession) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// === WebRTC-specific methods ===

// AudioTrack returns the remote audio track for receiving audio.
// Returns nil if the track has not been received yet.
func (s *WebRTCSession) AudioTrack() *webrtc.TrackRemote {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remoteTrack
}

// AddAudioTrack adds a local audio track for sending audio.
// This is the preferred way to send audio in WebRTC mode.
func (s *WebRTCSession) AddAudioTrack(track *webrtc.TrackLocalStaticSample) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.localTrack != nil {
		return fmt.Errorf("local audio track already added")
	}

	_, err := s.pc.AddTrack(track)
	if err != nil {
		return err
	}
	s.localTrack = track
	return nil
}

// DataChannel returns the data channel used for events.
func (s *WebRTCSession) DataChannel() *webrtc.DataChannel {
	return s.dc
}

// PeerConnection returns the underlying WebRTC peer connection.
func (s *WebRTCSession) PeerConnection() *webrtc.PeerConnection {
	return s.pc
}

// sendEvent sends a JSON event through the data channel.
func (s *WebRTCSession) sendEvent(event map[string]interface{}) error {
	if s.dc == nil || s.dc.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("data channel not ready")
	}

	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		if jsonBytes, err := json.MarshalIndent(event, "", "  "); err == nil {
			str := string(jsonBytes)
			if len(str) > 500 {
				str = str[:500] + "..."
			}
			slog.Debug("sending event", "content", str)
		}
	}

	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return s.dc.Send(jsonBytes)
}

// parseEvent parses a raw JSON message into a ServerEvent.
func (s *WebRTCSession) parseEvent(message []byte) (*ServerEvent, error) {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		msgStr := string(message)
		if len(msgStr) > 1000 {
			msgStr = msgStr[:1000] + "..."
		}
		slog.Debug("received message", "len", len(message), "content", msgStr)
	}

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

// Ensure WebRTCSession implements Session interface.
var _ Session = (*WebRTCSession)(nil)
