package minimax

import (
	"context"
	"encoding/json"
	"iter"
)

// TextService provides text generation operations.
type TextService struct {
	client *Client
}

// newTextService creates a new text service.
func newTextService(client *Client) *TextService {
	return &TextService{client: client}
}

// CreateChatCompletion creates a chat completion.
func (s *TextService) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	var resp ChatCompletionResponse
	err := s.client.http.request(ctx, "POST", "/v1/chat/completions", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateChatCompletionStream creates a streaming chat completion.
//
// Returns an iterator that yields chunks. The connection is automatically
// closed when iteration completes or breaks.
//
// Example:
//
//	for chunk, err := range client.Text.CreateChatCompletionStream(ctx, req) {
//	    if err != nil {
//	        return err
//	    }
//	    if len(chunk.Choices) > 0 {
//	        fmt.Print(chunk.Choices[0].Delta.Content)
//	    }
//	}
func (s *TextService) CreateChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) iter.Seq2[*ChatCompletionChunk, error] {
	return func(yield func(*ChatCompletionChunk, error) bool) {
		// Add stream flag to request
		streamReq := struct {
			*ChatCompletionRequest
			Stream bool `json:"stream"`
		}{
			ChatCompletionRequest: req,
			Stream:                true,
		}

		resp, err := s.client.http.requestStream(ctx, "POST", "/v1/chat/completions", streamReq)
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

			var chunk ChatCompletionChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				yield(nil, err)
				return
			}

			if !yield(&chunk, nil) {
				return
			}
		}
	}
}
