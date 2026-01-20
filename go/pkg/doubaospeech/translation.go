package doubaospeech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"iter"
	"sync"

	"github.com/gorilla/websocket"
)

// TranslationService represents simultaneous translation service
// translationService provides translation operations
// TranslationService provides real-time speech translation functionality
type TranslationService struct {
	client *Client
}

// newTranslationService creates translation service
func newTranslationService(c *Client) *TranslationService {
	return &TranslationService{client: c}
}

// OpenSession opens translation session
func (s *TranslationService) OpenSession(ctx context.Context, config *TranslationConfig) (*TranslationSession, error) {
	url := s.client.config.wsURL + "/api/v2/st?" + s.client.getWSAuthParams()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, wrapError(err, "connect websocket")
	}

	session := &TranslationSession{
		conn:      conn,
		client:    s.client,
		config:    config,
		reqID:     generateReqID(),
		recvChan:  make(chan *TranslationChunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Send start request
	startReq := map[string]any{
		"app": map[string]any{
			"appid":   s.client.config.appID,
			"cluster": s.client.config.cluster,
		},
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"audio": map[string]any{
			"format":      string(config.AudioConfig.Format),
			"sample_rate": int(config.AudioConfig.SampleRate),
			"channel":     config.AudioConfig.Channel,
			"bits":        config.AudioConfig.Bits,
		},
		"request": map[string]any{
			"reqid":           session.reqID,
			"source_language": string(config.SourceLanguage),
			"target_language": string(config.TargetLanguage),
			"enable_asr":      true,
			"enable_tts":      config.EnableTTS,
		},
	}

	if config.EnableTTS && config.TTSVoice != "" {
		startReq["request"].(map[string]any)["tts_voice_type"] = config.TTSVoice
	}

	if err := conn.WriteJSON(startReq); err != nil {
		conn.Close()
		return nil, wrapError(err, "send start request")
	}

	// Start receive loop
	go session.receiveLoop()

	return session, nil
}

// ================== Translation Session Implementation ==================

// TranslationSession represents an active translation session
type TranslationSession struct {
	conn      *websocket.Conn
	client    *Client
	config    *TranslationConfig
	reqID     string
	recvChan  chan *TranslationChunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
	sequence  int32
}

func (s *TranslationSession) SendAudio(ctx context.Context, audio []byte, isLast bool) error {
	// Send audio data (binary frame)
	if err := s.conn.WriteMessage(websocket.BinaryMessage, audio); err != nil {
		return wrapError(err, "send audio")
	}

	// If last frame, send finish command
	if isLast {
		finishReq := map[string]any{
			"request": map[string]any{
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

func (s *TranslationSession) Recv() iter.Seq2[*TranslationChunk, error] {
	return func(yield func(*TranslationChunk, error) bool) {
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

func (s *TranslationSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

func (s *TranslationSession) receiveLoop() {
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

		// Parse JSON response
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
			// May be binary audio data, skip
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
		chunk := &TranslationChunk{
			SourceText: resp.Data.SourceText,
			TargetText: resp.Data.TargetText,
			IsDefinite: resp.Data.IsFinal,
			IsFinal:    resp.Data.IsFinal,
			Sequence:   s.sequence,
		}

		// Decode audio if present
		if resp.Data.Audio != "" {
			audioData, err := base64.StdEncoding.DecodeString(resp.Data.Audio)
			if err == nil {
				chunk.Audio = audioData
			}
		}

		// If type is asr, use text as source
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
