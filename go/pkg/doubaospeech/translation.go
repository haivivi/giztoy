package doubaospeech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"iter"
	"sync"

	iface "github.com/haivivi/giztoy/pkg/doubao_speech_interface"

	"github.com/gorilla/websocket"
)

// translationService 同声传译服务实现
type translationService struct {
	client *Client
}

// newTranslationService 创建同声传译服务
func newTranslationService(c *Client) iface.TranslationService {
	return &translationService{client: c}
}

// OpenSession 打开同传会话
func (s *translationService) OpenSession(ctx context.Context, config *iface.TranslationConfig) (iface.TranslationSession, error) {
	url := s.client.config.wsURL + "/api/v2/st?" + s.client.getWSAuthParams()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, wrapError(err, "connect websocket")
	}

	session := &translationSession{
		conn:      conn,
		client:    s.client,
		config:    config,
		reqID:     generateReqID(),
		recvChan:  make(chan *iface.TranslationChunk, 100),
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
			"format":      string(config.AudioConfig.Format),
			"sample_rate": int(config.AudioConfig.SampleRate),
			"channel":     config.AudioConfig.Channel,
			"bits":        config.AudioConfig.Bits,
		},
		"request": map[string]interface{}{
			"reqid":           session.reqID,
			"source_language": string(config.SourceLanguage),
			"target_language": string(config.TargetLanguage),
			"enable_asr":      true,
			"enable_tts":      config.EnableTTS,
		},
	}

	if config.EnableTTS && config.TTSVoice != "" {
		startReq["request"].(map[string]interface{})["tts_voice_type"] = config.TTSVoice
	}

	if err := conn.WriteJSON(startReq); err != nil {
		conn.Close()
		return nil, wrapError(err, "send start request")
	}

	// 启动接收协程
	go session.receiveLoop()

	return session, nil
}

// ================== 同传会话实现 ==================

type translationSession struct {
	conn      *websocket.Conn
	client    *Client
	config    *iface.TranslationConfig
	reqID     string
	recvChan  chan *iface.TranslationChunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
	sequence  int32
}

func (s *translationSession) SendAudio(ctx context.Context, audio []byte, isLast bool) error {
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

func (s *translationSession) Recv() iter.Seq2[*iface.TranslationChunk, error] {
	return func(yield func(*iface.TranslationChunk, error) bool) {
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

func (s *translationSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

func (s *translationSession) receiveLoop() {
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
			Type string `json:"type"`
			Data struct {
				Text       string `json:"text"`
				SourceText string `json:"source_text"`
				TargetText string `json:"target_text"`
				IsFinal    bool   `json:"is_final"`
				Audio      string `json:"audio,omitempty"`
				Sequence   int32  `json:"sequence"`
			} `json:"data"`
			Code    int    `json:"code"`
			Message string `json:"message"`
		}

		if err := json.Unmarshal(data, &resp); err != nil {
			// 可能是二进制音频数据，跳过
			continue
		}

		if resp.Code != 0 && resp.Code != CodeASRSuccess {
			select {
			case s.errChan <- &Error{
				Code:    resp.Code,
				Message: resp.Message,
			}:
			default:
			}
			return
		}

		s.sequence++
		chunk := &iface.TranslationChunk{
			SourceText: resp.Data.SourceText,
			TargetText: resp.Data.TargetText,
			IsDefinite: resp.Data.IsFinal,
			IsFinal:    resp.Data.IsFinal,
			Sequence:   s.sequence,
		}

		// 解码音频（如果有）
		if resp.Data.Audio != "" {
			audioData, err := base64.StdEncoding.DecodeString(resp.Data.Audio)
			if err == nil {
				chunk.Audio = audioData
			}
		}

		// 如果 type 是 asr，使用 text 作为 source
		if resp.Type == "asr" {
			chunk.SourceText = resp.Data.Text
		} else if resp.Type == "translation" {
			chunk.SourceText = resp.Data.SourceText
			chunk.TargetText = resp.Data.TargetText
		}

		select {
		case s.recvChan <- chunk:
		case <-s.closeChan:
			return
		}

		if resp.Data.IsFinal {
			return
		}
	}
}

// 注册实现验证
var _ iface.TranslationService = (*translationService)(nil)
var _ iface.TranslationSession = (*translationSession)(nil)
