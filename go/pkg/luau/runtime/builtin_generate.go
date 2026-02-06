package runtime

import (
	"context"
	"errors"
	"io"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// GeneratorFunc is the function signature for LLM generation.
// It takes a model name and model context, returns a Stream.
type GeneratorFunc func(ctx context.Context, model string, mctx map[string]any) (Stream, error)

// builtinGenerate implements __builtin.generate(model, mctx) -> stream, err
func (rt *Runtime) builtinGenerate(state *luau.State) int {
	// Check if any generator is configured
	if rt.genxGenerator == nil && rt.generator == nil {
		state.PushNil()
		state.PushString("generator not configured")
		return 2
	}

	model := state.ToString(1)
	if model == "" {
		state.PushNil()
		state.PushString("model is required")
		return 2
	}

	// Parse model context from Lua table
	var mctx map[string]any
	if state.GetTop() >= 2 && state.IsTable(2) {
		mctx = luaTableToMap(state, 2)
	}

	var stream Stream
	var err error

	// Prefer genxGenerator if available
	if rt.genxGenerator != nil {
		stream, err = rt.generateWithGenx(model, mctx)
	} else {
		stream, err = rt.generator(rt.ctx, model, mctx)
	}

	if err != nil {
		state.PushNil()
		state.PushString(err.Error())
		return 2
	}

	// Register stream and push handle
	id := rt.streams.registerStream(rt.ctx, stream)
	rt.pushStreamObject(state, id)
	state.PushNil() // no error
	return 2
}

// generateWithGenx uses the genx.Generator interface to generate a stream.
func (rt *Runtime) generateWithGenx(model string, mctx map[string]any) (Stream, error) {
	// Build genx.ModelContext from map
	builder := &genx.ModelContextBuilder{}

	// System prompt
	if sys, ok := mctx["system"].(string); ok && sys != "" {
		builder.PromptText("system", sys)
	}

	// Messages
	if msgs, ok := mctx["messages"].([]any); ok {
		for _, m := range msgs {
			if msg, ok := m.(map[string]any); ok {
				role := ""
				content := ""
				if r, ok := msg["role"].(string); ok {
					role = r
				}
				if c, ok := msg["content"].(string); ok {
					content = c
				}
				switch role {
				case "user":
					builder.UserText("", content)
				case "assistant", "model":
					builder.ModelText("", content)
				case "system":
					builder.PromptText("system", content)
				}
			}
		}
	}

	genxCtx := builder.Build()

	// Call the generator
	genxStream, err := rt.genxGenerator.GenerateStream(rt.ctx, model, genxCtx)
	if err != nil {
		return nil, err
	}

	return &genxStreamAdapter{stream: genxStream}, nil
}

// genxStreamAdapter wraps a genx.Stream to implement runtime.Stream.
type genxStreamAdapter struct {
	stream genx.Stream
}

func (s *genxStreamAdapter) Recv(ctx context.Context) (*MessageChunk, error) {
	chunk, err := s.stream.Next()
	if err != nil {
		// genx uses ErrDone or io.EOF to signal end of stream
		if errors.Is(err, genx.ErrDone) || err == io.EOF {
			return nil, ErrStreamEOF
		}
		return nil, err
	}
	if chunk == nil {
		return nil, ErrStreamEOF
	}

	// Convert genx.MessageChunk to runtime.MessageChunk
	rtChunk := &MessageChunk{}

	// Convert Part
	if chunk.Part != nil {
		switch p := chunk.Part.(type) {
		case genx.Text:
			rtChunk.Part = string(p)
		case *genx.Blob:
			if p != nil {
				rtChunk.Part = p.Data
			}
		}
	}

	// Convert StreamCtrl
	if chunk.Ctrl != nil {
		rtChunk.StreamID = chunk.Ctrl.StreamID
		rtChunk.IsBOS = chunk.Ctrl.BeginOfStream
		rtChunk.IsEOS = chunk.Ctrl.EndOfStream
	}

	return rtChunk, nil
}

func (s *genxStreamAdapter) Close() error {
	return s.stream.Close()
}

// luaTableToMap converts a Lua table to map[string]any.
func luaTableToMap(state *luau.State, idx int) map[string]any {
	if idx < 0 {
		idx = state.GetTop() + idx + 1
	}

	result := make(map[string]any)

	state.PushNil()
	for state.Next(idx) {
		var key string
		switch state.TypeOf(-2) {
		case luau.TypeString:
			key = state.ToString(-2)
		case luau.TypeNumber:
			key = state.ToString(-2) // Let Lua convert number to string
		default:
			state.Pop(1)
			continue
		}
		result[key] = luaToGo(state, -1)
		state.Pop(1)
	}

	return result
}

// channelStream adapts a channel to the Stream interface.
type channelStream struct {
	ch     <-chan *MessageChunk
	done   chan struct{}
	closed bool
}

// NewChannelStream creates a Stream from a channel.
func NewChannelStream(ch <-chan *MessageChunk) Stream {
	return &channelStream{
		ch:   ch,
		done: make(chan struct{}),
	}
}

func (s *channelStream) Recv(ctx context.Context) (*MessageChunk, error) {
	if s.closed {
		return nil, ErrStreamClosed
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrStreamClosed
	case chunk, ok := <-s.ch:
		if !ok {
			return nil, ErrStreamEOF
		}
		return chunk, nil
	}
}

func (s *channelStream) Close() error {
	if !s.closed {
		s.closed = true
		close(s.done)
	}
	return nil
}

// callbackStream wraps a callback function as a Stream.
type callbackStream struct {
	next   func(ctx context.Context) (*MessageChunk, error)
	close  func() error
	closed bool
}

// NewCallbackStream creates a Stream from callback functions.
func NewCallbackStream(
	next func(ctx context.Context) (*MessageChunk, error),
	closeFunc func() error,
) Stream {
	return &callbackStream{
		next:  next,
		close: closeFunc,
	}
}

func (s *callbackStream) Recv(ctx context.Context) (*MessageChunk, error) {
	if s.closed {
		return nil, ErrStreamClosed
	}
	return s.next(ctx)
}

func (s *callbackStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.close != nil {
		return s.close()
	}
	return nil
}

// simpleStream is a simple implementation for testing.
type simpleStream struct {
	chunks []*MessageChunk
	idx    int
	closed bool
}

// NewSimpleStream creates a Stream from a slice of chunks.
func NewSimpleStream(chunks []*MessageChunk) Stream {
	return &simpleStream{chunks: chunks}
}

func (s *simpleStream) Recv(ctx context.Context) (*MessageChunk, error) {
	if s.closed {
		return nil, ErrStreamClosed
	}
	if s.idx >= len(s.chunks) {
		return nil, ErrStreamEOF
	}
	chunk := s.chunks[s.idx]
	s.idx++
	return chunk, nil
}

func (s *simpleStream) Close() error {
	s.closed = true
	return nil
}

// ErrNotImplemented is returned when a feature is not implemented.
var ErrNotImplemented = errors.New("not implemented")
