// ASR V2 Service - BigModel ASR (大模型语音识别)
//
// V2 uses /api/v3/* endpoints with X-Api-* headers authentication.
//
// Endpoints:
//   - WSS /api/v3/sauc/bigmodel       - 流式语音识别 (Streaming ASR)
//   - POST /api/v3/sauc/bigmodel_async - 异步文件识别 (Async File ASR)
//   - WSS /api/v3/asr                  - 录音文件识别 WebSocket
//
// Resource IDs:
//   - volc.bigasr.sauc.duration: 大模型流式语音识别 (时长版)
//   - volc.seedasr.sauc.duration: 大模型流式语音识别 2.0
//   - volc.bigasr.auc.duration: 大模型录音文件识别
//
// Documentation: https://www.volcengine.com/docs/6561/1354868
package doubaospeech

import (
	"bytes"
	"compress/gzip"
	"context"
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

// ASRServiceV2 provides BigModel ASR functionality
type ASRServiceV2 struct {
	client *Client
}

func newASRServiceV2(c *Client) *ASRServiceV2 {
	return &ASRServiceV2{client: c}
}

// ASRV2Config represents ASR V2 streaming configuration
type ASRV2Config struct {
	// Audio format: pcm, wav, mp3, ogg_opus, etc.
	Format string `json:"format" yaml:"format"`

	// Sample rate in Hz (8000, 16000, etc.)
	SampleRate int `json:"sample_rate" yaml:"sample_rate"`

	// Number of audio channels (1 or 2)
	Channels int `json:"channels,omitempty" yaml:"channels,omitempty"`

	// Bits per sample (16, etc.)
	Bits int `json:"bits,omitempty" yaml:"bits,omitempty"`

	// Language: zh-CN, en-US, ja-JP, etc.
	Language string `json:"language,omitempty" yaml:"language,omitempty"`

	// Enable ITN (Inverse Text Normalization)
	EnableITN bool `json:"enable_itn,omitempty" yaml:"enable_itn,omitempty"`

	// Enable punctuation
	EnablePunc bool `json:"enable_punc,omitempty" yaml:"enable_punc,omitempty"`

	// Enable speaker diarization
	EnableDiarization bool `json:"enable_diarization,omitempty" yaml:"enable_diarization,omitempty"`

	// Number of speakers (for diarization)
	SpeakerNum int `json:"speaker_num,omitempty" yaml:"speaker_num,omitempty"`

	// Resource ID (default: volc.bigasr.sauc.duration)
	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`

	// Hotwords for recognition boost
	Hotwords []string `json:"hotwords,omitempty" yaml:"hotwords,omitempty"`
}

// ASRV2Result represents ASR V2 recognition result
type ASRV2Result struct {
	// Recognized text
	Text string `json:"text"`

	// Utterance list with detailed info
	Utterances []ASRV2Utterance `json:"utterances,omitempty"`

	// Is final result (sentence complete)
	IsFinal bool `json:"is_final"`

	// Audio duration in milliseconds
	Duration int `json:"duration,omitempty"`

	// Request ID
	ReqID string `json:"reqid"`
}

// ASRV2Utterance represents a single utterance in ASR result
type ASRV2Utterance struct {
	Text       string       `json:"text"`
	StartTime  int          `json:"start_time"`  // milliseconds
	EndTime    int          `json:"end_time"`    // milliseconds
	Definite   bool         `json:"definite"`    // Whether this utterance is final
	SpeakerID  string       `json:"speaker_id,omitempty"`
	Words      []ASRV2Word  `json:"words,omitempty"`
	Confidence float64      `json:"confidence,omitempty"`
}

// ASRV2Word represents a word in ASR utterance
type ASRV2Word struct {
	Text      string  `json:"text"`
	StartTime int     `json:"start_time"`
	EndTime   int     `json:"end_time"`
	Conf      float64 `json:"conf,omitempty"`
}

// =============================================================================
// Streaming ASR (WebSocket)
// =============================================================================

// ASRV2Session represents a streaming ASR WebSocket session
type ASRV2Session struct {
	conn      *websocket.Conn
	client    *Client
	config    *ASRV2Config
	reqID     string
	proto     *binaryProtocol

	recvChan  chan *ASRV2Result
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
	sequence  int32
}

// OpenStreamSession opens a streaming ASR WebSocket session
//
// This uses the streaming endpoint: WSS /api/v3/sauc/bigmodel
//
// Example:
//
//	session, err := client.ASRV2.OpenStreamSession(ctx, &ASRV2Config{
//	    Format:     "pcm",
//	    SampleRate: 16000,
//	    Language:   "zh-CN",
//	})
//	if err != nil {
//	    return err
//	}
//	defer session.Close()
//
//	// Send audio chunks
//	session.SendAudio(ctx, audioData, false)
//	session.SendAudio(ctx, lastChunk, true)
//
//	// Receive results
//	for result, err := range session.Recv() {
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Println(result.Text)
//	}
func (s *ASRServiceV2) OpenStreamSession(ctx context.Context, config *ASRV2Config) (*ASRV2Session, error) {
	resourceID := config.ResourceID
	if resourceID == "" {
		resourceID = ResourceASRStream
	}

	endpoint := s.client.config.wsURL + "/api/v3/sauc/bigmodel"
	connectID := fmt.Sprintf("asr-%d", time.Now().UnixNano())

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

	session := &ASRV2Session{
		conn:      conn,
		client:    s.client,
		config:    config,
		reqID:     connectID,
		proto:     &binaryProtocol{},
		recvChan:  make(chan *ASRV2Result, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Start receiver goroutine
	go session.receiveLoop()

	// Send session start with config
	if err := session.sendSessionStart(); err != nil {
		session.Close()
		return nil, fmt.Errorf("session start failed: %w", err)
	}

	return session, nil
}

// SendAudio sends audio data to the ASR session
//
// If isLast is true, this marks the end of the audio stream.
func (s *ASRV2Session) SendAudio(ctx context.Context, audio []byte, isLast bool) error {
	s.sequence++

	// Build binary message with audio data per SAUC protocol:
	// Byte 0: version (4 bits) + header_size (4 bits) = 0x11
	// Byte 1: message_type (4 bits) + flags (4 bits) = 0x20 (Audio only) or 0x22 (last audio)
	// Byte 2: serialization (4 bits) + compression (4 bits) = 0x00 (raw bytes, no compression)
	// Byte 3: reserved = 0x00
	header := []byte{0x11, 0x20, 0x00, 0x00}
	if isLast {
		header[1] = 0x22 // Set last audio flag
	}

	var buf bytes.Buffer
	buf.Write(header)
	binary.Write(&buf, binary.BigEndian, uint32(len(audio)))
	buf.Write(audio)

	return s.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

// Recv returns an iterator for receiving ASR results
func (s *ASRV2Session) Recv() iter.Seq2[*ASRV2Result, error] {
	return func(yield func(*ASRV2Result, error) bool) {
		for {
			select {
			case result, ok := <-s.recvChan:
				if !ok {
					return
				}
				if !yield(result, nil) {
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

// Close closes the ASR session
func (s *ASRV2Session) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		// Send session finish
		s.sendSessionFinish()
		s.conn.Close()
	})
	return nil
}

// ASR V2 event types
const (
	asrEventSessionStart  = 1
	asrEventSessionFinish = 2
	asrEventAudioData     = 20
	asrEventResult        = 30
	asrEventFinalResult   = 31
	asrEventError         = 255
)

func (s *ASRV2Session) sendSessionStart() error {
	// SAUC BigModel message format per documentation
	// Reference: docs/doubaospeech/asr2.0/README.md
	audio := map[string]any{
		"format":      s.config.Format,
		"sample_rate": s.config.SampleRate,
		"channel":     1,
		"bits":        16,
	}
	if s.config.Channels > 0 {
		audio["channel"] = s.config.Channels
	}
	if s.config.Bits > 0 {
		audio["bits"] = s.config.Bits
	}

	request := map[string]any{
		"reqid":           s.reqID,
		"sequence":        1,
		"show_utterances": true,
		"result_type":     "single",
	}
	if s.config.Language != "" {
		request["language"] = s.config.Language
	}
	if s.config.EnableITN {
		request["enable_itn"] = true
	}
	if s.config.EnablePunc {
		request["enable_punc"] = true
	}
	if s.config.EnableDiarization {
		request["enable_diarization"] = true
	}
	if len(s.config.Hotwords) > 0 {
		request["hotwords"] = s.config.Hotwords
	}
	if s.config.SpeakerNum > 0 {
		request["speaker_num"] = s.config.SpeakerNum
	}

	req := map[string]any{
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"audio":   audio,
		"request": request,
	}

	return s.sendBinaryMessage(req)
}

func (s *ASRV2Session) sendSessionFinish() error {
	req := map[string]any{
		"event": asrEventSessionFinish,
	}
	return s.sendBinaryMessage(req)
}

func (s *ASRV2Session) sendBinaryMessage(data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Build binary header per SAUC protocol:
	// Byte 0: version (4 bits) + header_size (4 bits) = 0x11
	// Byte 1: message_type (4 bits) + flags (4 bits) = 0x10 (Full client request)
	// Byte 2: serialization (4 bits) + compression (4 bits) = 0x10 (JSON, no compression)
	// Byte 3: reserved = 0x00
	header := []byte{0x11, 0x10, 0x10, 0x00}

	var buf bytes.Buffer
	buf.Write(header)
	binary.Write(&buf, binary.BigEndian, uint32(len(jsonData)))
	buf.Write(jsonData)

	return s.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

func (s *ASRV2Session) receiveLoop() {
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
				case s.errChan <- fmt.Errorf("ws read: %w", err):
				default:
				}
			}
			return
		}

		// Log raw message for debugging
		if len(data) > 0 && len(data) < 1000 {
			// Try to decode as text message for error responses
			if msgType == websocket.TextMessage {
				select {
				case s.errChan <- fmt.Errorf("server text response: %s", string(data)):
				default:
				}
				return
			}
		}

		if msgType != websocket.BinaryMessage || len(data) < 12 {
			continue
		}
		
		// Parse binary header per SAUC protocol:
		// Byte 0: version (4 bits) + header_size (4 bits) = 0x11
		// Byte 1: message_type (4 bits) + flags (4 bits) = e.g. 0x91 (type=9, flags=1)
		// Byte 2: serialization (4 bits) + compression (4 bits) = e.g. 0x10 (JSON, no compression)
		// Byte 3: reserved = 0x00
		// Byte 4-7: sequence number (4 bytes, big-endian)
		// Byte 8-11: payload size (4 bytes, big-endian)
		// Byte 12+: payload
		messageType := (data[1] >> 4) & 0x0F
		messageFlags := data[1] & 0x0F
		compression := data[2] & 0x0F
		payloadSize := binary.BigEndian.Uint32(data[8:12])

		if int(payloadSize) > len(data)-12 {
			continue
		}

		payload := data[12 : 12+payloadSize]
		
		// Decompress if needed
		if compression == 1 { // Gzip
			reader, err := gzip.NewReader(bytes.NewReader(payload))
			if err != nil {
				continue
			}
			payload, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				continue
			}
		}

		// SAUC response format:
		// message_type = 9 (Full server response)
		// flags = 1 (interim), 3 (final)
		if messageType == 9 { // Full server response
			var resp struct {
				AudioInfo struct {
					Duration int `json:"duration"`
				} `json:"audio_info"`
				Result struct {
					Text       string `json:"text"`
					Utterances []struct {
						Text      string `json:"text"`
						StartTime int    `json:"start_time"`
						EndTime   int    `json:"end_time"`
						Definite  bool   `json:"definite"`
						Words     []struct {
							Text      string `json:"text"`
							StartTime int    `json:"start_time"`
							EndTime   int    `json:"end_time"`
						} `json:"words"`
					} `json:"utterances"`
				} `json:"result"`
			}
			if err := json.Unmarshal(payload, &resp); err != nil {
				continue
			}
			
			// Check if this is the final result
			isFinal := messageFlags == 3
			for _, u := range resp.Result.Utterances {
				if u.Definite {
					isFinal = true
					break
				}
			}
			
			// Convert utterances
			var utterances []ASRV2Utterance
			for _, u := range resp.Result.Utterances {
				utt := ASRV2Utterance{
					Text:      u.Text,
					StartTime: u.StartTime,
					EndTime:   u.EndTime,
					Definite:  u.Definite,
				}
				for _, w := range u.Words {
					utt.Words = append(utt.Words, ASRV2Word{
						Text:      w.Text,
						StartTime: w.StartTime,
						EndTime:   w.EndTime,
					})
				}
				utterances = append(utterances, utt)
			}
			
			result := &ASRV2Result{
				Text:       resp.Result.Text,
				Utterances: utterances,
				Duration:   resp.AudioInfo.Duration,
				IsFinal:    isFinal,
				ReqID:      s.reqID,
			}
			
			select {
			case s.recvChan <- result:
			case <-s.closeChan:
				return
			}
			
			if isFinal {
				return
			}
		} else if messageType == 15 { // Error response
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

// =============================================================================
// Async File ASR
// =============================================================================

// ASRV2AsyncRequest represents an async file ASR request
type ASRV2AsyncRequest struct {
	// Audio URL (required, one of AudioURL or AudioData)
	AudioURL string `json:"audio_url,omitempty" yaml:"audio_url,omitempty"`

	// Audio file data (base64 encoded, one of AudioURL or AudioData)
	AudioData []byte `json:"-" yaml:"-"`

	// Audio format
	Format string `json:"format" yaml:"format"`

	// Language
	Language string `json:"language,omitempty" yaml:"language,omitempty"`

	// Enable ITN
	EnableITN bool `json:"enable_itn,omitempty" yaml:"enable_itn,omitempty"`

	// Enable punctuation
	EnablePunc bool `json:"enable_punc,omitempty" yaml:"enable_punc,omitempty"`

	// Enable speaker diarization
	EnableDiarization bool `json:"enable_diarization,omitempty" yaml:"enable_diarization,omitempty"`

	// Number of speakers
	SpeakerNum int `json:"speaker_num,omitempty" yaml:"speaker_num,omitempty"`

	// Callback URL for result notification
	CallbackURL string `json:"callback_url,omitempty" yaml:"callback_url,omitempty"`

	// Resource ID
	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`
}

// ASRV2AsyncResult represents the result of async file ASR
type ASRV2AsyncResult struct {
	// Task ID for querying status
	TaskID string `json:"task_id"`

	// Recognition result (when complete)
	Text string `json:"text,omitempty"`

	// Detailed utterances
	Utterances []ASRV2Utterance `json:"utterances,omitempty"`

	// Task status: pending, processing, success, failed
	Status string `json:"status"`

	// Error message (if failed)
	Error string `json:"error,omitempty"`

	// Request ID
	ReqID string `json:"reqid"`
}

// SubmitAsync submits an async file ASR task
//
// This uses the async endpoint: POST /api/v3/sauc/bigmodel_async
//
// Example:
//
//	result, err := client.ASRV2.SubmitAsync(ctx, &ASRV2AsyncRequest{
//	    AudioURL: "https://example.com/audio.mp3",
//	    Format:   "mp3",
//	    Language: "zh-CN",
//	})
//	if err != nil {
//	    return err
//	}
//	fmt.Println("Task ID:", result.TaskID)
func (s *ASRServiceV2) SubmitAsync(ctx context.Context, req *ASRV2AsyncRequest) (*ASRV2AsyncResult, error) {
	endpoint := s.client.config.baseURL + "/api/v3/sauc/bigmodel_async"

	// Build request body
	body := map[string]any{
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"audio": map[string]any{
			"format": req.Format,
		},
		"req_params": map[string]any{
			"language":           req.Language,
			"enable_itn":         req.EnableITN,
			"enable_punc":        req.EnablePunc,
			"enable_diarization": req.EnableDiarization,
		},
	}

	if req.AudioURL != "" {
		body["audio"].(map[string]any)["url"] = req.AudioURL
	}
	if req.CallbackURL != "" {
		body["callback_url"] = req.CallbackURL
	}
	if req.SpeakerNum > 0 {
		body["req_params"].(map[string]any)["speaker_num"] = req.SpeakerNum
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resourceID := req.ResourceID
	if resourceID == "" {
		resourceID = ResourceASRFile
	}
	s.client.setV2AuthHeaders(httpReq, resourceID)

	resp, err := s.client.config.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var result ASRV2AsyncResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// QueryAsync queries the status of an async ASR task
func (s *ASRServiceV2) QueryAsync(ctx context.Context, taskID string) (*ASRV2AsyncResult, error) {
	endpoint := s.client.config.baseURL + "/api/v3/sauc/bigmodel_async/" + taskID

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	s.client.setV2AuthHeaders(httpReq, ResourceASRFile)

	resp, err := s.client.config.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var result ASRV2AsyncResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
