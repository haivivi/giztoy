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

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"

	"github.com/gorilla/websocket"
)

// ttsService TTS 服务实现
type ttsService struct {
	client *Client
}

// newTTSService 创建 TTS 服务
func newTTSService(c *Client) iface.TTSService {
	return &ttsService{client: c}
}

// Synthesize 同步语音合成
func (s *ttsService) Synthesize(ctx context.Context, req *iface.TTSRequest) (*iface.TTSResponse, error) {
	ttsReq := s.buildRequest(req)

	var respBody []byte
	var logID string

	// 构建请求
	jsonBytes, err := json.Marshal(ttsReq)
	if err != nil {
		return nil, wrapError(err, "marshal request")
	}

	url := s.client.config.baseURL + "/api/v1/tts"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, wrapError(err, "create request")
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.client.setAuthHeaders(httpReq)

	resp, err := s.client.config.httpClient.Do(httpReq)
	if err != nil {
		return nil, wrapError(err, "send request")
	}
	defer resp.Body.Close()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError(err, "read response")
	}

	logID = resp.Header.Get("X-Tt-Logid")

	if resp.StatusCode != http.StatusOK {
		if apiErr := parseAPIError(resp.StatusCode, respBody, logID); apiErr != nil {
			return nil, apiErr
		}
	}

	// 解析响应
	var apiResp struct {
		ReqID    string `json:"reqid"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
		Data     string `json:"data"`
		Addition struct {
			Duration string `json:"duration"`
		} `json:"addition"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, wrapError(err, "unmarshal response")
	}

	if apiResp.Code != CodeSuccess {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
			ReqID:   apiResp.ReqID,
			LogID:   logID,
		}
	}

	// 解码音频数据
	audioData, err := base64.StdEncoding.DecodeString(apiResp.Data)
	if err != nil {
		return nil, wrapError(err, "decode audio data")
	}

	duration, _ := strconv.Atoi(apiResp.Addition.Duration)

	return &iface.TTSResponse{
		Audio:    audioData,
		Duration: duration,
		ReqID:    apiResp.ReqID,
	}, nil
}

// SynthesizeStream 流式语音合成（HTTP）
func (s *ttsService) SynthesizeStream(ctx context.Context, req *iface.TTSRequest) iter.Seq2[*iface.TTSChunk, error] {
	return func(yield func(*iface.TTSChunk, error) bool) {
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

		// 读取流式响应
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
				audioData, _ = base64.StdEncoding.DecodeString(chunk.Data)
			}

			duration := 0
			if chunk.Addition.Duration != "" {
				duration, _ = strconv.Atoi(chunk.Addition.Duration)
			}

			ttsChunk := &iface.TTSChunk{
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

// SynthesizeStreamWS 流式语音合成（WebSocket）
func (s *ttsService) SynthesizeStreamWS(ctx context.Context, req *iface.TTSRequest) iter.Seq2[*iface.TTSChunk, error] {
	return func(yield func(*iface.TTSChunk, error) bool) {
		url := s.client.config.wsURL + "/api/v1/tts/ws_binary?" + s.client.getWSAuthParams()

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			yield(nil, wrapError(err, "connect websocket"))
			return
		}
		defer conn.Close()

		// 发送请求
		ttsReq := s.buildRequest(req)
		if err := conn.WriteJSON(ttsReq); err != nil {
			yield(nil, wrapError(err, "send request"))
			return
		}

		// 接收响应
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
			chunk := &iface.TTSChunk{
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

// OpenDuplexSession 打开双工流式会话
func (s *ttsService) OpenDuplexSession(ctx context.Context, config *iface.TTSDuplexConfig) (iface.TTSDuplexSession, error) {
	url := s.client.config.wsURL + "/api/v1/tts/ws_binary?" + s.client.getWSAuthParams()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, wrapError(err, "connect websocket")
	}

	session := &ttsDuplexSession{
		conn:      conn,
		client:    s.client,
		config:    config,
		reqID:     generateReqID(),
		proto:     newBinaryProtocol(),
		recvChan:  make(chan *iface.TTSChunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// 启动接收协程
	go session.receiveLoop()

	return session, nil
}

// CreateAsyncTask 创建异步合成任务
func (s *ttsService) CreateAsyncTask(ctx context.Context, req *iface.AsyncTTSRequest) (*iface.Task[iface.TTSAsyncResult], error) {
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

	return newTask[iface.TTSAsyncResult](resp.TaskID, s.client, taskTypeTTSAsync, submitReq.ReqID), nil
}

// buildRequest 构建 TTS 请求
func (s *ttsService) buildRequest(req *iface.TTSRequest) *ttsRequest {
	ttsReq := s.client.buildTTSRequest(req.Text, req.VoiceType)

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

// ================== 双工会话实现 ==================

type ttsDuplexSession struct {
	conn       *websocket.Conn
	client     *Client
	config     *iface.TTSDuplexConfig
	reqID      string
	proto      *binaryProtocol
	started    bool
	recvChan   chan *iface.TTSChunk
	errChan    chan error
	closeChan  chan struct{}
	closeOnce  sync.Once
}

func (s *ttsDuplexSession) SendText(ctx context.Context, text string, isLast bool) error {
	var req map[string]interface{}

	if !s.started {
		// 首次发送，需要完整请求
		req = map[string]interface{}{
			"app": map[string]interface{}{
				"appid":   s.client.config.appID,
				"cluster": s.client.config.cluster,
			},
			"user": map[string]interface{}{
				"uid": s.client.config.userID,
			},
			"audio": map[string]interface{}{
				"voice_type": s.config.VoiceType,
			},
			"request": map[string]interface{}{
				"reqid":     s.reqID,
				"text":      text,
				"text_type": "plain",
				"operation": "submit",
			},
		}

		if s.config.Encoding != "" {
			req["audio"].(map[string]interface{})["encoding"] = string(s.config.Encoding)
		}
		if s.config.SpeedRatio != 0 {
			req["audio"].(map[string]interface{})["speed_ratio"] = s.config.SpeedRatio
		}
		if s.config.VolumeRatio != 0 {
			req["audio"].(map[string]interface{})["volume_ratio"] = s.config.VolumeRatio
		}
		if s.config.PitchRatio != 0 {
			req["audio"].(map[string]interface{})["pitch_ratio"] = s.config.PitchRatio
		}

		s.started = true
	} else if isLast {
		// 结束输入
		req = map[string]interface{}{
			"request": map[string]interface{}{
				"reqid":     s.reqID,
				"operation": "finish",
			},
		}
	} else {
		// 追加文本
		req = map[string]interface{}{
			"request": map[string]interface{}{
				"reqid":     s.reqID,
				"text":      text,
				"operation": "append",
			},
		}
	}

	return s.conn.WriteJSON(req)
}

func (s *ttsDuplexSession) Recv() iter.Seq2[*iface.TTSChunk, error] {
	return func(yield func(*iface.TTSChunk, error) bool) {
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

func (s *ttsDuplexSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

func (s *ttsDuplexSession) receiveLoop() {
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
		chunk := &iface.TTSChunk{
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

// TTSResponse 实现 ToReader
func ttsResponseToReader(audio []byte) io.Reader {
	return bytes.NewReader(audio)
}

// 注册实现验证
var _ iface.TTSService = (*ttsService)(nil)
var _ iface.TTSDuplexSession = (*ttsDuplexSession)(nil)
