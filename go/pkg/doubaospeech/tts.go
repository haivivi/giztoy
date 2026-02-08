package doubaospeech

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

// ttsService provides TTS operations
// TTSService provides text-to-speech synthesis functionality
type TTSService struct {
	client *Client
}

// newTTSService creates TTS service
func newTTSService(c *Client) *TTSService {
	return &TTSService{client: c}
}

// ttsAPIResponse represents TTS API response structure
type ttsAPIResponse struct {
	ReqID    string `json:"reqid"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Data     string `json:"data"`
	Addition struct {
		Duration string `json:"duration"`
	} `json:"addition"`
}

// Synthesize performs synchronous TTS
func (s *TTSService) Synthesize(ctx context.Context, req *TTSRequest) (*TTSResponse, error) {
	ttsReq := s.buildRequest(req)

	var apiResp ttsAPIResponse
	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/tts", ttsReq, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != CodeSuccess {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
			ReqID:   apiResp.ReqID,
		}
	}

	// Decode audio data
	audioData, err := base64.StdEncoding.DecodeString(apiResp.Data)
	if err != nil {
		return nil, wrapError(err, "decode audio data")
	}

	duration, _ := strconv.Atoi(apiResp.Addition.Duration)

	return &TTSResponse{
		Audio:    audioData,
		Duration: duration,
		ReqID:    apiResp.ReqID,
	}, nil
}

// SynthesizeStream performs streaming TTS over HTTP
func (s *TTSService) SynthesizeStream(ctx context.Context, req *TTSRequest) iter.Seq2[*TTSChunk, error] {
	return func(yield func(*TTSChunk, error) bool) {
		ttsReq := s.buildRequest(req)

		jsonBytes, err := json.Marshal(ttsReq)
		if err != nil {
			yield(nil, wrapError(err, "marshal request"))
			return
		}

		url := s.client.config.baseURL + "/api/v1/tts/stream"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBytes))
		if err != nil {
			yield(nil, wrapError(err, "create request"))
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		s.client.setAuthHeaders(httpReq)

		resp, err := s.client.config.httpClient.Do(httpReq)
		if err != nil {
			yield(nil, wrapError(err, "send request"))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			logID := resp.Header.Get("X-Tt-Logid")
			if apiErr := parseAPIError(resp.StatusCode, body, logID); apiErr != nil {
				yield(nil, apiErr)
				return
			}
		}

		// Read streaming response
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				yield(nil, wrapError(err, "read stream"))
				return
			}

			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			var chunk struct {
				ReqID    string `json:"reqid"`
				Code     int    `json:"code"`
				Message  string `json:"message"`
				Sequence int32  `json:"sequence"`
				Data     string `json:"data"`
				Addition struct {
					Duration string `json:"duration"`
				} `json:"addition"`
			}

			if err := json.Unmarshal(line, &chunk); err != nil {
				continue
			}

			if chunk.Code != CodeSuccess {
				yield(nil, &Error{
					Code:    chunk.Code,
					Message: chunk.Message,
					ReqID:   chunk.ReqID,
				})
				return
			}

			isLast := chunk.Sequence < 0

			var audioData []byte
			if chunk.Data != "" {
				var err error
				audioData, err = base64.StdEncoding.DecodeString(chunk.Data)
				if err != nil {
					yield(nil, wrapError(err, "decode audio data"))
					return
				}
			}

			duration := 0
			if chunk.Addition.Duration != "" {
				duration, _ = strconv.Atoi(chunk.Addition.Duration)
			}

			ttsChunk := &TTSChunk{
				Audio:    audioData,
				Sequence: chunk.Sequence,
				IsLast:   isLast,
				Duration: duration,
			}

			if !yield(ttsChunk, nil) {
				return
			}

			if isLast {
				break
			}
		}
	}
}

// SynthesizeStreamWS performs streaming TTS over WebSocket
func (s *TTSService) SynthesizeStreamWS(ctx context.Context, req *TTSRequest) iter.Seq2[*TTSChunk, error] {
	return func(yield func(*TTSChunk, error) bool) {
		url := s.client.config.wsURL + "/api/v1/tts/ws_binary?" + s.client.getWSAuthParams()

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			yield(nil, wrapError(err, "connect websocket"))
			return
		}
		defer conn.Close()

		// Send request
		ttsReq := s.buildRequest(req)
		if err := conn.WriteJSON(ttsReq); err != nil {
			yield(nil, wrapError(err, "send request"))
			return
		}

		// Receive response
		proto := newBinaryProtocol()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					break
				}
				yield(nil, wrapError(err, "read message"))
				return
			}

			msg, err := proto.unmarshal(data)
			if err != nil {
				yield(nil, wrapError(err, "unmarshal message"))
				return
			}

			if msg.isError() {
				yield(nil, &Error{
					Code:    int(msg.errorCode),
					Message: string(msg.payload),
				})
				return
			}

			isLast := msg.sequence < 0
			chunk := &TTSChunk{
				Audio:    msg.payload,
				Sequence: msg.sequence,
				IsLast:   isLast,
			}

			if !yield(chunk, nil) {
				return
			}

			if isLast {
				break
			}
		}
	}
}

// OpenDuplexSession opens duplex streaming session
func (s *TTSService) OpenDuplexSession(ctx context.Context, config *TTSDuplexConfig) (*TTSDuplexSession, error) {
	url := s.client.config.wsURL + "/api/v1/tts/ws_binary?" + s.client.getWSAuthParams()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, wrapError(err, "connect websocket")
	}

	session := &TTSDuplexSession{
		conn:      conn,
		client:    s.client,
		config:    config,
		reqID:     generateReqID(),
		proto:     newBinaryProtocol(),
		recvChan:  make(chan *TTSChunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Start receive loop
	go session.receiveLoop()

	return session, nil
}

// CreateAsyncTask creates async TTS task
func (s *TTSService) CreateAsyncTask(ctx context.Context, req *AsyncTTSRequest) (*Task[TTSAsyncResult], error) {
	submitReq := &asyncTTSSubmitRequest{
		AppID:     s.client.config.appID,
		ReqID:     generateReqID(),
		Text:      req.Text,
		VoiceType: req.VoiceType,
	}

	if req.Encoding != "" {
		submitReq.Format = string(req.Encoding)
	}
	if req.SampleRate != 0 {
		submitReq.SampleRate = int(req.SampleRate)
	}
	if req.SpeedRatio != 0 {
		submitReq.SpeedRatio = req.SpeedRatio
	}
	if req.VolumeRatio != 0 {
		submitReq.VolumeRatio = req.VolumeRatio
	}
	if req.PitchRatio != 0 {
		submitReq.PitchRatio = req.PitchRatio
	}
	if req.CallbackURL != "" {
		submitReq.CallbackURL = req.CallbackURL
	}

	var resp asyncTaskResponse
	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/tts_async/submit", submitReq, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, &Error{
			Code:    resp.Code,
			Message: resp.Message,
			ReqID:   resp.ReqID,
		}
	}

	return newTask[TTSAsyncResult](resp.TaskID, s.client, taskTypeTTSAsync, submitReq.ReqID), nil
}

// TTSAsyncTaskStatus represents async TTS task status
type TTSAsyncTaskStatus struct {
	TaskID        string     `json:"task_id"`
	Status        TaskStatus `json:"status"`
	Progress      int        `json:"progress,omitempty"`
	AudioURL      string     `json:"audio_url,omitempty"`
	AudioDuration int        `json:"audio_duration,omitempty"`
}

// GetAsyncTask queries async TTS task status
//
// Uses flat response format matching queryTaskStatus in task.go,
// since /api/v1/tts_async/query returns fields at the top level
// (not nested under a "data" key like some other V1 endpoints).
//
// Note: queryTaskStatus in task.go uses "reqid" (client-generated UUID) for this
// endpoint, while this method uses "task_id" (server-returned ID) for consistency
// with other service-specific GetTask methods (Podcast, Meeting, Media).
// If the API only recognizes "reqid", this query may fail. This has not been
// verified because V1 TTS async is not granted on the current test account.
func (s *TTSService) GetAsyncTask(ctx context.Context, taskID string) (*TTSAsyncTaskStatus, error) {
	queryReq := map[string]any{
		"appid":   s.client.config.appID,
		"task_id": taskID,
	}

	var apiResp struct {
		Code          int    `json:"code"`
		Message       string `json:"message"`
		TaskID        string `json:"task_id"`
		Status        string `json:"status"`
		Progress      int    `json:"progress,omitempty"`
		AudioURL      string `json:"audio_url,omitempty"`
		AudioDuration int    `json:"audio_duration,omitempty"`
	}

	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/tts_async/query", queryReq, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
		}
	}

	status := &TTSAsyncTaskStatus{
		TaskID:        apiResp.TaskID,
		Progress:      apiResp.Progress,
		AudioURL:      apiResp.AudioURL,
		AudioDuration: apiResp.AudioDuration,
	}

	switch apiResp.Status {
	case "submitted", "pending":
		status.Status = TaskStatusPending
	case "running", "processing":
		status.Status = TaskStatusProcessing
	case "success":
		status.Status = TaskStatusSuccess
	case "failed":
		status.Status = TaskStatusFailed
	case "cancelled":
		status.Status = TaskStatusCancelled
	default:
		status.Status = TaskStatusPending
	}

	return status, nil
}

// buildRequest builds TTS request
func (s *TTSService) buildRequest(req *TTSRequest) *ttsRequest {
	ttsReq := s.client.buildTTSRequest(req.Text, req.VoiceType)

	// Override cluster if specified in request
	if req.Cluster != "" {
		ttsReq.App.Cluster = req.Cluster
	}
	if req.TextType != "" {
		ttsReq.Request.TextType = string(req.TextType)
	}
	if req.Encoding != "" {
		ttsReq.Audio.Encoding = string(req.Encoding)
	}
	if req.SpeedRatio != 0 {
		ttsReq.Audio.SpeedRatio = req.SpeedRatio
	}
	if req.VolumeRatio != 0 {
		ttsReq.Audio.VolumeRatio = req.VolumeRatio
	}
	if req.PitchRatio != 0 {
		ttsReq.Audio.PitchRatio = req.PitchRatio
	}
	if req.Emotion != "" {
		ttsReq.Audio.Emotion = req.Emotion
	}
	if req.Language != "" {
		ttsReq.Audio.Language = string(req.Language)
	}
	if req.SilenceDuration != 0 {
		ttsReq.Request.SilenceDuration = req.SilenceDuration
	}

	return ttsReq
}

// ================== Duplex Session Implementation ==================

// TTSDuplexSession represents an active duplex TTS session
type TTSDuplexSession struct {
	conn      *websocket.Conn
	client    *Client
	config    *TTSDuplexConfig
	reqID     string
	proto     *binaryProtocol
	started   bool
	recvChan  chan *TTSChunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
}

func (s *TTSDuplexSession) SendText(ctx context.Context, text string, isLast bool) error {
	var req map[string]any

	if !s.started {
		// First send, need full request
		req = map[string]any{
			"app": map[string]any{
				"appid":   s.client.config.appID,
				"cluster": s.client.config.cluster,
			},
			"user": map[string]any{
				"uid": s.client.config.userID,
			},
			"audio": map[string]any{
				"voice_type": s.config.VoiceType,
			},
			"request": map[string]any{
				"reqid":     s.reqID,
				"text":      text,
				"text_type": "plain",
				"operation": "submit",
			},
		}

		if s.config.Encoding != "" {
			req["audio"].(map[string]any)["encoding"] = string(s.config.Encoding)
		}
		if s.config.SpeedRatio != 0 {
			req["audio"].(map[string]any)["speed_ratio"] = s.config.SpeedRatio
		}
		if s.config.VolumeRatio != 0 {
			req["audio"].(map[string]any)["volume_ratio"] = s.config.VolumeRatio
		}
		if s.config.PitchRatio != 0 {
			req["audio"].(map[string]any)["pitch_ratio"] = s.config.PitchRatio
		}

		s.started = true
	} else if isLast {
		// End input
		req = map[string]any{
			"request": map[string]any{
				"reqid":     s.reqID,
				"operation": "finish",
			},
		}
	} else {
		// Append text
		req = map[string]any{
			"request": map[string]any{
				"reqid":     s.reqID,
				"text":      text,
				"operation": "append",
			},
		}
	}

	return s.conn.WriteJSON(req)
}

func (s *TTSDuplexSession) Recv() iter.Seq2[*TTSChunk, error] {
	return func(yield func(*TTSChunk, error) bool) {
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

func (s *TTSDuplexSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

func (s *TTSDuplexSession) receiveLoop() {
	defer close(s.recvChan)

	for {
		select {
		case <-s.closeChan:
			return
		default:
		}

		_, data, err := s.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				select {
				case s.errChan <- wrapError(err, "read message"):
				default:
				}
			}
			return
		}

		msg, err := s.proto.unmarshal(data)
		if err != nil {
			select {
			case s.errChan <- wrapError(err, "unmarshal message"):
			default:
			}
			return
		}

		if msg.isError() {
			select {
			case s.errChan <- &Error{
				Code:    int(msg.errorCode),
				Message: string(msg.payload),
			}:
			default:
			}
			return
		}

		isLast := msg.sequence < 0
		chunk := &TTSChunk{
			Audio:    msg.payload,
			Sequence: msg.sequence,
			IsLast:   isLast,
		}

		select {
		case s.recvChan <- chunk:
		case <-s.closeChan:
			return
		}

		if isLast {
			return
		}
	}
}

// ttsResponseToReader converts audio data to io.Reader
func ttsResponseToReader(audio []byte) io.Reader {
	return bytes.NewReader(audio)
}
