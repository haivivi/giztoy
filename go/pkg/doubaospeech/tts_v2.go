// TTS V2 Service - BigModel TTS (大模型语音合成)
//
// V2 uses /api/v3/* endpoints with X-Api-* headers authentication.
//
// Endpoints:
//   - POST /api/v3/tts/unidirectional - Unidirectional streaming HTTP
//   - WSS  /api/v3/tts/bidirection    - Bidirectional WebSocket
//
// Resource IDs:
//   - seed-tts-1.0: 大模型 TTS 1.0 (字符版)
//   - seed-tts-2.0: 大模型 TTS 2.0 (字符版)
//   - seed-tts-1.0-concurr: 大模型 TTS 1.0 (并发版)
//   - seed-tts-2.0-concurr: 大模型 TTS 2.0 (并发版)
//
// Documentation: https://www.volcengine.com/docs/6561/1257584
package doubaospeech

import (
	"bufio"
	"bytes"
	"context"
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

// TTSV2Session represents a bidirectional WebSocket TTS session
type TTSV2Session struct {
	conn      *websocket.Conn
	client    *Client
	reqID     string
	sessionID string
	proto     *binaryProtocol

	recvChan  chan *TTSV2Chunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
	sequence  int32
}

// OpenSession opens a bidirectional WebSocket TTS session
//
// This uses the bidirectional streaming endpoint: WSS /api/v3/tts/bidirection
//
// Example:
//
//	session, err := client.TTSV2.OpenSession(ctx, "seed-tts-2.0")
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
func (s *TTSServiceV2) OpenSession(ctx context.Context, resourceID string) (*TTSV2Session, error) {
	if resourceID == "" {
		resourceID = ResourceTTSV2
	}

	endpoint := s.client.config.wsURL + "/api/v3/tts/bidirection"
	connectID := fmt.Sprintf("conn-%d", time.Now().UnixNano())

	// Set V2 auth headers
	headers := s.client.getV2WSHeaders(resourceID, connectID)

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
		proto:     &binaryProtocol{},
		recvChan:  make(chan *TTSV2Chunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Start receiver goroutine
	go session.receiveLoop()

	// Send session start
	if err := session.sendSessionStart(); err != nil {
		session.Close()
		return nil, fmt.Errorf("session start failed: %w", err)
	}

	return session, nil
}

// SendText sends text to synthesize
//
// If isLast is true, this marks the end of the text stream.
func (s *TTSV2Session) SendText(ctx context.Context, text string, isLast bool) error {
	s.sequence++

	req := map[string]any{
		"event":    ttsV2EventTTSText,
		"sequence": s.sequence,
		"text":     text,
		"is_last":  isLast,
	}

	return s.sendBinaryMessage(req)
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
		s.sendBinaryMessage(map[string]any{"event": ttsV2EventSessionFinish})
		s.conn.Close()
	})
	return nil
}

// TTS V2 WebSocket event types
const (
	ttsV2EventSessionStart  = 1
	ttsV2EventSessionFinish = 2
	ttsV2EventTTSText       = 20
	ttsV2EventTTSAudio      = 30
	ttsV2EventTTSEnd        = 31
	ttsV2EventError         = 255
)

func (s *TTSV2Session) sendSessionStart() error {
	req := map[string]any{
		"event": ttsV2EventSessionStart,
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
	}
	return s.sendBinaryMessage(req)
}

func (s *TTSV2Session) sendBinaryMessage(data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Build binary header (4 bytes)
	// Version(4) | HeaderSize(4) | MsgType(8) | Serialization(4) | Compression(4) | Reserved(8)
	header := uint32(0x10000000) | // Version 1
		uint32(0x01000000) | // Header size 1 (4 bytes)
		uint32(0x00010000) | // Message type: full client request
		uint32(0x00001000) | // Serialization: JSON
		uint32(0x00000000) // No compression

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, header)
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

		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				select {
				case s.errChan <- err:
				default:
				}
			}
			return
		}

		if msgType != websocket.BinaryMessage || len(data) < 8 {
			continue
		}

		// Parse binary header
		header := binary.BigEndian.Uint32(data[0:4])
		payloadSize := binary.BigEndian.Uint32(data[4:8])

		messageType := (header >> 16) & 0xFF

		if int(payloadSize) > len(data)-8 {
			continue
		}

		payload := data[8 : 8+payloadSize]

		switch messageType {
		case ttsV2EventTTSAudio:
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

		case ttsV2EventTTSEnd:
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

		case ttsV2EventError:
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
	}
}
