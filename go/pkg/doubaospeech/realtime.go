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

// realtimeService 实时对话服务实现
type realtimeService struct {
	client *Client
}

// newRealtimeService 创建实时对话服务
func newRealtimeService(c *Client) iface.RealtimeService {
	return &realtimeService{client: c}
}

// Connect 建立实时对话连接
func (s *realtimeService) Connect(ctx context.Context, config *iface.RealtimeConfig) (iface.RealtimeSession, error) {
	url := s.client.config.wsURL + "/api/v3/saas/chat?" + s.client.getWSAuthParams()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, wrapError(err, "connect websocket")
	}

	session := &realtimeSession{
		conn:      conn,
		client:    s.client,
		config:    config,
		proto:     newBinaryProtocol(),
		recvChan:  make(chan *iface.RealtimeEvent, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// 发送开始请求
	startReq := s.buildStartRequest(config)
	if err := conn.WriteJSON(startReq); err != nil {
		conn.Close()
		return nil, wrapError(err, "send start request")
	}

	// 等待连接确认
	_, data, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, wrapError(err, "read connect response")
	}

	msg, err := session.proto.unmarshal(data)
	if err == nil && msg.flags&msgFlagWithEvent != 0 {
		if msg.event == int32(iface.EventConnectionStarted) {
			session.connectID = msg.connectID
		}

		// 解析会话信息
		if len(msg.payload) > 0 {
			var payload struct {
				SessionID string `json:"session_id"`
				DialogID  string `json:"dialog_id"`
			}
			if json.Unmarshal(msg.payload, &payload) == nil {
				session.sessionID = payload.SessionID
				session.dialogID = payload.DialogID
			}
		}
	}

	// 启动接收协程
	go session.receiveLoop()

	return session, nil
}

func (s *realtimeService) buildStartRequest(config *iface.RealtimeConfig) map[string]interface{} {
	req := map[string]interface{}{
		"type": "start",
		"data": map[string]interface{}{
			"session_id": generateReqID(),
			"config": map[string]interface{}{
				"asr": map[string]interface{}{
					"extra": config.ASR.Extra,
				},
				"tts": map[string]interface{}{
					"speaker": config.TTS.Speaker,
					"audio_config": map[string]interface{}{
						"channel":     config.TTS.AudioConfig.Channel,
						"format":      config.TTS.AudioConfig.Format,
						"sample_rate": config.TTS.AudioConfig.SampleRate,
					},
				},
				"dialog": map[string]interface{}{
					"bot_name":           config.Dialog.BotName,
					"system_role":        config.Dialog.SystemRole,
					"speaking_style":     config.Dialog.SpeakingStyle,
					"character_manifest": config.Dialog.CharacterManifest,
					"extra":              config.Dialog.Extra,
				},
			},
		},
	}

	if config.Dialog.Location != nil {
		req["data"].(map[string]interface{})["config"].(map[string]interface{})["dialog"].(map[string]interface{})["location"] = map[string]interface{}{
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

	return req
}

// ================== 实时对话会话实现 ==================

type realtimeSession struct {
	conn      *websocket.Conn
	client    *Client
	config    *iface.RealtimeConfig
	proto     *binaryProtocol
	sessionID string
	dialogID  string
	connectID string
	recvChan  chan *iface.RealtimeEvent
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
}

func (s *realtimeSession) SendAudio(ctx context.Context, audio []byte) error {
	// 构建音频消息
	msg := &message{
		msgType:   msgTypeAudioOnlyClient,
		flags:     msgFlagWithEvent,
		event:     int32(iface.EventAudioReceived),
		sessionID: s.sessionID,
		payload:   audio,
	}

	data, err := s.proto.marshal(msg)
	if err != nil {
		return wrapError(err, "marshal audio message")
	}

	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (s *realtimeSession) SendText(ctx context.Context, text string) error {
	msg := map[string]interface{}{
		"type": "text",
		"data": map[string]interface{}{
			"session_id": s.sessionID,
			"text":       text,
		},
	}
	return s.conn.WriteJSON(msg)
}

func (s *realtimeSession) SayHello(ctx context.Context, content string) error {
	msg := map[string]interface{}{
		"type": "say_hello",
		"data": map[string]interface{}{
			"session_id": s.sessionID,
			"content":    content,
		},
	}
	return s.conn.WriteJSON(msg)
}

func (s *realtimeSession) Interrupt(ctx context.Context) error {
	msg := map[string]interface{}{
		"type": "cancel",
		"data": map[string]interface{}{
			"session_id": s.sessionID,
		},
	}
	return s.conn.WriteJSON(msg)
}

func (s *realtimeSession) Recv() iter.Seq2[*iface.RealtimeEvent, error] {
	return func(yield func(*iface.RealtimeEvent, error) bool) {
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

func (s *realtimeSession) SessionID() string {
	return s.sessionID
}

func (s *realtimeSession) DialogID() string {
	return s.dialogID
}

func (s *realtimeSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		// 发送结束消息
		finishMsg := map[string]interface{}{
			"type": "finish",
			"data": map[string]interface{}{
				"session_id": s.sessionID,
			},
		}
		s.conn.WriteJSON(finishMsg)
		s.conn.Close()
	})
	return nil
}

func (s *realtimeSession) receiveLoop() {
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

		// 尝试解析为二进制协议消息
		msg, err := s.proto.unmarshal(data)
		if err == nil && msg.flags&msgFlagWithEvent != 0 {
			event := s.parseProtocolEvent(msg)
			if event != nil {
				select {
				case s.recvChan <- event:
				case <-s.closeChan:
					return
				}
			}
			continue
		}

		// 尝试解析为 JSON 消息
		event := s.parseJSONEvent(data)
		if event != nil {
			select {
			case s.recvChan <- event:
			case <-s.closeChan:
				return
			}
		}
	}
}

func (s *realtimeSession) parseProtocolEvent(msg *message) *iface.RealtimeEvent {
	event := &iface.RealtimeEvent{
		Type:      iface.RealtimeEventType(msg.event),
		SessionID: msg.sessionID,
	}

	if msg.isAudioOnly() {
		event.Audio = msg.payload
		event.Type = iface.EventAudioReceived
	} else if len(msg.payload) > 0 {
		event.Payload = msg.payload

		// 尝试解析 payload 中的信息
		var payload struct {
			Text      string `json:"text"`
			SessionID string `json:"session_id"`
			ASRInfo   *struct {
				Text       string             `json:"text"`
				IsFinal    bool               `json:"is_final"`
				Utterances []iface.Utterance  `json:"utterances,omitempty"`
			} `json:"asr_info,omitempty"`
			TTSInfo *struct {
				TTSType string `json:"tts_type"`
				Content string `json:"content"`
			} `json:"tts_info,omitempty"`
		}

		if json.Unmarshal(msg.payload, &payload) == nil {
			event.Text = payload.Text
			if payload.ASRInfo != nil {
				event.ASRInfo = &iface.RealtimeASRInfo{
					Text:       payload.ASRInfo.Text,
					IsFinal:    payload.ASRInfo.IsFinal,
					Utterances: payload.ASRInfo.Utterances,
				}
			}
			if payload.TTSInfo != nil {
				event.TTSInfo = &iface.RealtimeTTSInfo{
					TTSType: payload.TTSInfo.TTSType,
					Content: payload.TTSInfo.Content,
				}
			}
		}
	}

	if msg.isError() {
		event.Error = &iface.Error{
			Code:    int(msg.errorCode),
			Message: string(msg.payload),
		}
	}

	return event
}

func (s *realtimeSession) parseJSONEvent(data []byte) *iface.RealtimeEvent {
	var jsonMsg struct {
		Type string `json:"type"`
		Data struct {
			SessionID string `json:"session_id"`
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

	event := &iface.RealtimeEvent{
		SessionID: jsonMsg.Data.SessionID,
	}

	switch jsonMsg.Type {
	case "text":
		if jsonMsg.Data.Role == "user" {
			event.Type = iface.EventASRFinished
			event.ASRInfo = &iface.RealtimeASRInfo{
				Text:    jsonMsg.Data.Content,
				IsFinal: jsonMsg.Data.IsFinal,
			}
		} else {
			event.Type = iface.EventTTSStarted
			event.Text = jsonMsg.Data.Content
		}
	case "audio":
		event.Type = iface.EventAudioReceived
		// Audio is base64 encoded
		if jsonMsg.Data.Audio != "" {
			audioData, err := base64.StdEncoding.DecodeString(jsonMsg.Data.Audio)
			if err == nil {
				event.Audio = audioData
			}
		}
	case "status":
		// Map status to event type
		event.Type = iface.EventSessionStarted
	case "error":
		event.Type = iface.EventSessionFailed
		event.Error = &iface.Error{
			Message: jsonMsg.Data.Content,
		}
	default:
		return nil
	}

	return event
}

// 注册实现验证
var _ iface.RealtimeService = (*realtimeService)(nil)
var _ iface.RealtimeSession = (*realtimeSession)(nil)
