package minimax

import (
	"context"
	"encoding/json"
	"iter"
)

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

		resp, err := s.client.http.requestStream(ctx, "POST", "/v1/t2a_v2", streamReq)
		if err != nil {
			yield(nil, err)
			return
		}

		reader := newSSEReader(resp)
		defer reader.close()

		for {
			data, done, err := reader.readEvent()
			if err != nil {
				yield(nil, err)
				return
			}
			if done {
				return
			}

			var streamResp speechStreamResponse
			if err := json.Unmarshal(data, &streamResp); err != nil {
				yield(nil, err)
				return
			}

			// Check for API error
			if streamResp.BaseResp != nil && streamResp.BaseResp.StatusCode != 0 {
				yield(nil, &Error{
					StatusCode: streamResp.BaseResp.StatusCode,
					StatusMsg:  streamResp.BaseResp.StatusMsg,
				})
				return
			}

			chunk := &SpeechChunk{
				Status:    streamResp.Data.Status,
				ExtraInfo: streamResp.ExtraInfo,
				TraceID:   streamResp.TraceID,
			}

			// Decode hex audio if present
			if streamResp.Data.Audio != "" {
				audio, err := decodeHexAudio(streamResp.Data.Audio)
				if err != nil {
					yield(nil, err)
					return
				}
				chunk.Audio = audio
			}

			if !yield(chunk, nil) {
				return
			}
		}
	}
}

// speechStreamResponse is the streaming response for speech synthesis.
type speechStreamResponse struct {
	Data      speechData `json:"data"`
	ExtraInfo *AudioInfo `json:"extra_info,omitempty"`
	TraceID   string     `json:"trace_id,omitempty"`
	BaseResp  *baseResp  `json:"base_resp,omitempty"`
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
