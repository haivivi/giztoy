package runtime

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// mockTransformer implements genx.Transformer for testing.
type mockTransformer struct {
	outputStream genx.Stream
	err          error
	called       bool
	pattern      string
	inputStream  genx.Stream
}

func (t *mockTransformer) Transform(ctx context.Context, pattern string, input genx.Stream) (genx.Stream, error) {
	t.called = true
	t.pattern = pattern
	t.inputStream = input
	if t.err != nil {
		return nil, t.err
	}
	return t.outputStream, nil
}

// Test genxBufferStream
func TestGenxBufferStream_PushAndNext(t *testing.T) {
	stream := newGenxBufferStream(10)

	// Push some chunks
	chunk1 := &genx.MessageChunk{Part: genx.Text("hello")}
	chunk2 := &genx.MessageChunk{Part: genx.Text("world")}

	if err := stream.Push(chunk1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := stream.Push(chunk2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close to signal end
	stream.Close()

	// Read back
	got1, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got1.Part != genx.Text("hello") {
		t.Errorf("expected 'hello', got %v", got1.Part)
	}

	got2, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got2.Part != genx.Text("world") {
		t.Errorf("expected 'world', got %v", got2.Part)
	}

	// Should get ErrDone after close
	_, err = stream.Next()
	if !errors.Is(err, genx.ErrDone) {
		t.Errorf("expected genx.ErrDone, got %v", err)
	}
}

func TestGenxBufferStream_PushAfterClose(t *testing.T) {
	stream := newGenxBufferStream(10)
	stream.Close()

	err := stream.Push(&genx.MessageChunk{Part: genx.Text("test")})
	if err == nil {
		t.Error("expected error when pushing to closed stream")
	}
}

func TestGenxBufferStream_CloseWithError(t *testing.T) {
	stream := newGenxBufferStream(10)
	customErr := errors.New("custom error")

	stream.CloseWithError(customErr)

	_, err := stream.Next()
	if err != customErr {
		t.Errorf("expected custom error, got %v", err)
	}
}

func TestGenxBufferStream_DoubleClose(t *testing.T) {
	stream := newGenxBufferStream(10)

	err := stream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Double close should be safe
	err = stream.Close()
	if err != nil {
		t.Errorf("unexpected error on double close: %v", err)
	}
}

// Test genxTransformerBiStream
func TestGenxTransformerBiStream_SendText(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{
		{Part: genx.Text("response")},
	}}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	err := biStream.Send(ctx, &MessageChunk{Part: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the chunk was pushed to input stream
	inputStream.Close()
	chunk, _ := inputStream.Next()
	if chunk.Part != genx.Text("hello") {
		t.Errorf("expected Text 'hello', got %v", chunk.Part)
	}
}

func TestGenxTransformerBiStream_SendBytes(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	data := []byte{0x01, 0x02, 0x03}
	err := biStream.Send(ctx, &MessageChunk{Part: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inputStream.Close()
	chunk, _ := inputStream.Next()
	blob, ok := chunk.Part.(*genx.Blob)
	if !ok {
		t.Fatalf("expected Blob, got %T", chunk.Part)
	}
	if blob.MIMEType != "audio/pcm" {
		t.Errorf("expected 'audio/pcm', got %s", blob.MIMEType)
	}
}

func TestGenxTransformerBiStream_SendMapWithData(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	data := []byte{0x01, 0x02}
	err := biStream.Send(ctx, &MessageChunk{
		Part: map[string]any{
			"data":      data,
			"mime_type": "audio/mp3",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inputStream.Close()
	chunk, _ := inputStream.Next()
	blob, ok := chunk.Part.(*genx.Blob)
	if !ok {
		t.Fatalf("expected Blob, got %T", chunk.Part)
	}
	if blob.MIMEType != "audio/mp3" {
		t.Errorf("expected 'audio/mp3', got %s", blob.MIMEType)
	}
}

func TestGenxTransformerBiStream_SendMapWithText(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	err := biStream.Send(ctx, &MessageChunk{
		Part: map[string]any{
			"text": "hello from map",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inputStream.Close()
	chunk, _ := inputStream.Next()
	if chunk.Part != genx.Text("hello from map") {
		t.Errorf("expected Text 'hello from map', got %v", chunk.Part)
	}
}

func TestGenxTransformerBiStream_SendMapWithDefaultMimeType(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	data := []byte{0x01}
	err := biStream.Send(ctx, &MessageChunk{
		Part: map[string]any{
			"data": data,
			// no mime_type, should default to audio/pcm
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inputStream.Close()
	chunk, _ := inputStream.Next()
	blob := chunk.Part.(*genx.Blob)
	if blob.MIMEType != "audio/pcm" {
		t.Errorf("expected default 'audio/pcm', got %s", blob.MIMEType)
	}
}

func TestGenxTransformerBiStream_SendWithStreamCtrl(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()

	// Send with BOS
	err := biStream.Send(ctx, &MessageChunk{
		Part:     "start",
		StreamID: "stream-1",
		IsBOS:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Send with EOS
	err = biStream.Send(ctx, &MessageChunk{
		Part:     "end",
		StreamID: "stream-1",
		IsEOS:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inputStream.Close()

	// Check BOS chunk
	chunk1, _ := inputStream.Next()
	if chunk1.Ctrl == nil || !chunk1.Ctrl.BeginOfStream {
		t.Error("expected BOS flag")
	}

	// Check EOS chunk
	chunk2, _ := inputStream.Next()
	if chunk2.Ctrl == nil || !chunk2.Ctrl.EndOfStream {
		t.Error("expected EOS flag")
	}
}

func TestGenxTransformerBiStream_SendClosed(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	biStream.Close()

	ctx := context.Background()
	err := biStream.Send(ctx, &MessageChunk{Part: "test"})
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestGenxTransformerBiStream_CloseSend(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	err := biStream.CloseSend()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Input stream should be closed
	if !inputStream.closed {
		t.Error("expected input stream to be closed")
	}
}

func TestGenxTransformerBiStream_RecvText(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{
		{Part: genx.Text("response text")},
	}}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	chunk, err := biStream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "response text" {
		t.Errorf("expected 'response text', got %v", chunk.Part)
	}
}

func TestGenxTransformerBiStream_RecvBlob(t *testing.T) {
	data := []byte{0x01, 0x02}
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{
		{Part: &genx.Blob{MIMEType: "audio/mp3", Data: data}},
	}}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	chunk, err := biStream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	partMap, ok := chunk.Part.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", chunk.Part)
	}
	if partMap["type"] != "blob" {
		t.Errorf("expected type 'blob', got %v", partMap["type"])
	}
	if partMap["mime_type"] != "audio/mp3" {
		t.Errorf("expected mime_type 'audio/mp3', got %v", partMap["mime_type"])
	}
}

func TestGenxTransformerBiStream_RecvWithStreamCtrl(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{
		{
			Part: genx.Text("test"),
			Ctrl: &genx.StreamCtrl{
				StreamID:      "stream-abc",
				BeginOfStream: true,
				EndOfStream:   true,
			},
		},
	}}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	chunk, err := biStream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chunk.StreamID != "stream-abc" {
		t.Errorf("expected StreamID 'stream-abc', got %s", chunk.StreamID)
	}
	if !chunk.IsBOS {
		t.Error("expected IsBOS to be true")
	}
	if !chunk.IsEOS {
		t.Error("expected IsEOS to be true")
	}
}

func TestGenxTransformerBiStream_RecvNilChunk(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{nil}}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	_, err := biStream.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF for nil chunk, got %v", err)
	}
}

func TestGenxTransformerBiStream_RecvError(t *testing.T) {
	customErr := errors.New("recv error")
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{err: customErr}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	_, err := biStream.Recv(ctx)
	if err != customErr {
		t.Errorf("expected custom error, got %v", err)
	}
}

func TestGenxTransformerBiStream_RecvIOEOF(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{err: io.EOF}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	_, err := biStream.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF for io.EOF, got %v", err)
	}
}

func TestGenxTransformerBiStream_Close(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	err := biStream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !biStream.closed {
		t.Error("expected biStream to be closed")
	}
	if !inputStream.closed {
		t.Error("expected input stream to be closed")
	}
	if !outputStream.closed {
		t.Error("expected output stream to be closed")
	}

	// Double close should be safe
	err = biStream.Close()
	if err != nil {
		t.Errorf("unexpected error on double close: %v", err)
	}
}

func TestGenxTransformerBiStream_CloseNilOutput(t *testing.T) {
	inputStream := newGenxBufferStream(10)

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: nil,
	}

	err := biStream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test transformWithGenx
func TestTransformWithGenx_Success(t *testing.T) {
	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{
		{Part: genx.Text("output")},
	}}
	transformer := &mockTransformer{outputStream: outputStream}

	rt := &Runtime{
		ctx:             context.Background(),
		genxTransformer: transformer,
	}

	config := map[string]any{"model": "test-pattern"}
	biStream, err := rt.transformWithGenx(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if biStream == nil {
		t.Error("expected non-nil biStream")
	}
	if !transformer.called {
		t.Error("expected transformer to be called")
	}
	if transformer.pattern != "test-pattern" {
		t.Errorf("expected pattern 'test-pattern', got %s", transformer.pattern)
	}
}

func TestTransformWithGenx_MissingModel(t *testing.T) {
	transformer := &mockTransformer{}

	rt := &Runtime{
		ctx:             context.Background(),
		genxTransformer: transformer,
	}

	config := map[string]any{} // no model
	_, err := rt.transformWithGenx(config)
	if err == nil {
		t.Error("expected error for missing model")
	}
}

func TestTransformWithGenx_TransformError(t *testing.T) {
	transformer := &mockTransformer{err: errors.New("transform failed")}

	rt := &Runtime{
		ctx:             context.Background(),
		genxTransformer: transformer,
	}

	config := map[string]any{"model": "test"}
	_, err := rt.transformWithGenx(config)
	if err == nil {
		t.Error("expected error")
	}
	if err.Error() != "transform failed" {
		t.Errorf("expected 'transform failed', got %v", err)
	}
}

// Test pipeBiStream
func TestPipeBiStream_SendAndRecv(t *testing.T) {
	ctx := context.Background()

	// Create a pipe that echoes input to output
	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		for chunk := range input {
			output <- chunk
		}
	})

	// Send a chunk
	err := biStream.Send(ctx, &MessageChunk{Part: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	biStream.CloseSend()

	// Receive the echoed chunk
	chunk, err := biStream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "test" {
		t.Errorf("expected 'test', got %v", chunk.Part)
	}

	// After close, should get EOF
	_, err = biStream.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF, got %v", err)
	}
}

func TestPipeBiStream_SendClosed(t *testing.T) {
	ctx := context.Background()

	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		for range input {
		}
	})

	biStream.Close()

	err := biStream.Send(ctx, &MessageChunk{Part: "test"})
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestPipeBiStream_SendContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a pipe that doesn't read from input (will block)
	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		<-ctx.Done()
	})

	// Fill up the buffer
	for i := 0; i < 16; i++ {
		biStream.Send(ctx, &MessageChunk{Part: "test"})
	}

	// Cancel context
	cancel()

	// Now send should return context error
	err := biStream.Send(ctx, &MessageChunk{Part: "test"})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestPipeBiStream_RecvContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		// Don't write anything
		<-ctx.Done()
	})

	cancel()

	_, err := biStream.Recv(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestPipeBiStream_CloseSend(t *testing.T) {
	ctx := context.Background()

	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		for range input {
		}
	})

	err := biStream.CloseSend()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Double close should be safe
	err = biStream.CloseSend()
	if err != nil {
		t.Errorf("unexpected error on double CloseSend: %v", err)
	}
}

func TestPipeBiStream_Close(t *testing.T) {
	ctx := context.Background()

	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		for range input {
		}
	}).(*pipeBiStream)

	err := biStream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !biStream.closed {
		t.Error("expected stream to be closed")
	}

	// Double close should be safe
	err = biStream.Close()
	if err != nil {
		t.Errorf("unexpected error on double close: %v", err)
	}
}

// Test callbackBiStream
func TestCallbackBiStream_Send(t *testing.T) {
	sentChunks := []*MessageChunk{}
	biStream := NewCallbackBiStream(
		func(ctx context.Context, chunk *MessageChunk) error {
			sentChunks = append(sentChunks, chunk)
			return nil
		},
		func() error { return nil },
		func(ctx context.Context) (*MessageChunk, error) { return nil, ErrStreamEOF },
		func() error { return nil },
	)

	ctx := context.Background()
	biStream.Send(ctx, &MessageChunk{Part: "chunk1"})
	biStream.Send(ctx, &MessageChunk{Part: "chunk2"})

	if len(sentChunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(sentChunks))
	}
}

func TestCallbackBiStream_SendClosed(t *testing.T) {
	biStream := NewCallbackBiStream(
		func(ctx context.Context, chunk *MessageChunk) error { return nil },
		func() error { return nil },
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		func() error { return nil },
	)

	biStream.Close()

	ctx := context.Background()
	err := biStream.Send(ctx, &MessageChunk{Part: "test"})
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestCallbackBiStream_CloseSend(t *testing.T) {
	closeSendCalled := false
	biStream := NewCallbackBiStream(
		func(ctx context.Context, chunk *MessageChunk) error { return nil },
		func() error { closeSendCalled = true; return nil },
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		func() error { return nil },
	)

	biStream.CloseSend()

	if !closeSendCalled {
		t.Error("expected CloseSend callback to be called")
	}
}

func TestCallbackBiStream_CloseSendNil(t *testing.T) {
	biStream := NewCallbackBiStream(
		func(ctx context.Context, chunk *MessageChunk) error { return nil },
		nil, // nil closeSend
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		func() error { return nil },
	)

	err := biStream.CloseSend()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCallbackBiStream_Recv(t *testing.T) {
	count := 0
	biStream := NewCallbackBiStream(
		func(ctx context.Context, chunk *MessageChunk) error { return nil },
		func() error { return nil },
		func(ctx context.Context) (*MessageChunk, error) {
			count++
			if count > 1 {
				return nil, ErrStreamEOF
			}
			return &MessageChunk{Part: "received"}, nil
		},
		func() error { return nil },
	)

	ctx := context.Background()
	chunk, err := biStream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "received" {
		t.Errorf("expected 'received', got %v", chunk.Part)
	}

	_, err = biStream.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF, got %v", err)
	}
}

func TestCallbackBiStream_Close(t *testing.T) {
	closeCalled := false
	biStream := NewCallbackBiStream(
		func(ctx context.Context, chunk *MessageChunk) error { return nil },
		func() error { return nil },
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		func() error { closeCalled = true; return nil },
	)

	biStream.Close()

	if !closeCalled {
		t.Error("expected close callback to be called")
	}

	// Double close should be safe
	biStream.Close()
}

func TestCallbackBiStream_CloseNilFunc(t *testing.T) {
	biStream := NewCallbackBiStream(
		func(ctx context.Context, chunk *MessageChunk) error { return nil },
		func() error { return nil },
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		nil, // nil close
	)

	err := biStream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test WithGenxTransformer option
func TestWithGenxTransformer(t *testing.T) {
	transformer := &mockTransformer{}
	opt := WithGenxTransformer(transformer)

	rt := &Runtime{}
	opt(rt)

	if rt.genxTransformer != transformer {
		t.Error("expected genxTransformer to be set")
	}
}

// Test concurrent access to genxBufferStream
func TestGenxBufferStream_Concurrent(t *testing.T) {
	stream := newGenxBufferStream(100)

	// Writer goroutine
	go func() {
		for i := 0; i < 50; i++ {
			stream.Push(&genx.MessageChunk{Part: genx.Text("msg")})
		}
		stream.Close()
	}()

	// Reader goroutine
	count := 0
	for {
		_, err := stream.Next()
		if errors.Is(err, genx.ErrDone) {
			break
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			break
		}
		count++
	}

	if count != 50 {
		t.Errorf("expected 50 messages, got %d", count)
	}
}

// Test nil blob handling in Recv
func TestGenxTransformerBiStream_RecvNilBlob(t *testing.T) {
	inputStream := newGenxBufferStream(10)
	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{
		{Part: (*genx.Blob)(nil)},
	}}

	biStream := &genxTransformerBiStream{
		input:  inputStream,
		output: outputStream,
	}

	ctx := context.Background()
	chunk, err := biStream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Part should be nil for nil blob
	if chunk.Part != nil {
		t.Errorf("expected nil Part for nil blob, got %v", chunk.Part)
	}
}

// Test PipeBiStream close while sending
func TestPipeBiStream_CloseWhileSending(t *testing.T) {
	ctx := context.Background()

	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		time.Sleep(100 * time.Millisecond)
		for range input {
		}
	})

	// Close immediately
	biStream.Close()

	// Send after close should fail
	err := biStream.Send(ctx, &MessageChunk{Part: "test"})
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

// newLuauState creates a new Luau state for testing
func newLuauState(t *testing.T) *luau.State {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	return state
}

// Test builtinTransformer with empty name
func TestBuiltinTransformer_EmptyName(t *testing.T) {
	state := newLuauState(t)
	defer state.Close()
	state.OpenLibs()

	transformer := &mockTransformer{}
	rt := NewWithOptions(state, WithGenxTransformer(transformer))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err := state.DoString(`
local stream, err = __builtin.transformer("", {})
if err ~= "transformer name is required" then
    error("expected 'transformer name is required', got: " .. tostring(err))
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}

// Test builtinTransformer with genxTransformer
func TestBuiltinTransformer_WithGenxTransformer(t *testing.T) {
	state := newLuauState(t)
	defer state.Close()
	state.OpenLibs()

	outputStream := &mockGenxStream{chunks: []*genx.MessageChunk{
		{Part: genx.Text("transformed output")},
	}}
	transformer := &mockTransformer{outputStream: outputStream}

	rt := NewWithOptions(state, WithGenxTransformer(transformer))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err := state.DoString(`
local stream, err = __builtin.transformer("ignored", {model = "test-pattern"})
if err then
    error("transformer failed: " .. err)
end
if not stream then
    error("stream is nil")
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	if !transformer.called {
		t.Error("expected transformer to be called")
	}
	if transformer.pattern != "test-pattern" {
		t.Errorf("expected pattern 'test-pattern', got %s", transformer.pattern)
	}
}

// Test builtinTransformer with transformer error
func TestBuiltinTransformer_TransformerError(t *testing.T) {
	state := newLuauState(t)
	defer state.Close()
	state.OpenLibs()

	transformer := &mockTransformer{err: errors.New("transform failed")}

	rt := NewWithOptions(state, WithGenxTransformer(transformer))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err := state.DoString(`
local stream, err = __builtin.transformer("ignored", {model = "test"})
if err ~= "transform failed" then
    error("expected 'transform failed', got: " .. tostring(err))
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}

// Test builtinTransformer without transformer configured
func TestBuiltinTransformer_NoTransformer(t *testing.T) {
	state := newLuauState(t)
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err := state.DoString(`
local stream, err = __builtin.transformer("nonexistent", {})
if not err:find("unknown transformer") then
    error("expected 'unknown transformer' error, got: " .. tostring(err))
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}

// Test builtinTransformer with legacy TransformerFactory
func TestBuiltinTransformer_WithLegacyTransformerFactory(t *testing.T) {
	state := newLuauState(t)
	defer state.Close()
	state.OpenLibs()

	called := false
	ctx := context.Background()

	factory := func(ctx context.Context, config map[string]any) (BiStream, error) {
		called = true
		return NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
			for range input {
			}
		}), nil
	}

	rt := NewWithOptions(state, WithTransformer("test-transformer", factory))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err := state.DoString(`
local stream, err = __builtin.transformer("test-transformer", {})
if err then
    error("transformer failed: " .. err)
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	if !called {
		t.Error("expected legacy transformer factory to be called")
	}
	_ = ctx // silence unused variable
}

// Test WithTransformer option
func TestWithTransformer(t *testing.T) {
	factory := func(ctx context.Context, config map[string]any) (BiStream, error) {
		return nil, nil
	}

	rt := &Runtime{transformers: make(map[string]TransformerFactory)}
	opt := WithTransformer("test", factory)
	opt(rt)

	if rt.transformers["test"] == nil {
		t.Error("expected transformer to be registered")
	}
}

// Test pipeBiStream inputDone channel check in Close
func TestPipeBiStream_CloseInputDoneChannel(t *testing.T) {
	ctx := context.Background()

	biStream := NewPipeBiStream(ctx, func(ctx context.Context, input <-chan *MessageChunk, output chan<- *MessageChunk) {
		// Don't read from input, just exit
	}).(*pipeBiStream)

	// CloseSend first to close inputDone
	biStream.CloseSend()

	// Then Close should handle the already-closed inputDone
	err := biStream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
