// Podcast Service - Multi-speaker Podcast Synthesis
//
// Supports multiple API versions:
//
// V1 API (Async HTTP):
//   - POST /api/v1/podcast/submit - Submit async task
//   - POST /api/v1/podcast/query  - Query task status
//   - Auth: Authorization: Bearer {token}
//
// V3 TTS Podcast API (WebSocket):
//   - WSS /api/v3/tts/podcast - TTS Podcast streaming
//   - Documentation: https://www.volcengine.com/docs/6561/1356830
//   - Auth: URL params (?appid=xxx&token=xxx&cluster=xxx)
//   - Note: May require special permissions
//
// V3 SAMI Podcast API (WebSocket) - Recommended:
//   - WSS /api/v3/sami/podcasttts - SAMI Podcast streaming
//   - Documentation: https://www.volcengine.com/docs/6561/1668014
//   - Resource ID: volc.service_type.10050
//   - Auth Headers:
//     - X-Api-App-Id: APP ID
//     - X-Api-Access-Key: Access Token
//     - X-Api-Resource-Id: volc.service_type.10050
//     - X-Api-App-Key: aGjiRDfUWi (固定值)
//
// ⚠️ IMPORTANT: SAMI Podcast requires specific speaker voices!
//
//   | Speaker                                    | Description    |
//   |--------------------------------------------|----------------|
//   | zh_male_dayixiansheng_v2_saturn_bigtts     | 男声-大一先生  |
//   | zh_female_mizaitongxue_v2_saturn_bigtts    | 女声-米仔同学  |
//   | zh_male_liufei_v2_saturn_bigtts            | 男声-刘飞      |
//   | zh_male_xiaolei_v2_saturn_bigtts           | 男声-小雷      |
//
// Note: SAMI Podcast speakers have "_v2_saturn_bigtts" suffix
package doubaospeech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// PodcastService represents podcast synthesis service
// podcastService provides podcast operations
// PodcastService provides podcast synthesis functionality
type PodcastService struct {
	client *Client
}

// newPodcastService creates podcast service
func newPodcastService(c *Client) *PodcastService {
	return &PodcastService{client: c}
}

// CreateTask creates podcast synthesis task
func (s *PodcastService) CreateTask(ctx context.Context, req *PodcastTaskRequest) (*Task[PodcastResult], error) {
	// Build dialogue list
	dialogues := make([]map[string]any, len(req.Script))
	for i, line := range req.Script {
		d := map[string]any{
			"speaker": line.SpeakerID,
			"text":    line.Text,
		}
		if line.Emotion != "" {
			d["emotion"] = line.Emotion
		}
		if line.SpeedRatio != 0 {
			d["speed_ratio"] = line.SpeedRatio
		}
		dialogues[i] = d
	}

	submitReq := map[string]any{
		"app": map[string]any{
			"appid":   s.client.config.appID,
			"cluster": s.client.config.cluster,
		},
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"request": map[string]any{
			"reqid":     generateReqID(),
			"dialogues": dialogues,
		},
	}

	if req.Encoding != "" {
		submitReq["audio"] = map[string]any{
			"encoding": string(req.Encoding),
		}
		if req.SampleRate != 0 {
			submitReq["audio"].(map[string]any)["sample_rate"] = int(req.SampleRate)
		}
	}

	if req.CallbackURL != "" {
		submitReq["request"].(map[string]any)["callback_url"] = req.CallbackURL
	}

	var resp asyncTaskResponse
	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/podcast/submit", submitReq, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, &Error{
			Code:    resp.Code,
			Message: resp.Message,
			ReqID:   resp.ReqID,
		}
	}

	return newTask[PodcastResult](resp.TaskID, s.client, taskTypePodcast, submitReq["request"].(map[string]any)["reqid"].(string)), nil
}

// GetTask queries task status
func (s *PodcastService) GetTask(ctx context.Context, taskID string) (*PodcastTaskStatus, error) {
	queryReq := map[string]any{
		"appid":   s.client.config.appID,
		"task_id": taskID,
	}

	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID   string `json:"task_id"`
			Status   string `json:"status"`
			Progress int    `json:"progress,omitempty"`
			AudioURL string `json:"audio_url,omitempty"`
			Duration int    `json:"duration,omitempty"`
		} `json:"data"`
	}

	if err := s.client.doJSONRequest(ctx, http.MethodPost, "/api/v1/podcast/query", queryReq, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, &Error{
			Code:    apiResp.Code,
			Message: apiResp.Message,
		}
	}

	status := &PodcastTaskStatus{
		TaskID:   apiResp.Data.TaskID,
		Progress: apiResp.Data.Progress,
	}

	// Convert status
	switch apiResp.Data.Status {
	case "submitted", "pending":
		status.Status = TaskStatusPending
	case "running", "processing":
		status.Status = TaskStatusProcessing
	case "success":
		status.Status = TaskStatusSuccess
		status.Result = &PodcastResult{
			AudioURL: apiResp.Data.AudioURL,
			Duration: apiResp.Data.Duration,
		}
	case "failed":
		status.Status = TaskStatusFailed
	default:
		status.Status = TaskStatusPending
	}

	return status, nil
}

// =============================================================================
// V3 TTS Podcast WebSocket API
// Endpoint: WSS /api/v3/tts/podcast
// Documentation: https://www.volcengine.com/docs/6561/1356830
// =============================================================================

// PodcastSpeaker represents a speaker configuration for TTS Podcast
type PodcastSpeaker struct {
	Name        string  `json:"name"`                    // Speaker name (used to match dialogues)
	VoiceType   string  `json:"voice_type"`              // Voice ID
	SpeedRatio  float64 `json:"speed_ratio,omitempty"`   // Speech speed
	VolumeRatio float64 `json:"volume_ratio,omitempty"`  // Volume
}

// PodcastDialogueLine represents a single dialogue line for TTS Podcast
type PodcastDialogueLine struct {
	Speaker string `json:"speaker"`           // Speaker name (must match one in speakers array)
	Text    string `json:"text"`              // Dialogue content
	Emotion string `json:"emotion,omitempty"` // Emotion
}

// PodcastStreamRequest represents a TTS Podcast streaming request
type PodcastStreamRequest struct {
	// Speakers configuration (required)
	Speakers []PodcastSpeaker `json:"speakers" yaml:"speakers"`

	// Dialogues content (required)
	Dialogues []PodcastDialogueLine `json:"dialogues" yaml:"dialogues"`

	// Audio format: mp3, pcm, ogg_opus
	Encoding AudioEncoding `json:"encoding,omitempty" yaml:"encoding,omitempty"`

	// Sample rate: 8000, 16000, 24000, 32000
	SampleRate int `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`
}

// PodcastStreamChunk represents a streaming audio chunk from TTS Podcast
type PodcastStreamChunk struct {
	ReqID         string `json:"reqid"`
	Code          int    `json:"code"`
	Message       string `json:"message,omitempty"`
	Sequence      int    `json:"sequence"`
	Audio         []byte `json:"-"`
	Speaker       string `json:"speaker,omitempty"`
	DialogueIndex int    `json:"dialogue_index,omitempty"`
	Duration      int    `json:"duration,omitempty"`      // Total duration (in last chunk)
	IsLast        bool   `json:"is_last"`
}

// PodcastStreamSession represents a TTS Podcast WebSocket session
type PodcastStreamSession struct {
	conn      *websocket.Conn
	client    *Client
	reqID     string

	recvChan  chan *PodcastStreamChunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
}

// Stream opens a TTS Podcast WebSocket session for real-time podcast synthesis
//
// This uses the TTS Podcast endpoint: WSS /api/v3/tts/podcast
// Auth via URL params: ?appid=xxx&token=xxx&cluster=xxx
//
// Example:
//
//	session, err := client.Podcast.Stream(ctx, &PodcastStreamRequest{
//	    Speakers: []PodcastSpeaker{
//	        {Name: "主持人A", VoiceType: "zh_male_yangguang"},
//	        {Name: "主持人B", VoiceType: "zh_female_cancan"},
//	    },
//	    Dialogues: []PodcastDialogueLine{
//	        {Speaker: "主持人A", Text: "大家好，欢迎收听今天的节目。"},
//	        {Speaker: "主持人B", Text: "是的，今天我们要聊的话题非常有趣。"},
//	    },
//	    Encoding:   EncodingMP3,
//	    SampleRate: SampleRate24000,
//	})
//	if err != nil {
//	    return err
//	}
//	defer session.Close()
//
//	for chunk, err := range session.Recv() {
//	    // process chunk.Audio
//	}
func (s *PodcastService) Stream(ctx context.Context, req *PodcastStreamRequest) (*PodcastStreamSession, error) {
	reqID := generateReqID()

	// Build WebSocket URL with auth params
	endpoint := s.client.config.wsURL + "/api/v3/tts/podcast"
	endpoint += "?appid=" + s.client.config.appID
	if s.client.config.accessToken != "" {
		endpoint += "&token=" + s.client.config.accessToken
	}
	if s.client.config.cluster != "" {
		endpoint += "&cluster=" + s.client.config.cluster
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("websocket connect failed: %w, status=%s, body=%s", err, resp.Status, string(body))
		}
		return nil, fmt.Errorf("websocket connect failed: %w", err)
	}

	session := &PodcastStreamSession{
		conn:      conn,
		client:    s.client,
		reqID:     reqID,
		recvChan:  make(chan *PodcastStreamChunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Start receiver goroutine
	go session.receiveLoop()

	// Send the podcast request
	if err := session.sendRequest(req, reqID); err != nil {
		session.Close()
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return session, nil
}

func (s *PodcastStreamSession) sendRequest(req *PodcastStreamRequest, reqID string) error {
	// Build speakers array
	speakers := make([]map[string]any, len(req.Speakers))
	for i, sp := range req.Speakers {
		speaker := map[string]any{
			"name":       sp.Name,
			"voice_type": sp.VoiceType,
		}
		if sp.SpeedRatio != 0 {
			speaker["speed_ratio"] = sp.SpeedRatio
		}
		if sp.VolumeRatio != 0 {
			speaker["volume_ratio"] = sp.VolumeRatio
		}
		speakers[i] = speaker
	}

	// Build dialogues array
	dialogues := make([]map[string]any, len(req.Dialogues))
	for i, d := range req.Dialogues {
		dialogue := map[string]any{
			"speaker": d.Speaker,
			"text":    d.Text,
		}
		if d.Emotion != "" {
			dialogue["emotion"] = d.Emotion
		}
		dialogues[i] = dialogue
	}

	// Build full request
	fullReq := map[string]any{
		"app": map[string]any{
			"appid":   s.client.config.appID,
			"cluster": s.client.config.cluster,
		},
		"user": map[string]any{
			"uid": s.client.config.userID,
		},
		"request": map[string]any{
			"reqid":     reqID,
			"speakers":  speakers,
			"dialogues": dialogues,
		},
	}

	// Add audio config if specified
	if req.Encoding != "" || req.SampleRate != 0 {
		audio := map[string]any{}
		if req.Encoding != "" {
			audio["encoding"] = string(req.Encoding)
		}
		if req.SampleRate != 0 {
			audio["sample_rate"] = int(req.SampleRate)
		}
		fullReq["audio"] = audio
	}

	return s.conn.WriteJSON(fullReq)
}

func (s *PodcastStreamSession) receiveLoop() {
	defer close(s.recvChan)

	for {
		select {
		case <-s.closeChan:
			return
		default:
		}

		_, data, err := s.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				select {
				case s.errChan <- err:
				default:
				}
			}
			return
		}

		var resp struct {
			ReqID    string `json:"reqid"`
			Code     int    `json:"code"`
			Message  string `json:"message"`
			Sequence int    `json:"sequence"`
			Data     string `json:"data"` // Base64 encoded audio
			Speaker  string `json:"speaker"`
			DialogueIndex int `json:"dialogue_index"`
			Addition struct {
				Duration       string `json:"duration"`
				TotalDialogues int    `json:"total_dialogues"`
			} `json:"addition"`
		}

		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}

		// Check for error
		if resp.Code != 0 && resp.Code != 3000 {
			select {
			case s.errChan <- &Error{Code: resp.Code, Message: resp.Message, ReqID: resp.ReqID}:
			default:
			}
			return
		}

		chunk := &PodcastStreamChunk{
			ReqID:         resp.ReqID,
			Code:          resp.Code,
			Message:       resp.Message,
			Sequence:      resp.Sequence,
			Speaker:       resp.Speaker,
			DialogueIndex: resp.DialogueIndex,
			IsLast:        resp.Sequence == -1,
		}

		// Decode base64 audio data
		if resp.Data != "" {
			audioData, err := base64.StdEncoding.DecodeString(resp.Data)
			if err == nil {
				chunk.Audio = audioData
			}
		}

		// Parse duration from addition (last chunk)
		if resp.Addition.Duration != "" {
			if d, err := strconv.Atoi(resp.Addition.Duration); err == nil {
				chunk.Duration = d
			}
		}

		select {
		case s.recvChan <- chunk:
		case <-s.closeChan:
			return
		}

		if chunk.IsLast {
			return
		}
	}
}

// Recv returns an iterator for receiving TTS Podcast audio chunks
func (s *PodcastStreamSession) Recv() iter.Seq2[*PodcastStreamChunk, error] {
	return func(yield func(*PodcastStreamChunk, error) bool) {
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

// Close closes the TTS Podcast session
func (s *PodcastStreamSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

// =============================================================================
// V3 SAMI Podcast WebSocket API
// Endpoint: WSS /api/v3/sami/podcasttts
// Documentation: https://www.volcengine.com/docs/6561/1668014
// =============================================================================

// PodcastSAMIRequest represents a SAMI Podcast streaming request
type PodcastSAMIRequest struct {
	// Action type:
	//   0: Summary generation (from input_text)
	//   3: Direct dialogue generation (from nlp_texts)
	//   4: Extended generation (from prompt_text)
	Action int `json:"action"`

	// Input ID for tracking
	InputID string `json:"input_id,omitempty"`

	// Input text for action=0 (summary generation)
	InputText string `json:"input_text,omitempty"`

	// Dialogue texts for action=3 (direct generation)
	NlpTexts []PodcastDialogue `json:"nlp_texts,omitempty"`

	// Prompt text for action=4 (extended generation)
	PromptText string `json:"prompt_text,omitempty"`

	// Use head/tail music
	UseHeadMusic bool `json:"use_head_music,omitempty"`
	UseTailMusic bool `json:"use_tail_music,omitempty"`

	// Audio configuration
	AudioConfig *PodcastAudioConfig `json:"audio_config,omitempty"`

	// Speaker configuration (exactly 2 speakers required)
	SpeakerInfo *PodcastSpeakerInfo `json:"speaker_info,omitempty"`
}

// PodcastV3Request is an alias for PodcastSAMIRequest for backward compatibility
// Deprecated: Use PodcastSAMIRequest instead
type PodcastV3Request = PodcastSAMIRequest

// PodcastDialogue represents a single dialogue line
type PodcastDialogue struct {
	Speaker string `json:"speaker"` // speaker_1 or speaker_2
	Text    string `json:"text"`
}

// PodcastAudioConfig represents audio output configuration
type PodcastAudioConfig struct {
	Format     string `json:"format,omitempty"`      // pcm, mp3, ogg_opus, aac
	SampleRate int    `json:"sample_rate,omitempty"` // 16000, 24000, 48000
	SpeechRate int    `json:"speech_rate,omitempty"` // -50 ~ 100
}

// PodcastSpeakerInfo represents speaker configuration
type PodcastSpeakerInfo struct {
	RandomOrder bool     `json:"random_order,omitempty"`
	Speakers    []string `json:"speakers,omitempty"` // exactly 2 speakers
}

// PodcastSAMIChunk represents a streaming audio chunk from SAMI Podcast
type PodcastSAMIChunk struct {
	Event    string `json:"event"`
	TaskID   string `json:"task_id,omitempty"`
	Sequence int    `json:"sequence,omitempty"`
	Audio    []byte `json:"-"`
	Text     string `json:"text,omitempty"`    // Generated text (for summary)
	Message  string `json:"message,omitempty"` // Status message
	IsLast   bool   `json:"is_last"`
}

// PodcastV3Chunk is an alias for PodcastSAMIChunk for backward compatibility
// Deprecated: Use PodcastSAMIChunk instead
type PodcastV3Chunk = PodcastSAMIChunk

// PodcastSAMISession represents a SAMI Podcast WebSocket session
type PodcastSAMISession struct {
	conn      *websocket.Conn
	client    *Client
	reqID     string
	proto     *binaryProtocol

	recvChan  chan *PodcastSAMIChunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
}

// PodcastV3Session is an alias for PodcastSAMISession for backward compatibility
// Deprecated: Use PodcastSAMISession instead
type PodcastV3Session = PodcastSAMISession

// StreamSAMI opens a SAMI Podcast WebSocket session for real-time podcast synthesis
//
// This uses the SAMI Podcast endpoint: WSS /api/v3/sami/podcasttts
// Auth via HTTP headers: X-Api-App-Id, X-Api-Access-Key
//
// Example:
//
//	session, err := client.Podcast.StreamSAMI(ctx, &PodcastSAMIRequest{
//	    Action: 3,
//	    NlpTexts: []PodcastDialogue{
//	        {Speaker: "speaker_1", Text: "Hello!"},
//	        {Speaker: "speaker_2", Text: "Hi there!"},
//	    },
//	    SpeakerInfo: &PodcastSpeakerInfo{
//	        Speakers: []string{"zh_male_liufei_v2_saturn_bigtts", "zh_male_xiaolei_v2_saturn_bigtts"},
//	    },
//	})
//	if err != nil {
//	    return err
//	}
//	defer session.Close()
//
//	for chunk, err := range session.Recv() {
//	    // process chunk.Audio
//	}
func (s *PodcastService) StreamSAMI(ctx context.Context, req *PodcastSAMIRequest) (*PodcastSAMISession, error) {
	endpoint := s.client.config.wsURL + "/api/v3/sami/podcasttts"
	reqID := fmt.Sprintf("podcast-%d", time.Now().UnixNano())

	// SAMI Podcast requires specific headers:
	// - X-Api-App-Id: APP ID
	// - X-Api-Access-Key: Access Token
	// - X-Api-Resource-Id: volc.service_type.10050 (fixed)
	// - X-Api-App-Key: aGjiRDfUWi (fixed)
	// - X-Api-Request-Id: request UUID (optional)
	headers := http.Header{}
	headers.Set("X-Api-App-Id", s.client.config.appID)
	headers.Set("X-Api-Resource-Id", "volc.service_type.10050")
	headers.Set("X-Api-App-Key", "aGjiRDfUWi")
	headers.Set("X-Api-Request-Id", reqID)

	if s.client.config.accessToken != "" {
		headers.Set("X-Api-Access-Key", s.client.config.accessToken)
	} else if s.client.config.accessKey != "" {
		headers.Set("X-Api-Access-Key", s.client.config.accessKey)
	} else if s.client.config.apiKey != "" {
		headers.Set("X-Api-Access-Key", s.client.config.apiKey)
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("websocket connect failed: %w, status=%s, body=%s", err, resp.Status, string(body))
		}
		return nil, fmt.Errorf("websocket connect failed: %w", err)
	}

	session := &PodcastSAMISession{
		conn:      conn,
		client:    s.client,
		reqID:     reqID,
		proto:     newBinaryProtocol(),
		recvChan:  make(chan *PodcastSAMIChunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Start receiver goroutine
	go session.receiveLoopSAMI()

	// Send the podcast request
	if err := session.sendRequestSAMI(req); err != nil {
		session.Close()
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return session, nil
}

// StreamV3 is an alias for StreamSAMI for backward compatibility
// Deprecated: Use StreamSAMI instead
func (s *PodcastService) StreamV3(ctx context.Context, req *PodcastSAMIRequest) (*PodcastSAMISession, error) {
	return s.StreamSAMI(ctx, req)
}

// Recv returns an iterator for receiving SAMI Podcast audio chunks
func (s *PodcastSAMISession) Recv() iter.Seq2[*PodcastSAMIChunk, error] {
	return func(yield func(*PodcastSAMIChunk, error) bool) {
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

// Close closes the SAMI Podcast session
func (s *PodcastSAMISession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

func (s *PodcastSAMISession) sendRequestSAMI(req *PodcastSAMIRequest) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// SAMI Podcast 需要发送带 StartSession 事件的二进制帧
	// 格式：Header(4B) + EventType(4B) + SessionIDLen(4B) + SessionID + PayloadLen(4B) + Payload
	//
	// Header:
	//   Byte 0: 0x11 (version=1, header_size=1)
	//   Byte 1: 0x94 (msg_type=9=Full-client-request, flags=4=with-event)
	//   Byte 2: 0x10 (serialization=1=JSON, compression=0=none)
	//   Byte 3: 0x00 (reserved)
	var buf bytes.Buffer

	// Header (4 bytes)
	buf.WriteByte(0x11) // version=1, header_size=1
	buf.WriteByte(0x14) // msg_type=1=Full-client, flags=4=with-event
	buf.WriteByte(0x10) // serialization=JSON, compression=none
	buf.WriteByte(0x00) // reserved

	// Event type: StartSession = 100
	binary.Write(&buf, binary.BigEndian, int32(100))

	// Session ID (使用 reqID 作为 session_id)
	sessionIDBytes := []byte(s.reqID)
	binary.Write(&buf, binary.BigEndian, uint32(len(sessionIDBytes)))
	buf.Write(sessionIDBytes)

	// Payload length + Payload
	binary.Write(&buf, binary.BigEndian, uint32(len(jsonData)))
	buf.Write(jsonData)

	return s.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

func (s *PodcastSAMISession) receiveLoopSAMI() {
	defer close(s.recvChan)

	for {
		select {
		case <-s.closeChan:
			return
		default:
		}

		// Set read deadline for SAMI Podcast (can take a while for LLM processing)
		// 5 minutes timeout should be enough for most requests
		s.conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

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

		if msgType != websocket.BinaryMessage || len(data) < 4 {
			continue
		}

		// Parse V3 binary header
		// Byte 0: version (4 bits) + header_size (4 bits)
		// Byte 1: msg_type (4 bits) + flags (4 bits)
		// Byte 2: serialization (4 bits) + compression (4 bits)
		// Byte 3: reserved
		headerByte1 := data[1]
		headerByte2 := data[2]

		serverMsgType := (headerByte1 >> 4) & 0x0F
		msgFlags := headerByte1 & 0x0F
		serType := (headerByte2 >> 4) & 0x0F
		compType := headerByte2 & 0x0F

		// Offset starts after header
		offset := 4

		// Check if message has event flag (flags bit 2)
		var eventCode int32
		if msgFlags&0x04 != 0 {
			if len(data) < offset+4 {
				continue
			}
			eventCode = int32(binary.BigEndian.Uint32(data[offset : offset+4]))
			offset += 4

			// Read session ID length + session ID (skip)
			if len(data) < offset+4 {
				continue
			}
			sessionIDLen := binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4 + int(sessionIDLen)
		}

		// Read payload size
		if len(data) < offset+4 {
			continue
		}
		payloadSize := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if int(payloadSize) > len(data)-offset {
			continue
		}
		payload := data[offset : offset+int(payloadSize)]

		// Decompress if needed
		if compType == byte(compressionGzip) && len(payload) > 0 {
			if decompressed, err := gzipDecompress(payload); err == nil {
				payload = decompressed
			}
		}

		// Handle error response (msg_type = 0x0F or error flag)
		if serverMsgType == 0x0F || msgFlags&0x08 != 0 {
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

		// Determine if it's the last message based on event code
		// 152 = SessionFinished, 363 = PodcastEnd
		// Note: Some servers may send PodcastEnd without SessionFinished,
		// so we treat both as the end of the stream
		isLast := eventCode == 152 || eventCode == 363

		chunk := &PodcastSAMIChunk{
				IsLast: isLast,
			}

		// Map event codes to event names
		switch eventCode {
		case 150:
			chunk.Event = "SessionStarted"
		case 360:
			chunk.Event = "PodcastRoundStart"
		case 361:
			chunk.Event = "PodcastRoundResponse"
		case 362:
			chunk.Event = "PodcastRoundEnd"
		case 363:
			chunk.Event = "PodcastEnd"
		case 152:
			chunk.Event = "SessionFinished"
		case 154:
			chunk.Event = "UsageResponse"
		default:
			chunk.Event = fmt.Sprintf("Event_%d", eventCode)
		}

		// Parse payload based on serialization type
		if serType == byte(serializationNone) {
			// Raw audio data
				chunk.Audio = payload
		} else if serType == byte(serializationJSON) {
				// JSON response
				var resp struct {
					TaskID   string `json:"task_id"`
					Sequence int    `json:"sequence"`
					Data     string `json:"data"`
					Message  string `json:"message"`
				MetaInfo struct {
					AudioURL string `json:"audio_url"`
				} `json:"meta_info"`
				}
				if json.Unmarshal(payload, &resp) == nil {
					chunk.TaskID = resp.TaskID
					chunk.Sequence = resp.Sequence
					chunk.Text = resp.Data
					chunk.Message = resp.Message
				if resp.MetaInfo.AudioURL != "" {
					chunk.Text = resp.MetaInfo.AudioURL
				}
				}
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
