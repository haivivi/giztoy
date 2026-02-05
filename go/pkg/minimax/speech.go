package minimax

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strings"
)

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// SpeechService provides speech synthesis operations.
type SpeechService struct {
	client *Client
}

// newSpeechService creates a new speech service.
func newSpeechService(client *Client) *SpeechService {
	return &SpeechService{client: client}
}

// speechResponse is the API response for speech synthesis.
type speechResponse struct {
	Data      speechData `json:"data"`
	ExtraInfo *AudioInfo `json:"extra_info"`
	TraceID   string     `json:"trace_id"`
	BaseResp  *baseResp  `json:"base_resp"`
}

type speechData struct {
	Audio    string `json:"audio"`     // hex-encoded audio data
	AudioURL string `json:"audio_url"` // URL when output_format is "url"
	Status   int    `json:"status"`
}

// Synthesize performs synchronous speech synthesis.
//
// The returned audio data is automatically decoded from hex format.
// Maximum text length is 10,000 characters.
func (s *SpeechService) Synthesize(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error) {
	var apiResp speechResponse
	err := s.client.http.request(ctx, "POST", "/v1/t2a_v2", req, &apiResp)
	if err != nil {
		return nil, err
	}

	resp := &SpeechResponse{
		AudioURL:  apiResp.Data.AudioURL,
		ExtraInfo: apiResp.ExtraInfo,
		TraceID:   apiResp.TraceID,
	}

	// Decode hex audio if present
	if apiResp.Data.Audio != "" {
		audio, err := decodeHexAudio(apiResp.Data.Audio)
		if err != nil {
			return nil, err
		}
		resp.Audio = audio
	}

	return resp, nil
}

// SynthesizeStream performs streaming speech synthesis.
//
// Returns an iterator that yields audio chunks. The connection is automatically
// closed when iteration completes or breaks.
//
// Example:
//
//	var buf bytes.Buffer
//	for chunk, err := range client.Speech.SynthesizeStream(ctx, req) {
//	    if err != nil {
//	        return err
//	    }
//	    if chunk.Audio != nil {
//	        buf.Write(chunk.Audio)
//	    }
//	}
func (s *SpeechService) SynthesizeStream(ctx context.Context, req *SpeechRequest) iter.Seq2[*SpeechChunk, error] {
	return func(yield func(*SpeechChunk, error) bool) {
		// Add stream flag to request
		streamReq := struct {
			*SpeechRequest
			Stream bool `json:"stream"`
		}{
			SpeechRequest: req,
			Stream:        true,
		}

		slog.Debug("MiniMax SynthesizeStream starting", "model", req.Model, "text_len", len(req.Text))

		resp, err := s.client.http.requestStream(ctx, "POST", "/v1/t2a_v2", streamReq)
		if err != nil {
			slog.Debug("MiniMax SynthesizeStream request error", "err", err)
			yield(nil, err)
			return
		}

		contentType := resp.Header.Get("Content-Type")
		slog.Debug("MiniMax SynthesizeStream response", "status", resp.StatusCode, "content_type", contentType)

		// Check if this is NOT a streaming response
		if !strings.Contains(contentType, "event-stream") {
			// Non-streaming response - read and parse JSON directly
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				yield(nil, fmt.Errorf("read response: %w", err))
				return
			}
			slog.Debug("MiniMax non-streaming response", "body_len", len(body), "body_preview", truncateStr(string(body), 200))

			var jsonResp struct {
				Data struct {
					Audio string `json:"audio"`
				} `json:"data"`
				ExtraInfo *AudioInfo `json:"extra_info,omitempty"`
				TraceID   string     `json:"trace_id,omitempty"`
				BaseResp  *baseResp  `json:"base_resp,omitempty"`
			}
			if err := json.Unmarshal(body, &jsonResp); err != nil {
				yield(nil, fmt.Errorf("unmarshal response: %w", err))
				return
			}

			// Check for API error
			if jsonResp.BaseResp != nil && jsonResp.BaseResp.StatusCode != 0 {
				yield(nil, &Error{
					StatusCode: jsonResp.BaseResp.StatusCode,
					StatusMsg:  jsonResp.BaseResp.StatusMsg,
				})
				return
			}

			// Decode audio
			if jsonResp.Data.Audio != "" {
				audio, err := decodeHexAudio(jsonResp.Data.Audio)
				if err != nil {
					yield(nil, fmt.Errorf("decode audio: %w", err))
					return
				}
				chunk := &SpeechChunk{
					ExtraInfo: jsonResp.ExtraInfo,
					TraceID:   jsonResp.TraceID,
					Audio:     audio,
				}
				slog.Debug("MiniMax audio from JSON", "audio_len", len(audio))
				yield(chunk, nil)
			}
			return
		}

		reader := newSSEReader(resp)
		defer reader.close()

		eventCount := 0
		for {
			data, done, err := reader.readEvent()
			if err != nil {
				slog.Debug("MiniMax SSE read error", "err", err)
				yield(nil, err)
				return
			}
			if done {
				slog.Debug("MiniMax SSE done", "events", eventCount)
				return
			}

			eventCount++
			slog.Debug("MiniMax SSE event", "count", eventCount, "data_len", len(data))

			var streamResp speechStreamResponse
			if err := json.Unmarshal(data, &streamResp); err != nil {
				slog.Debug("MiniMax SSE unmarshal error", "err", err, "data", string(data))
				yield(nil, err)
				return
			}

			// Check for API error
			if streamResp.BaseResp != nil && streamResp.BaseResp.StatusCode != 0 {
				slog.Debug("MiniMax API error", "code", streamResp.BaseResp.StatusCode, "msg", streamResp.BaseResp.StatusMsg)
				yield(nil, &Error{
					StatusCode: streamResp.BaseResp.StatusCode,
					StatusMsg:  streamResp.BaseResp.StatusMsg,
				})
				return
			}

			chunk := &SpeechChunk{
				Status:    streamResp.Data.Status,
				ExtraInfo: streamResp.ExtraInfo,
				Subtitle:  streamResp.Subtitle,
				TraceID:   streamResp.TraceID,
			}

			// Decode hex audio if present
			// Note: status=2 contains the COMPLETE audio file (not incremental),
			// so we skip it to avoid duplication when streaming
			if streamResp.Data.Audio != "" && streamResp.Data.Status != 2 {
				audio, err := decodeHexAudio(streamResp.Data.Audio)
				if err != nil {
					yield(nil, err)
					return
				}
				chunk.Audio = audio
				slog.Debug("MiniMax audio chunk", "audio_len", len(audio))
			}

			if !yield(chunk, nil) {
				return
			}
		}
	}
}

// speechStreamResponse is the streaming response for speech synthesis.
type speechStreamResponse struct {
	Data      speechData       `json:"data"`
	ExtraInfo *AudioInfo       `json:"extra_info,omitempty"`
	Subtitle  *SubtitleSegment `json:"subtitle,omitempty"`
	TraceID   string           `json:"trace_id,omitempty"`
	BaseResp  *baseResp        `json:"base_resp,omitempty"`
}

// CreateAsyncTask creates an async speech synthesis task.
//
// For long texts up to 1,000,000 characters. Returns a Task that can be
// polled for completion.
//
// Example:
//
//	task, err := client.Speech.CreateAsyncTask(ctx, req)
//	if err != nil {
//	    return err
//	}
//	result, err := task.Wait(ctx)
func (s *SpeechService) CreateAsyncTask(ctx context.Context, req *AsyncSpeechRequest) (*Task[SpeechAsyncResult], error) {
	var resp struct {
		TaskID   string    `json:"task_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/t2a_async", req, &resp)
	if err != nil {
		return nil, err
	}

	return &Task[SpeechAsyncResult]{
		ID:       resp.TaskID,
		client:   s.client,
		taskType: taskTypeSpeechAsync,
	}, nil
}
