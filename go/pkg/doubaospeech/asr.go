package doubaospeech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"sync"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"

	"github.com/gorilla/websocket"
)

// asrService ASR 服务实现
type asrService struct {
	client *Client
}

// newASRService 创建 ASR 服务
func newASRService(c *Client) iface.ASRService {
	return &asrService{client: c}
}

// RecognizeOneSentence 一句话识别（ASR 1.0）
func (s *asrService) RecognizeOneSentence(ctx context.Context, req *iface.OneSentenceRequest) (*iface.ASRResult, error) {
	asrReq := s.client.buildASRRequest(string(req.Format))

	// 设置音频数据
	if req.AudioURL != "" {
		asrReq.Audio.URL = req.AudioURL
	} else if req.Audio != nil {
		asrReq.Audio.Data = base64.StdEncoding.EncodeToString(req.Audio)
	} else if req.AudioReader != nil {
		audioData, err := io.ReadAll(req.AudioReader)
		if err != nil {
			return nil, wrapError(err, "read audio data")
		}
		asrReq.Audio.Data = base64.StdEncoding.EncodeToString(audioData)
	}

	if req.SampleRate != 0 {
		asrReq.Audio.SampleRate = int(req.SampleRate)
	}
	if req.Language != "" {
		asrReq.Request.Language = string(req.Language)
	}
	asrReq.Request.EnableITN = req.EnableITN
	asrReq.Request.EnablePunc = req.EnablePunc
	asrReq.Request.EnableDDC = req.EnableDDC

	// 发送请求
	jsonBytes, err := json.Marshal(asrReq)
	if err != nil {
		return nil, wrapError(err, "marshal request")
	}

	url := s.client.config.baseURL + "/api/v1/asr"
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError(err, "read response")
	}

	logID := resp.Header.Get("X-Tt-Logid")

	if resp.StatusCode != http.StatusOK {
		if apiErr := parseAPIError(resp.StatusCode, respBody, logID); apiErr != nil {
			return nil, apiErr
		}
	}

	// 解析响应
	var apiResp struct {
		ReqID   string `json:"reqid"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  struct {
			Text     string `json:"text"`
			Duration int    `json:"duration"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, wrapError(err, "unmarshal response")
	}

	if apiResp.Code != CodeASRSuccess && apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
			ReqID:   apiResp.ReqID,
			LogID:   logID,
		}
	}

	return &iface.ASRResult{
		Text:     apiResp.Result.Text,
		Duration: apiResp.Result.Duration,
	}, nil
}

// OpenStreamSession 打开流式识别会话（ASR 2.0）
func (s *asrService) OpenStreamSession(ctx context.Context, config *iface.StreamASRConfig) (iface.ASRStreamSession, error) {
	url := s.client.config.wsURL + "/api/v2/asr?" + s.client.getWSAuthParams()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, wrapError(err, "connect websocket")
	}

	session := &asrStreamSession{
		conn:      conn,
		client:    s.client,
		config:    config,
		reqID:     generateReqID(),
		recvChan:  make(chan *iface.ASRChunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// 发送开始请求
	startReq := map[string]interface{}{
		"app": map[string]interface{}{
			"appid":   s.client.config.appID,
			"cluster": s.client.config.cluster,
		},
		"user": map[string]interface{}{
			"uid": s.client.config.userID,
		},
		"audio": map[string]interface{}{
			"format":      string(config.Format),
			"sample_rate": int(config.SampleRate),
			"channel":     config.Channel,
			"bits":        config.Bits,
		},
		"request": map[string]interface{}{
			"reqid":           session.reqID,
			"workflow":        "audio_in,resample,partition,vad,fe,decode,itn,nlu_punctuate",
			"show_utterances": config.ShowUtterances,
			"result_type":     "full",
		},
	}

	if config.Language != "" {
		startReq["request"].(map[string]interface{})["language"] = string(config.Language)
	}

	if err := conn.WriteJSON(startReq); err != nil {
		conn.Close()
		return nil, wrapError(err, "send start request")
	}

	// 启动接收协程
	go session.receiveLoop()

	return session, nil
}

// RecognizeFile 文件识别（ASR 2.0）
func (s *asrService) RecognizeFile(ctx context.Context, req *iface.FileASRRequest) (*iface.Task[iface.ASRResult], error) {
	submitReq := &asyncASRSubmitRequest{
		AppID:       s.client.config.appID,
		ReqID:       generateReqID(),
		AudioURL:    req.AudioURL,
		EnableITN:   req.EnableITN,
		EnablePunc:  req.EnablePunc,
	}

	if req.Language != "" {
		submitReq.Language = string(req.Language)
	}
	if req.CallbackURL != "" {
		submitReq.CallbackURL = req.CallbackURL
	}

	var resp asyncTaskResponse
	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/asr/submit", submitReq, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, &Error{
			Code:    resp.Code,
			Message: resp.Message,
			ReqID:   resp.ReqID,
		}
	}

	return newTask[iface.ASRResult](resp.TaskID, s.client, taskTypeASRFile, submitReq.ReqID), nil
}

// ================== 流式识别会话实现 ==================

type asrStreamSession struct {
	conn      *websocket.Conn
	client    *Client
	config    *iface.StreamASRConfig
	reqID     string
	recvChan  chan *iface.ASRChunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
	sequence  int32
}

func (s *asrStreamSession) SendAudio(ctx context.Context, audio []byte, isLast bool) error {
	// 发送音频数据（二进制帧）
	if err := s.conn.WriteMessage(websocket.BinaryMessage, audio); err != nil {
		return wrapError(err, "send audio")
	}

	// 如果是最后一帧，发送结束命令
	if isLast {
		finishReq := map[string]interface{}{
			"request": map[string]interface{}{
				"reqid":   s.reqID,
				"command": "finish",
			},
		}
		if err := s.conn.WriteJSON(finishReq); err != nil {
			return wrapError(err, "send finish command")
		}
	}

	return nil
}

func (s *asrStreamSession) Recv() iter.Seq2[*iface.ASRChunk, error] {
	return func(yield func(*iface.ASRChunk, error) bool) {
		for {
			select {
			case chunk, ok := <-s.recvChan:
				if !ok {
					return
				}
				if !yield(chunk, nil) {
					return
				}
				if chunk.IsFinal {
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

func (s *asrStreamSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

func (s *asrStreamSession) receiveLoop() {
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

		// 解析 JSON 响应
		var resp struct {
			ReqID   string `json:"reqid"`
			Code    int    `json:"code"`
			Message string `json:"message"`
			Result  struct {
				Text       string `json:"text"`
				IsFinal    bool   `json:"is_final"`
				Utterances []struct {
					Text      string `json:"text"`
					StartTime int    `json:"start_time"`
					EndTime   int    `json:"end_time"`
					Words     []struct {
						Text      string `json:"text"`
						StartTime int    `json:"start_time"`
						EndTime   int    `json:"end_time"`
					} `json:"words,omitempty"`
				} `json:"utterances,omitempty"`
			} `json:"result"`
		}

		if err := json.Unmarshal(data, &resp); err != nil {
			// 可能是二进制音频数据，跳过
			continue
		}

		if resp.Code != CodeASRSuccess && resp.Code != 0 {
			select {
			case s.errChan <- &Error{
				Code:    resp.Code,
				Message: resp.Message,
				ReqID:   resp.ReqID,
			}:
			default:
			}
			return
		}

		// 转换 utterances
		var utterances []iface.Utterance
		for _, u := range resp.Result.Utterances {
			utt := iface.Utterance{
				Text:      u.Text,
				StartTime: u.StartTime,
				EndTime:   u.EndTime,
				Definite:  resp.Result.IsFinal,
			}
			for _, w := range u.Words {
				utt.Words = append(utt.Words, iface.Word{
					Text:      w.Text,
					StartTime: w.StartTime,
					EndTime:   w.EndTime,
				})
			}
			utterances = append(utterances, utt)
		}

		s.sequence++
		chunk := &iface.ASRChunk{
			Text:       resp.Result.Text,
			IsDefinite: resp.Result.IsFinal,
			IsFinal:    resp.Result.IsFinal,
			Utterances: utterances,
			Sequence:   s.sequence,
		}

		select {
		case s.recvChan <- chunk:
		case <-s.closeChan:
			return
		}

		if resp.Result.IsFinal {
			return
		}
	}
}

// 注册实现验证
var _ iface.ASRService = (*asrService)(nil)
var _ iface.ASRStreamSession = (*asrStreamSession)(nil)
