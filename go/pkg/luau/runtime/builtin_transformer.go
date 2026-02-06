package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// TransformerFactory creates a BiStream for a given transformer type and config.
type TransformerFactory func(ctx context.Context, config map[string]any) (BiStream, error)

// builtinTransformer implements __builtin.transformer(name, config) -> bistream, err
// When using genxTransformer, name is ignored and config["model"] is used as the pattern.
func (rt *Runtime) builtinTransformer(state *luau.State) int {
	name := state.ToString(1)
	if name == "" {
		state.PushNil()
		state.PushString("transformer name is required")
		return 2
	}

	// Parse config from Lua table
	var config map[string]any
	if state.GetTop() >= 2 && state.IsTable(2) {
		config = luaTableToMap(state, 2)
	}

	var bistream BiStream
	var err error

	// Try genxTransformer first if available
	if rt.genxTransformer != nil {
		bistream, err = rt.transformWithGenx(config)
	} else {
		// Fallback to registered transformer factories
		rt.transformersMu.RLock()
		factory, ok := rt.transformers[name]
		rt.transformersMu.RUnlock()

		if !ok {
			state.PushNil()
			state.PushString("unknown transformer: " + name)
			return 2
		}

		bistream, err = factory(rt.ctx, config)
	}

	if err != nil {
		state.PushNil()
		state.PushString(err.Error())
		return 2
	}

	// Register bistream and push handle
	id := rt.streams.registerBiStream(rt.ctx, bistream)
	rt.pushBiStreamObject(state, id)
	state.PushNil() // no error
	return 2
}

// transformWithGenx creates a BiStream using the genx.Transformer interface.
func (rt *Runtime) transformWithGenx(config map[string]any) (BiStream, error) {
	// Extract model/pattern from config
	pattern, _ := config["model"].(string)
	if pattern == "" {
		return nil, fmt.Errorf("transformer: model is required in config")
	}

	// Create input buffer stream
	inputStream := newGenxBufferStream(100)

	// Start the transformer
	outputStream, err := rt.genxTransformer.Transform(rt.ctx, pattern, inputStream)
	if err != nil {
		inputStream.Close()
		return nil, err
	}

	return &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}, nil
}

// genxBufferStream is a simple buffer stream for genx.Stream.
type genxBufferStream struct {
	ch     chan *genx.MessageChunk
	closed bool
	mu     sync.Mutex
	err    error
}

func newGenxBufferStream(size int) *genxBufferStream {
	return &genxBufferStream{
		ch: make(chan *genx.MessageChunk, size),
	}
}

func (s *genxBufferStream) Next() (*genx.MessageChunk, error) {
	chunk, ok := <-s.ch
	if !ok {
		s.mu.Lock()
		err := s.err
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return nil, genx.ErrDone
	}
	return chunk, nil
}

func (s *genxBufferStream) Push(chunk *genx.MessageChunk) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("stream closed")
	}
	s.mu.Unlock()
	s.ch <- chunk
	return nil
}

func (s *genxBufferStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
	return nil
}

func (s *genxBufferStream) CloseWithError(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
	return nil
}

// genxTransformerBiStream wraps a genx transformer as a BiStream.
type genxTransformerBiStream struct {
	input  *genxBufferStream
	output genx.Stream
	closed bool
	mu     sync.Mutex
}

func (s *genxTransformerBiStream) Send(ctx context.Context, chunk *MessageChunk) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrStreamClosed
	}
	s.mu.Unlock()

	// Convert runtime chunk to genx chunk
	genxChunk := &genx.MessageChunk{}

	if chunk.Part != nil {
		switch p := chunk.Part.(type) {
		case string:
			genxChunk.Part = genx.Text(p)
		case []byte:
			genxChunk.Part = &genx.Blob{MIMEType: "audio/pcm", Data: p}
		case map[string]any:
			if data, ok := p["data"].([]byte); ok {
				mimeType, _ := p["mime_type"].(string)
				if mimeType == "" {
					mimeType = "audio/pcm"
				}
				genxChunk.Part = &genx.Blob{MIMEType: mimeType, Data: data}
			} else if text, ok := p["text"].(string); ok {
				genxChunk.Part = genx.Text(text)
			}
		}
	}

	// Handle stream control
	if chunk.IsBOS {
		genxChunk.Ctrl = &genx.StreamCtrl{BeginOfStream: true, StreamID: chunk.StreamID}
	}
	if chunk.IsEOS {
		genxChunk.Ctrl = &genx.StreamCtrl{EndOfStream: true, StreamID: chunk.StreamID}
	}

	return s.input.Push(genxChunk)
}

func (s *genxTransformerBiStream) CloseSend() error {
	return s.input.Close()
}

func (s *genxTransformerBiStream) Recv(ctx context.Context) (*MessageChunk, error) {
	chunk, err := s.output.Next()
	if err != nil {
		if errors.Is(err, genx.ErrDone) || err == io.EOF {
			return nil, ErrStreamEOF
		}
		return nil, err
	}
	if chunk == nil {
		return nil, ErrStreamEOF
	}

	// Convert to runtime chunk
	rtChunk := &MessageChunk{}
	if chunk.Part != nil {
		switch p := chunk.Part.(type) {
		case genx.Text:
			rtChunk.Part = string(p)
		case *genx.Blob:
			if p != nil {
				rtChunk.Part = map[string]any{
					"type":      "blob",
					"mime_type": p.MIMEType,
					"data":      p.Data,
				}
			}
		}
	}
	if chunk.Ctrl != nil {
		rtChunk.StreamID = chunk.Ctrl.StreamID
		rtChunk.IsBOS = chunk.Ctrl.BeginOfStream
		rtChunk.IsEOS = chunk.Ctrl.EndOfStream
	}
	return rtChunk, nil
}

func (s *genxTransformerBiStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.input.Close()
	if s.output != nil {
		return s.output.Close()
	}
	return nil
}

// pipeBiStream is a simple BiStream backed by channels.
type pipeBiStream struct {
	input     chan *MessageChunk
	output    chan *MessageChunk
	inputDone chan struct{}
	closedMu  sync.Mutex
	closed    bool
}

// NewPipeBiStream creates a BiStream backed by channels.
// The processor function runs in a goroutine and processes input to output.
func NewPipeBiStream(
	ctx context.Context,
	processor func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk),
) BiStream {
	bs := &pipeBiStream{
		input:     make(chan *MessageChunk, 16),
		output:    make(chan *MessageChunk, 16),
		inputDone: make(chan struct{}),
	}

	go func() {
		defer close(bs.output)
		processor(ctx, bs.input, bs.output)
	}()

	return bs
}

func (bs *pipeBiStream) Send(ctx context.Context, chunk *MessageChunk) error {
	bs.closedMu.Lock()
	if bs.closed {
		bs.closedMu.Unlock()
		return ErrStreamClosed
	}
	bs.closedMu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case bs.input <- chunk:
		return nil
	}
}

func (bs *pipeBiStream) CloseSend() error {
	bs.closedMu.Lock()
	defer bs.closedMu.Unlock()

	select {
	case <-bs.inputDone:
		// Already closed
	default:
		close(bs.inputDone)
		close(bs.input)
	}
	return nil
}

func (bs *pipeBiStream) Recv(ctx context.Context) (*MessageChunk, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case chunk, ok := <-bs.output:
		if !ok {
			return nil, ErrStreamEOF
		}
		return chunk, nil
	}
}

func (bs *pipeBiStream) Close() error {
	bs.closedMu.Lock()
	defer bs.closedMu.Unlock()

	if bs.closed {
		return nil
	}
	bs.closed = true

	// Close input if not already closed
	select {
	case <-bs.inputDone:
	default:
		close(bs.inputDone)
		close(bs.input)
	}

	return nil
}

// callbackBiStream wraps callback functions as a BiStream.
type callbackBiStream struct {
	send      func(ctx context.Context, chunk *MessageChunk) error
	closeSend func() error
	recv      func(ctx context.Context) (*MessageChunk, error)
	close     func() error
	closedMu  sync.Mutex
	closed    bool
}

// NewCallbackBiStream creates a BiStream from callback functions.
func NewCallbackBiStream(
	send func(ctx context.Context, chunk *MessageChunk) error,
	closeSend func() error,
	recv func(ctx context.Context) (*MessageChunk, error),
	closeFunc func() error,
) BiStream {
	return &callbackBiStream{
		send:      send,
		closeSend: closeSend,
		recv:      recv,
		close:     closeFunc,
	}
}

func (bs *callbackBiStream) Send(ctx context.Context, chunk *MessageChunk) error {
	bs.closedMu.Lock()
	if bs.closed {
		bs.closedMu.Unlock()
		return ErrStreamClosed
	}
	bs.closedMu.Unlock()
	return bs.send(ctx, chunk)
}

func (bs *callbackBiStream) CloseSend() error {
	if bs.closeSend != nil {
		return bs.closeSend()
	}
	return nil
}

func (bs *callbackBiStream) Recv(ctx context.Context) (*MessageChunk, error) {
	return bs.recv(ctx)
}

func (bs *callbackBiStream) Close() error {
	bs.closedMu.Lock()
	defer bs.closedMu.Unlock()

	if bs.closed {
		return nil
	}
	bs.closed = true

	if bs.close != nil {
		return bs.close()
	}
	return nil
}
