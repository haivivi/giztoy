// TTS V2 Service - BigModel TTS (大模型语音合成)
//
// V2 uses /api/v3/* endpoints with X-Api-* headers authentication.
//
// Endpoints:
//   - POST /api/v3/tts/unidirectional - Unidirectional streaming HTTP
//   - WSS  /api/v3/tts/unidirectional - Unidirectional streaming WebSocket
//   - WSS  /api/v3/tts/bidirection    - Bidirectional WebSocket
//   - POST /api/v3/tts/async/submit   - Async long text synthesis
//
// Authentication Headers:
//   - X-Api-App-Id: APP ID
//   - X-Api-Access-Key: Access Token
//   - X-Api-Resource-Id: Resource ID (see below)
//
// Resource IDs:
//   - seed-tts-1.0: 大模型 TTS 1.0 字符版 (需要 *_moon_bigtts 音色)
//   - seed-tts-2.0: 大模型 TTS 2.0 字符版 (需要 *_uranus_bigtts 音色)
//   - seed-tts-1.0-concurr: 大模型 TTS 1.0 并发版
//   - seed-tts-2.0-concurr: 大模型 TTS 2.0 并发版
//
// ⚠️ IMPORTANT: Speaker voice must match Resource ID!
//
//   | Resource ID    | Required Speaker Suffix | Example                              |
//   |----------------|-------------------------|--------------------------------------|
//   | seed-tts-2.0   | *_uranus_bigtts         | zh_female_xiaohe_uranus_bigtts ✅    |
//   | seed-tts-1.0   | *_moon_bigtts           | zh_female_shuangkuaisisi_moon_bigtts |
//
// Common Error:
//   {"code": 55000000, "message": "resource ID is mismatched with speaker related resource"}
//   This means speaker suffix doesn't match resource ID, NOT "service not enabled"!
//
// Documentation: https://www.volcengine.com/docs/6561/1257584
package doubaospeech

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TTSServiceV2 provides BigModel TTS functionality
type TTSServiceV2 struct {
	client *Client
}

func newTTSServiceV2(c *Client) *TTSServiceV2 {
	return &TTSServiceV2{client: c}
}

// TTSV2Request represents a TTS V2 API request
type TTSV2Request struct {
	// Text to synthesize (required)
	Text string `json:"text" yaml:"text"`

	// Speaker voice type (required)
	// Examples: zh_female_cancan, zh_female_shuangkuaisisi_moon_bigtts
	Speaker string `json:"speaker" yaml:"speaker"`

	// Audio parameters
	Format     string `json:"format,omitempty" yaml:"format,omitempty"`           // pcm, mp3, ogg_opus (default: mp3)
	SampleRate int    `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"` // 8000, 16000, 24000, 32000 (default: 24000)
	BitRate    int    `json:"bit_rate,omitempty" yaml:"bit_rate,omitempty"`       // For mp3: 32000, 64000, 128000

	// Speech control
	SpeedRatio  float64 `json:"speed_ratio,omitempty" yaml:"speed_ratio,omitempty"`   // 0.2-3.0, default 1.0
	VolumeRatio float64 `json:"volume_ratio,omitempty" yaml:"volume_ratio,omitempty"` // 0.1-3.0, default 1.0
	PitchRatio  float64 `json:"pitch_ratio,omitempty" yaml:"pitch_ratio,omitempty"`   // 0.1-3.0, default 1.0
	Emotion     string  `json:"emotion,omitempty" yaml:"emotion,omitempty"`           // happy, sad, angry, fear, hate, surprise
	Language    string  `json:"language,omitempty" yaml:"language,omitempty"`         // zh, en, ja, etc.

	// Resource ID (default: seed-tts-2.0)
	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`

	// Mixed speaker for voice mixing
	MixSpeaker *MixSpeakerConfig `json:"mix_speaker,omitempty" yaml:"mix_speaker,omitempty"`
}

// MixSpeakerConfig represents mixed speaker configuration
type MixSpeakerConfig struct {
	SpeakerID  string  `json:"speaker_id"`
	Weight     float64 `json:"weight"`      // 0-1
	VolumeGain float64 `json:"volume_gain"` // -10 to 10 dB
}

// TTSV2Response represents a TTS V2 API response
type TTSV2Response struct {
	Audio    []byte `json:"-"`
	Duration int    `json:"duration"` // milliseconds
	ReqID    string `json:"reqid"`
}

// TTSV2Chunk represents a streaming TTS chunk
type TTSV2Chunk struct {
	Audio   []byte `json:"-"`
	IsLast  bool   `json:"is_last"`
	ReqID   string `json:"reqid"`
	Payload []byte `json:"-"` // Raw payload for debugging
}

// Stream synthesizes speech using streaming HTTP API
//
// This uses the unidirectional streaming endpoint: POST /api/v3/tts/unidirectional
//
// Example:
//
//	for chunk, err := range client.TTSV2.Stream(ctx, req) {
//	    if err != nil {
//	        return err
//	    }
//	    // process chunk.Audio
//	}
func (s *TTSServiceV2) Stream(ctx context.Context, req *TTSV2Request) iter.Seq2[*TTSV2Chunk, error] {
	return func(yield func(*TTSV2Chunk, error) bool) {
		endpoint := s.client.config.baseURL + "/api/v3/tts/unidirectional"

		// Build request body
		body := s.buildRequestBody(req)
		jsonBody, err := json.Marshal(body)
		if err != nil {
			yield(nil, fmt.Errorf("marshal request: %w", err))
			return
		}

		// Create HTTP request
		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
		if err != nil {
			yield(nil, fmt.Errorf("create request: %w", err))
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")

		// Set V2 auth headers
		resourceID := req.ResourceID
		if resourceID == "" {
			resourceID = ResourceTTSV2 // Default to TTS 2.0
		}
		s.client.setV2AuthHeaders(httpReq, resourceID)

		// Send request
		resp, err := s.client.config.httpClient.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("send request: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			yield(nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body)))
			return
		}

		// Parse streaming response (newline-delimited JSON)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunkResp struct {
				ReqID   string `json:"reqid"`
				Data    string `json:"data"`    // base64 encoded audio
				Done    bool   `json:"done"`    // last chunk
				Code    int    `json:"code"`    // error code
				Message string `json:"message"` // error message
			}

			if err := json.Unmarshal(line, &chunkResp); err != nil {
				yield(nil, fmt.Errorf("unmarshal chunk: %w", err))
				return
			}

			// Check for error in response (code 0 or 20000000 is success)
			if chunkResp.Code != 0 && chunkResp.Code != 20000000 {
				yield(nil, &Error{
					Code:    chunkResp.Code,
					Message: chunkResp.Message,
				})
				return
			}

			chunk := &TTSV2Chunk{
				ReqID:  chunkResp.ReqID,
				IsLast: chunkResp.Done,
			}

			if chunkResp.Data != "" {
				// Decode base64 audio data
				audioData, err := base64.StdEncoding.DecodeString(chunkResp.Data)
				if err != nil {
					yield(nil, fmt.Errorf("decode audio data: %w", err))
					return
				}
				chunk.Audio = audioData
			}

			if !yield(chunk, nil) {
				return
			}

			if chunkResp.Done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			yield(nil, fmt.Errorf("read response: %w", err))
		}
	}
}

// buildRequestBody builds the V2 API request body
func (s *TTSServiceV2) buildRequestBody(req *TTSV2Request) map[string]any {
	audioParams := map[string]any{}
	if req.Format != "" {
		audioParams["format"] = req.Format
	}
	if req.SampleRate > 0 {
		audioParams["sample_rate"] = req.SampleRate
	}
	if req.BitRate > 0 {
		audioParams["bit_rate"] = req.BitRate
	}
	if req.SpeedRatio > 0 {
		audioParams["speed_ratio"] = req.SpeedRatio
	}
	if req.VolumeRatio > 0 {
		audioParams["volume_ratio"] = req.VolumeRatio
	}
	if req.PitchRatio > 0 {
		audioParams["pitch_ratio"] = req.PitchRatio
	}
	if req.Emotion != "" {
		audioParams["emotion"] = req.Emotion
	}
	if req.Language != "" {
		audioParams["language"] = req.Language
	}

	body := map[string]any{
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"req_params": map[string]any{
			"text":         req.Text,
			"speaker":      req.Speaker,
			"audio_params": audioParams,
		},
	}

	if req.MixSpeaker != nil {
		body["req_params"].(map[string]any)["mix_speaker"] = map[string]any{
			"speaker_id":  req.MixSpeaker.SpeakerID,
			"weight":      req.MixSpeaker.Weight,
			"volume_gain": req.MixSpeaker.VolumeGain,
		}
	}

	return body
}

// =============================================================================
// WebSocket Bidirectional TTS
// =============================================================================

// TTSV2SessionConfig represents configuration for a bidirectional TTS session
type TTSV2SessionConfig struct {
	// Speaker voice type (required)
	Speaker string `json:"speaker" yaml:"speaker"`

	// Audio format: pcm, mp3, ogg_opus (default: mp3)
	Format string `json:"format,omitempty" yaml:"format,omitempty"`

	// Sample rate: 8000, 16000, 24000, 32000 (default: 24000)
	SampleRate int `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`

	// Speech control
	SpeedRatio  float64 `json:"speed_ratio,omitempty" yaml:"speed_ratio,omitempty"`
	VolumeRatio float64 `json:"volume_ratio,omitempty" yaml:"volume_ratio,omitempty"`
	PitchRatio  float64 `json:"pitch_ratio,omitempty" yaml:"pitch_ratio,omitempty"`
	Emotion     string  `json:"emotion,omitempty" yaml:"emotion,omitempty"`
	Language    string  `json:"language,omitempty" yaml:"language,omitempty"`

	// Resource ID (default: seed-tts-2.0)
	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`
}

// TTSV2Session represents a bidirectional WebSocket TTS session
type TTSV2Session struct {
	conn      *websocket.Conn
	client    *Client
	reqID     string
	sessionID string
	config    *TTSV2SessionConfig

	recvChan  chan *TTSV2Chunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
	sequence  int32
	started   bool
}

// OpenSession opens a bidirectional WebSocket TTS session
//
// This uses the bidirectional streaming endpoint: WSS /api/v3/tts/bidirection
// The config parameter specifies the speaker and audio settings for the session.
//
// The protocol flow is:
// 1. WebSocket connect
// 2. Send StartConnection (event=1)
// 3. Receive ConnectionStarted (event=50)
// 4. Send StartSession (event=100) with speaker/audio config
// 5. Receive SessionStarted (event=150)
// 6. Ready to send TaskRequest (event=200) and receive TTSResponse (event=352)
//
// Example:
//
//	session, err := client.TTSV2.OpenSession(ctx, &TTSV2SessionConfig{
//	    Speaker:    "zh_female_cancan",
//	    Format:     "mp3",
//	    SampleRate: 24000,
//	})
//	if err != nil {
//	    return err
//	}
//	defer session.Close()
//
//	// Send text in chunks
//	session.SendText(ctx, "Hello, ", false)
//	session.SendText(ctx, "world!", true)
//
//	// Receive audio
//	for chunk, err := range session.Recv() {
//	    // process chunk
//	}
func (s *TTSServiceV2) OpenSession(ctx context.Context, config *TTSV2SessionConfig) (*TTSV2Session, error) {
	if config == nil {
		config = &TTSV2SessionConfig{}
	}
	if config.Speaker == "" {
		config.Speaker = "zh_female_cancan"
	}
	if config.ResourceID == "" {
		config.ResourceID = ResourceTTSV2
	}

	endpoint := s.client.config.wsURL + "/api/v3/tts/bidirection"
	connectID := fmt.Sprintf("conn-%d", time.Now().UnixNano())
	sessionID := generateSessionID()

	// Set V2 auth headers
	headers := s.client.getV2WSHeaders(config.ResourceID, connectID)

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("websocket connect failed: %w, status=%s, body=%s", err, resp.Status, string(body))
		}
		return nil, fmt.Errorf("websocket connect failed: %w", err)
	}

	session := &TTSV2Session{
		conn:      conn,
		client:    s.client,
		reqID:     connectID,
		sessionID: sessionID,
		config:    config,
		recvChan:  make(chan *TTSV2Chunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Start receiver goroutine
	go session.receiveLoop()

	// Step 1: Send StartConnection (event=1)
	if err := session.sendStartConnection(); err != nil {
		session.Close()
		return nil, fmt.Errorf("start connection failed: %w", err)
	}

	// Step 2: Wait for ConnectionStarted (event=50)
	select {
	case <-ctx.Done():
		session.Close()
		return nil, ctx.Err()
	case err := <-session.errChan:
		session.Close()
		return nil, fmt.Errorf("connection start failed: %w", err)
	case <-session.recvChan:
		// ConnectionStarted received
	case <-time.After(10 * time.Second):
		session.Close()
		return nil, fmt.Errorf("timeout waiting for ConnectionStarted")
	}

	// Step 3: Send StartSession (event=100)
	if err := session.sendSessionStart(); err != nil {
		session.Close()
		return nil, fmt.Errorf("session start failed: %w", err)
	}

	// Step 4: Wait for SessionStarted (event=150)
	select {
	case <-ctx.Done():
		session.Close()
		return nil, ctx.Err()
	case err := <-session.errChan:
		session.Close()
		return nil, fmt.Errorf("session start failed: %w", err)
	case <-session.recvChan:
		session.started = true
	case <-time.After(10 * time.Second):
		session.Close()
		return nil, fmt.Errorf("timeout waiting for SessionStarted")
	}

	return session, nil
}

// SendText sends text to synthesize
//
// If isLast is true, this marks the end of the text stream and
// automatically sends FinishSession to trigger audio completion.
func (s *TTSV2Session) SendText(ctx context.Context, text string, isLast bool) error {
	s.sequence++
	if err := s.sendTaskRequest(text, isLast); err != nil {
		return err
	}
	// When isLast is true, send FinishSession to trigger audio completion
	if isLast {
		return s.sendFinishSession()
	}
	return nil
}

// Recv returns an iterator for receiving audio chunks
func (s *TTSV2Session) Recv() iter.Seq2[*TTSV2Chunk, error] {
	return func(yield func(*TTSV2Chunk, error) bool) {
		for {
			select {
			case chunk, ok := <-s.recvChan:
				if !ok {
					return
				}
				if !yield(chunk, nil) {
					return
				}
				if chunk.IsLast {
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

// Close closes the session
func (s *TTSV2Session) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		// Send session finish
		s.sendFinishSession()
		s.conn.Close()
	})
	return nil
}

// TTS V2 WebSocket event types (from documentation)
const (
	// Upstream events (client -> server)
	ttsV2EventStartConnection  int32 = 1
	ttsV2EventFinishConnection int32 = 2
	ttsV2EventStartSession     int32 = 100
	ttsV2EventCancelSession    int32 = 101
	ttsV2EventFinishSession    int32 = 102
	ttsV2EventTaskRequest      int32 = 200

	// Downstream events (server -> client)
	ttsV2EventConnectionStarted  int32 = 50
	ttsV2EventConnectionFailed   int32 = 51
	ttsV2EventConnectionFinished int32 = 52
	ttsV2EventSessionStarted     int32 = 150
	ttsV2EventSessionCanceled    int32 = 151
	ttsV2EventSessionFinished    int32 = 152
	ttsV2EventSessionFailed      int32 = 153
	ttsV2EventTTSSentenceStart   int32 = 350
	ttsV2EventTTSSentenceEnd     int32 = 351
	ttsV2EventTTSResponse        int32 = 352
)

// generateSessionID generates a 12-char cryptographically secure session ID
func generateSessionID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 12)
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to time-based if crypto/rand fails (should not happen)
		for i := range b {
			b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		}
		return string(b)
	}
	for i := range b {
		b[i] = chars[int(randomBytes[i])%len(chars)]
	}
	return string(b)
}

func (s *TTSV2Session) sendStartConnection() error {
	// StartConnection payload only needs namespace
	payload := map[string]any{
		"namespace": "BidirectionalTTS",
	}
	return s.sendV2BinaryMessageConnect(ttsV2EventStartConnection, payload)
}

func (s *TTSV2Session) sendSessionStart() error {
	// Build audio params
	audioParams := map[string]any{}
	if s.config.Format != "" {
		audioParams["format"] = s.config.Format
	}
	if s.config.SampleRate > 0 {
		audioParams["sample_rate"] = s.config.SampleRate
	}
	if s.config.SpeedRatio > 0 {
		audioParams["speed_ratio"] = s.config.SpeedRatio
	}
	if s.config.VolumeRatio > 0 {
		audioParams["volume_ratio"] = s.config.VolumeRatio
	}
	if s.config.PitchRatio > 0 {
		audioParams["pitch_ratio"] = s.config.PitchRatio
	}
	if s.config.Emotion != "" {
		audioParams["emotion"] = s.config.Emotion
	}
	if s.config.Language != "" {
		audioParams["language"] = s.config.Language
	}

	// Build session start payload
	// Note: event=100 is included in the JSON payload as well
	payload := map[string]any{
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"event": ttsV2EventStartSession,
		"req_params": map[string]any{
			"speaker":      s.config.Speaker,
			"audio_params": audioParams,
		},
	}

	return s.sendV2BinaryMessage(ttsV2EventStartSession, payload)
}

func (s *TTSV2Session) sendTaskRequest(text string, isLast bool) error {
	// TaskRequest payload format:
	// {
	//   "user": {"uid": "xxx"},
	//   "event": 200,
	//   "req_params": {"text": "xxx"}
	// }
	payload := map[string]any{
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"event": ttsV2EventTaskRequest,
		"req_params": map[string]any{
			"text": text,
		},
	}
	return s.sendV2BinaryMessage(ttsV2EventTaskRequest, payload)
}

func (s *TTSV2Session) sendFinishSession() error {
	return s.sendV2BinaryMessage(ttsV2EventFinishSession, map[string]any{})
}

// sendV2BinaryMessageConnect sends a TTS V2 binary message for Connect-class events
// Connect-class events (StartConnection, FinishConnection) do NOT include connect_id
// The connect_id is returned by the server in ConnectionStarted response
//
// Binary format (from documentation):
// - Header (4 bytes): 0x11 0x14 0x10 0x00
// - Event number (4 bytes): int32(event)
// - Payload length (4 bytes): uint32(len(json))
// - Payload (JSON): {}
func (s *TTSV2Session) sendV2BinaryMessageConnect(event int32, payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	// Header (4 bytes)
	buf.WriteByte(0x11) // Version 1, Header size 1 (4 bytes)
	buf.WriteByte(0x14) // Full-client request (0x1) with event flag (0x4)
	buf.WriteByte(0x10) // JSON serialization, no compression
	buf.WriteByte(0x00) // Reserved

	// Event number (4 bytes, big endian)
	binary.Write(&buf, binary.BigEndian, event)

	// Payload length + Payload (NO connect_id for Connect-class events)
	binary.Write(&buf, binary.BigEndian, uint32(len(jsonData)))
	buf.Write(jsonData)

	return s.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

// sendV2BinaryMessage sends a TTS V2 binary message with event number and session ID
// Session-class events (StartSession, FinishSession, TaskRequest) use session_id
//
// Binary format (from documentation):
// - Byte 0: version (high nibble) | header_size (low nibble) = 0x11
// - Byte 1: message_type (high nibble) | flags (low nibble) = 0x14 (Full-client with event)
// - Byte 2: serialization (high nibble) | compression (low nibble) = 0x10 (JSON, no compression)
// - Byte 3: reserved = 0x00
// - Bytes 4-7: Event number (int32, big endian)
// - Bytes 8-11: Session ID length (uint32, big endian)
// - Bytes 12-n: Session ID (string)
// - Next 4 bytes: Payload length (uint32, big endian)
// - Remaining: Payload (JSON)
func (s *TTSV2Session) sendV2BinaryMessage(event int32, payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	// Header (4 bytes)
	buf.WriteByte(0x11) // Version 1, Header size 1 (4 bytes)
	buf.WriteByte(0x14) // Full-client request (0x1) with event flag (0x4)
	buf.WriteByte(0x10) // JSON serialization, no compression
	buf.WriteByte(0x00) // Reserved

	// Event number (4 bytes, big endian)
	binary.Write(&buf, binary.BigEndian, event)

	// Session ID length + Session ID
	sessionIDBytes := []byte(s.sessionID)
	binary.Write(&buf, binary.BigEndian, uint32(len(sessionIDBytes)))
	buf.Write(sessionIDBytes)

	// Payload length + Payload
	binary.Write(&buf, binary.BigEndian, uint32(len(jsonData)))
	buf.Write(jsonData)

	return s.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

func (s *TTSV2Session) receiveLoop() {
	defer close(s.recvChan)

	for {
		select {
		case <-s.closeChan:
			return
		default:
		}

		wsMsgType, data, err := s.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				select {
				case s.errChan <- err:
				default:
				}
			}
			return
		}

		if wsMsgType != websocket.BinaryMessage || len(data) < 4 {
			continue
		}

		// Parse binary header (TTS V2 protocol)
		// - Byte 0: version (high nibble) | header_size (low nibble)
		// - Byte 1: message_type (high nibble) | flags (low nibble)
		// - Byte 2: serialization (high nibble) | compression (low nibble)
		// - Byte 3: reserved
		msgType := (data[1] >> 4) & 0x0F
		flags := data[1] & 0x0F
		serializationType := (data[2] >> 4) & 0x0F

		offset := 4

		// If flags has 0x4 bit set, there's an event number
		var eventNum int32
		if flags&0x04 != 0 {
			if len(data) < offset+4 {
				continue
			}
			eventNum = int32(binary.BigEndian.Uint32(data[offset : offset+4]))
			offset += 4

			// Connection-class events (50, 51, 52) have connect_id
			// Session-class events (150-153, 350-352) have session_id
			// Both have the same format: length + id
			if eventNum == ttsV2EventConnectionStarted ||
				eventNum == ttsV2EventConnectionFailed ||
				eventNum == ttsV2EventConnectionFinished {
				// Connection-class event: read connect_id length and skip
				if len(data) < offset+4 {
					continue
				}
				connectIDLen := binary.BigEndian.Uint32(data[offset : offset+4])
				offset += 4
				offset += int(connectIDLen) // Skip connect ID
			} else if eventNum >= 150 {
				// Session-class event: read session_id length and skip
				if len(data) < offset+4 {
					continue
				}
				sessionIDLen := binary.BigEndian.Uint32(data[offset : offset+4])
				offset += 4
				offset += int(sessionIDLen) // Skip session ID
			}
		}

		// Read payload size and payload
		if len(data) < offset+4 {
			continue
		}
		payloadSize := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if len(data) < offset+int(payloadSize) {
			continue
		}
		payload := data[offset : offset+int(payloadSize)]

		// Handle different message types
		switch msgType {
		case 0x9: // Full-server response
			switch eventNum {
			case ttsV2EventConnectionStarted: // 50
				// Connection started - signal success
				chunk := &TTSV2Chunk{ReqID: s.reqID}
				select {
				case s.recvChan <- chunk:
				case <-s.closeChan:
					return
				}

			case ttsV2EventSessionStarted: // 150
				// Session started - signal success
				chunk := &TTSV2Chunk{ReqID: s.reqID}
				select {
				case s.recvChan <- chunk:
				case <-s.closeChan:
					return
				}

			case ttsV2EventTTSSentenceStart, ttsV2EventTTSSentenceEnd: // 350, 351
				// Sentence boundary events - just skip or log
				continue

			case ttsV2EventTTSResponse: // 352
				// TTS response with audio data
				if serializationType == 0 {
					// Raw audio data
					chunk := &TTSV2Chunk{
						Audio:   payload,
						ReqID:   s.reqID,
						Payload: payload,
					}
					select {
					case s.recvChan <- chunk:
					case <-s.closeChan:
						return
					}
				} else if serializationType == 1 {
					// JSON response with base64 audio
					var resp struct {
						Data string `json:"data"` // base64 encoded audio
					}
					if json.Unmarshal(payload, &resp) == nil && resp.Data != "" {
						if audioData, err := base64.StdEncoding.DecodeString(resp.Data); err == nil {
							chunk := &TTSV2Chunk{
								Audio: audioData,
								ReqID: s.reqID,
							}
							select {
							case s.recvChan <- chunk:
							case <-s.closeChan:
								return
							}
						}
					}
				}

			case ttsV2EventSessionFinished: // 152
				// Session finished
				chunk := &TTSV2Chunk{
					IsLast: true,
					ReqID:  s.reqID,
				}
				select {
				case s.recvChan <- chunk:
				case <-s.closeChan:
					return
				}
				return

			case ttsV2EventConnectionFailed, ttsV2EventSessionFailed: // 51, 153
				// Connection or session failed
				var errResp struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}
				if json.Unmarshal(payload, &errResp) == nil {
					select {
					case s.errChan <- &Error{Code: errResp.Code, Message: errResp.Message}:
					default:
					}
				}
				return
			}

		case 0xB: // Audio-only response
			chunk := &TTSV2Chunk{
				Audio:   payload,
				ReqID:   s.reqID,
				Payload: payload,
			}
			select {
			case s.recvChan <- chunk:
			case <-s.closeChan:
				return
			}

		case 0xF: // Error
			var errResp struct {
				StatusCode int    `json:"status_code"`
				Code       int    `json:"code"`
				Message    string `json:"message"`
			}
			if json.Unmarshal(payload, &errResp) == nil {
				code := errResp.Code
				if code == 0 {
					code = errResp.StatusCode
				}
				select {
				case s.errChan <- &Error{Code: code, Message: errResp.Message}:
				default:
				}
			}
			return
		}
	}
}
