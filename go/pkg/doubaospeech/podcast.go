// Podcast Service - Multi-speaker Podcast Synthesis
//
// Supports both V1 async API and V3 WebSocket streaming API.
//
// V1 API (Async):
//   - POST /api/v1/podcast/submit - Submit async task
//   - POST /api/v1/podcast/query  - Query task status
//
// V3 API (WebSocket Streaming):
//   - WSS /api/v3/sami/podcasttts - Real-time podcast synthesis
//
// Documentation: https://www.volcengine.com/docs/6561/1668014
package doubaospeech

import (
	"bytes"
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
// V3 WebSocket Streaming API
// =============================================================================

// PodcastV3Request represents a V3 podcast streaming request
type PodcastV3Request struct {
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

// PodcastV3Chunk represents a streaming audio chunk
type PodcastV3Chunk struct {
	Event    string `json:"event"`
	TaskID   string `json:"task_id,omitempty"`
	Sequence int    `json:"sequence,omitempty"`
	Audio    []byte `json:"-"`
	Text     string `json:"text,omitempty"`    // Generated text (for summary)
	Message  string `json:"message,omitempty"` // Status message
	IsLast   bool   `json:"is_last"`
}

// PodcastV3Session represents a V3 WebSocket podcast session
type PodcastV3Session struct {
	conn      *websocket.Conn
	client    *Client
	reqID     string

	recvChan  chan *PodcastV3Chunk
	errChan   chan error
	closeChan chan struct{}
	closeOnce sync.Once
}

// StreamV3 opens a V3 WebSocket session for real-time podcast synthesis
//
// This uses the streaming endpoint: WSS /api/v3/sami/podcasttts
//
// Example:
//
//	session, err := client.Podcast.StreamV3(ctx, &PodcastV3Request{
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
func (s *PodcastService) StreamV3(ctx context.Context, req *PodcastV3Request) (*PodcastV3Session, error) {
	endpoint := s.client.config.wsURL + "/api/v3/sami/podcasttts"
	reqID := fmt.Sprintf("podcast-%d", time.Now().UnixNano())

	// V3 Podcast uses X-Api-App-Id and X-Api-Access-Key headers
	headers := http.Header{}
	headers.Set("X-Api-App-Id", s.client.config.appID)
	if s.client.config.accessToken != "" {
		headers.Set("X-Api-Access-Key", s.client.config.accessToken)
	} else if s.client.config.accessKey != "" {
		headers.Set("X-Api-Access-Key", s.client.config.accessKey)
	} else if s.client.config.apiKey != "" {
		headers.Set("x-api-key", s.client.config.apiKey)
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("websocket connect failed: %w, status=%s, body=%s", err, resp.Status, string(body))
		}
		return nil, fmt.Errorf("websocket connect failed: %w", err)
	}

	session := &PodcastV3Session{
		conn:      conn,
		client:    s.client,
		reqID:     reqID,
		recvChan:  make(chan *PodcastV3Chunk, 100),
		errChan:   make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	// Start receiver goroutine
	go session.receiveLoop()

	// Send the podcast request
	if err := session.sendRequest(req); err != nil {
		session.Close()
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return session, nil
}

// Recv returns an iterator for receiving podcast audio chunks
func (s *PodcastV3Session) Recv() iter.Seq2[*PodcastV3Chunk, error] {
	return func(yield func(*PodcastV3Chunk, error) bool) {
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

// Close closes the podcast session
func (s *PodcastV3Session) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.conn.Close()
	})
	return nil
}

// Podcast V3 binary protocol constants
const (
	podcastV3Version     = 0b0001
	podcastV3HeaderSize  = 0b0001
	podcastV3MsgSendText = 0b0001
	podcastV3SerJSON     = 0b0001
	podcastV3CompNone    = 0b0000
)

func (s *PodcastV3Session) sendRequest(req *PodcastV3Request) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Build V3 binary header
	// Byte 0: [Protocol version (4bit) | Header size (4bit)]
	// Byte 1: [Message type (4bit) | Message type specific flags (4bit)]
	// Byte 2: [Serialization method (4bit) | Compression type (4bit)]
	// Byte 3: Reserved
	header := []byte{
		(podcastV3Version << 4) | podcastV3HeaderSize,
		podcastV3MsgSendText << 4,
		(podcastV3SerJSON << 4) | podcastV3CompNone,
		0,
	}

	var buf bytes.Buffer
	buf.Write(header)
	binary.Write(&buf, binary.BigEndian, uint32(len(jsonData)))
	buf.Write(jsonData)

	return s.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

func (s *PodcastV3Session) receiveLoop() {
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

		if msgType != websocket.BinaryMessage || len(data) < 4 {
			continue
		}

		// Parse V3 binary header
		headerByte1 := data[1]
		headerByte2 := data[2]

		serverMsgType := (headerByte1 >> 4) & 0x0F
		msgFlags := headerByte1 & 0x0F
		serType := (headerByte2 >> 4) & 0x0F

		// Read payload size
		if len(data) < 8 {
			continue
		}
		payloadSize := binary.BigEndian.Uint32(data[4:8])
		if int(payloadSize) > len(data)-8 {
			continue
		}
		payload := data[8 : 8+payloadSize]

		// Server response type 0b1001 (9)
		if serverMsgType == 0b1001 {
			// Check if it's an error (msg_flags bit 3)
			if msgFlags&0b1000 != 0 {
				var errResp struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}
				if serType == podcastV3SerJSON {
					if json.Unmarshal(payload, &errResp) == nil {
						select {
						case s.errChan <- &Error{Code: errResp.Code, Message: errResp.Message}:
						default:
						}
					}
				}
				return
			}

			// Check if it's the last message (msg_flags bit 2)
			isLast := msgFlags&0b0100 != 0

			chunk := &PodcastV3Chunk{
				IsLast: isLast,
			}

			// Audio data (serialization type 0 = raw)
			if serType == 0 {
				chunk.Audio = payload
				chunk.Event = "audio"
			} else if serType == podcastV3SerJSON {
				// JSON response
				var resp struct {
					Event    string `json:"event"`
					TaskID   string `json:"task_id"`
					Sequence int    `json:"sequence"`
					Data     string `json:"data"`
					Message  string `json:"message"`
				}
				if json.Unmarshal(payload, &resp) == nil {
					chunk.Event = resp.Event
					chunk.TaskID = resp.TaskID
					chunk.Sequence = resp.Sequence
					chunk.Text = resp.Data
					chunk.Message = resp.Message
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
}
