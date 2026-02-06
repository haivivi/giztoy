package runtime

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// mockGenxStream implements genx.Stream for testing.
type mockGenxStream struct {
	chunks []*genx.MessageChunk
	idx    int
	err    error
	closed bool
}

func (s *mockGenxStream) Next() (*genx.MessageChunk, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.idx >= len(s.chunks) {
		return nil, genx.ErrDone
	}
	chunk := s.chunks[s.idx]
	s.idx++
	return chunk, nil
}

func (s *mockGenxStream) Close() error {
	s.closed = true
	return nil
}

func (s *mockGenxStream) CloseWithError(err error) error {
	s.closed = true
	s.err = err
	return nil
}

// mockGenerator implements genx.Generator for testing.
type mockGenerator struct {
	stream genx.Stream
	err    error
	called bool
	model  string
	mctx   genx.ModelContext
}

func (g *mockGenerator) GenerateStream(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
	g.called = true
	g.model = model
	g.mctx = mctx
	if g.err != nil {
		return nil, g.err
	}
	return g.stream, nil
}

func (g *mockGenerator) Invoke(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return genx.Usage{}, nil, errors.New("not implemented")
}

func TestGenxStreamAdapter_Recv_Text(t *testing.T) {
	chunks := []*genx.MessageChunk{
		{Part: genx.Text("Hello")},
		{Part: genx.Text(" World")},
	}
	stream := &mockGenxStream{chunks: chunks}
	adapter := &genxStreamAdapter{stream: stream}

	ctx := context.Background()

	// First chunk
	chunk, err := adapter.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "Hello" {
		t.Errorf("expected 'Hello', got %v", chunk.Part)
	}

	// Second chunk
	chunk, err = adapter.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != " World" {
		t.Errorf("expected ' World', got %v", chunk.Part)
	}

	// EOF
	_, err = adapter.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF, got %v", err)
	}
}

func TestGenxStreamAdapter_Recv_Blob(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	chunks := []*genx.MessageChunk{
		{Part: &genx.Blob{MIMEType: "audio/pcm", Data: data}},
	}
	stream := &mockGenxStream{chunks: chunks}
	adapter := &genxStreamAdapter{stream: stream}

	ctx := context.Background()
	chunk, err := adapter.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	partData, ok := chunk.Part.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", chunk.Part)
	}
	if len(partData) != 3 {
		t.Errorf("expected 3 bytes, got %d", len(partData))
	}
}

func TestGenxStreamAdapter_Recv_WithStreamCtrl(t *testing.T) {
	chunks := []*genx.MessageChunk{
		{
			Part: genx.Text("test"),
			Ctrl: &genx.StreamCtrl{
				StreamID:      "stream-123",
				BeginOfStream: true,
			},
		},
		{
			Part: genx.Text("end"),
			Ctrl: &genx.StreamCtrl{
				StreamID:    "stream-123",
				EndOfStream: true,
			},
		},
	}
	stream := &mockGenxStream{chunks: chunks}
	adapter := &genxStreamAdapter{stream: stream}

	ctx := context.Background()

	// First chunk with BOS
	chunk, err := adapter.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.StreamID != "stream-123" {
		t.Errorf("expected StreamID 'stream-123', got %s", chunk.StreamID)
	}
	if !chunk.IsBOS {
		t.Error("expected IsBOS to be true")
	}

	// Second chunk with EOS
	chunk, err = adapter.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !chunk.IsEOS {
		t.Error("expected IsEOS to be true")
	}
}

func TestGenxStreamAdapter_Recv_NilChunk(t *testing.T) {
	chunks := []*genx.MessageChunk{nil}
	stream := &mockGenxStream{chunks: chunks}
	adapter := &genxStreamAdapter{stream: stream}

	ctx := context.Background()
	_, err := adapter.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF for nil chunk, got %v", err)
	}
}

func TestGenxStreamAdapter_Recv_NilBlobPart(t *testing.T) {
	chunks := []*genx.MessageChunk{
		{Part: (*genx.Blob)(nil)},
	}
	stream := &mockGenxStream{chunks: chunks}
	adapter := &genxStreamAdapter{stream: stream}

	ctx := context.Background()
	chunk, err := adapter.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Part should be nil for nil blob
	if chunk.Part != nil {
		t.Errorf("expected nil Part for nil blob, got %v", chunk.Part)
	}
}

func TestGenxStreamAdapter_Recv_Error(t *testing.T) {
	customErr := errors.New("custom error")
	stream := &mockGenxStream{err: customErr}
	adapter := &genxStreamAdapter{stream: stream}

	ctx := context.Background()
	_, err := adapter.Recv(ctx)
	if err != customErr {
		t.Errorf("expected custom error, got %v", err)
	}
}

func TestGenxStreamAdapter_Recv_IOEOFError(t *testing.T) {
	stream := &mockGenxStream{err: io.EOF}
	adapter := &genxStreamAdapter{stream: stream}

	ctx := context.Background()
	_, err := adapter.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF for io.EOF, got %v", err)
	}
}

func TestGenxStreamAdapter_Close(t *testing.T) {
	stream := &mockGenxStream{}
	adapter := &genxStreamAdapter{stream: stream}

	err := adapter.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !stream.closed {
		t.Error("expected stream to be closed")
	}
}

func TestGenerateWithGenx_SystemPrompt(t *testing.T) {
	stream := &mockGenxStream{chunks: []*genx.MessageChunk{{Part: genx.Text("ok")}}}
	gen := &mockGenerator{stream: stream}

	rt := &Runtime{
		ctx:           context.Background(),
		genxGenerator: gen,
	}

	mctx := map[string]any{
		"system": "You are a helpful assistant",
	}

	_, err := rt.generateWithGenx("test-model", mctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !gen.called {
		t.Error("expected generator to be called")
	}
	if gen.model != "test-model" {
		t.Errorf("expected model 'test-model', got %s", gen.model)
	}
}

func TestGenerateWithGenx_Messages(t *testing.T) {
	stream := &mockGenxStream{chunks: []*genx.MessageChunk{{Part: genx.Text("ok")}}}
	gen := &mockGenerator{stream: stream}

	rt := &Runtime{
		ctx:           context.Background(),
		genxGenerator: gen,
	}

	mctx := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "Hello"},
			map[string]any{"role": "assistant", "content": "Hi there"},
			map[string]any{"role": "model", "content": "Another response"},
			map[string]any{"role": "system", "content": "System message"},
		},
	}

	result, err := rt.generateWithGenx("test-model", mctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestGenerateWithGenx_Error(t *testing.T) {
	gen := &mockGenerator{err: errors.New("generation failed")}

	rt := &Runtime{
		ctx:           context.Background(),
		genxGenerator: gen,
	}

	_, err := rt.generateWithGenx("test-model", nil)
	if err == nil {
		t.Error("expected error")
	}
	if err.Error() != "generation failed" {
		t.Errorf("expected 'generation failed', got %v", err)
	}
}

func TestGenerateWithGenx_EmptyMctx(t *testing.T) {
	stream := &mockGenxStream{chunks: []*genx.MessageChunk{{Part: genx.Text("ok")}}}
	gen := &mockGenerator{stream: stream}

	rt := &Runtime{
		ctx:           context.Background(),
		genxGenerator: gen,
	}

	_, err := rt.generateWithGenx("test-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test channelStream
func TestChannelStream_Recv(t *testing.T) {
	ch := make(chan *MessageChunk, 2)
	ch <- &MessageChunk{Part: "chunk1"}
	ch <- &MessageChunk{Part: "chunk2"}
	close(ch)

	stream := NewChannelStream(ch)
	ctx := context.Background()

	chunk, err := stream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "chunk1" {
		t.Errorf("expected 'chunk1', got %v", chunk.Part)
	}

	chunk, err = stream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "chunk2" {
		t.Errorf("expected 'chunk2', got %v", chunk.Part)
	}

	// EOF after channel close
	_, err = stream.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF, got %v", err)
	}
}

func TestChannelStream_Recv_Closed(t *testing.T) {
	ch := make(chan *MessageChunk, 1)
	stream := NewChannelStream(ch)
	stream.Close()

	ctx := context.Background()
	_, err := stream.Recv(ctx)
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestChannelStream_Recv_ContextCanceled(t *testing.T) {
	ch := make(chan *MessageChunk) // unbuffered, will block
	stream := NewChannelStream(ch)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := stream.Recv(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestChannelStream_Close(t *testing.T) {
	ch := make(chan *MessageChunk, 1)
	stream := NewChannelStream(ch).(*channelStream)

	err := stream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !stream.closed {
		t.Error("expected stream to be closed")
	}

	// Double close should be safe
	err = stream.Close()
	if err != nil {
		t.Errorf("unexpected error on double close: %v", err)
	}
}

// Test callbackStream
func TestCallbackStream_Recv(t *testing.T) {
	count := 0
	stream := NewCallbackStream(
		func(ctx context.Context) (*MessageChunk, error) {
			count++
			if count > 2 {
				return nil, ErrStreamEOF
			}
			return &MessageChunk{Part: count}, nil
		},
		func() error { return nil },
	)

	ctx := context.Background()
	chunk, _ := stream.Recv(ctx)
	if chunk.Part != 1 {
		t.Errorf("expected 1, got %v", chunk.Part)
	}

	chunk, _ = stream.Recv(ctx)
	if chunk.Part != 2 {
		t.Errorf("expected 2, got %v", chunk.Part)
	}

	_, err := stream.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF, got %v", err)
	}
}

func TestCallbackStream_Close(t *testing.T) {
	closeCalled := false
	stream := NewCallbackStream(
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		func() error { closeCalled = true; return nil },
	)

	stream.Close()
	if !closeCalled {
		t.Error("expected close callback to be called")
	}

	// After close, Recv should return ErrStreamClosed
	ctx := context.Background()
	_, err := stream.Recv(ctx)
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestCallbackStream_Close_NilCloseFunc(t *testing.T) {
	stream := NewCallbackStream(
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		nil,
	)

	err := stream.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test simpleStream
func TestSimpleStream_Recv(t *testing.T) {
	chunks := []*MessageChunk{
		{Part: "a"},
		{Part: "b"},
	}
	stream := NewSimpleStream(chunks)
	ctx := context.Background()

	chunk, err := stream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "a" {
		t.Errorf("expected 'a', got %v", chunk.Part)
	}

	chunk, err = stream.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Part != "b" {
		t.Errorf("expected 'b', got %v", chunk.Part)
	}

	_, err = stream.Recv(ctx)
	if err != ErrStreamEOF {
		t.Errorf("expected ErrStreamEOF, got %v", err)
	}
}

func TestSimpleStream_Close(t *testing.T) {
	stream := NewSimpleStream([]*MessageChunk{{Part: "test"}})
	stream.Close()

	ctx := context.Background()
	_, err := stream.Recv(ctx)
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

// Test WithGenxGenerator option
func TestWithGenxGenerator(t *testing.T) {
	gen := &mockGenerator{}
	opt := WithGenxGenerator(gen)

	rt := &Runtime{}
	opt(rt)

	if rt.genxGenerator != gen {
		t.Error("expected genxGenerator to be set")
	}
}

// Test luaTableToMap via Lua script execution (safer approach)
func TestLuaTableToMap_ViaScript(t *testing.T) {
	state, err := NewLuauState()
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	tc := rt.CreateToolContext()

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Test nested table through input/output mechanism
	tc.SetInput(map[string]any{
		"nested": map[string]any{
			"value": 123,
		},
		"array": []any{1, 2, 3},
		"bool":  true,
	})

	err = state.DoString(`
local input = rt:input()
rt:output({
    nested_value = input.nested.value,
    bool_value = input.bool,
}, nil)
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	output, err := tc.GetOutput()
	if err != nil {
		t.Fatalf("GetOutput failed: %v", err)
	}

	outputMap := output.(map[string]any)
	if outputMap["nested_value"] != float64(123) {
		t.Errorf("expected nested_value=123, got %v", outputMap["nested_value"])
	}
	if outputMap["bool_value"] != true {
		t.Errorf("expected bool_value=true, got %v", outputMap["bool_value"])
	}
}

// Test channelStream done channel signaling
func TestChannelStream_DoneChannel(t *testing.T) {
	ch := make(chan *MessageChunk, 1)
	stream := NewChannelStream(ch).(*channelStream)

	// Close done channel directly by closing stream
	stream.Close()

	// Recv should return ErrStreamClosed via done channel
	ctx := context.Background()
	_, err := stream.Recv(ctx)
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

// Test callbackStream double close
func TestCallbackStream_DoubleClose(t *testing.T) {
	closeCount := 0
	stream := NewCallbackStream(
		func(ctx context.Context) (*MessageChunk, error) { return nil, nil },
		func() error { closeCount++; return nil },
	)

	stream.Close()
	stream.Close()

	if closeCount != 1 {
		t.Errorf("expected close to be called once, got %d", closeCount)
	}
}

// NewLuauState creates a new Luau state for testing
func NewLuauState() (*luau.State, error) {
	return luau.New()
}

// Test builtinGenerate without generator configured
func TestBuiltinGenerate_NoGenerator(t *testing.T) {
	state, err := NewLuauState()
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Try to call generate without a generator configured
	err = state.DoString(`
local stream, err = __builtin.generate("test-model", {})
if err ~= "generator not configured" then
    error("expected 'generator not configured', got: " .. tostring(err))
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}

// Test builtinGenerate with empty model
func TestBuiltinGenerate_EmptyModel(t *testing.T) {
	state, err := NewLuauState()
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	gen := &mockGenerator{}
	rt := NewWithOptions(state, WithGenxGenerator(gen))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = state.DoString(`
local stream, err = __builtin.generate("", {})
if err ~= "model is required" then
    error("expected 'model is required', got: " .. tostring(err))
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}

// Test builtinGenerate with genxGenerator
func TestBuiltinGenerate_WithGenxGenerator(t *testing.T) {
	state, err := NewLuauState()
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	chunks := []*genx.MessageChunk{
		{Part: genx.Text("Hello")},
		{Part: genx.Text(" World")},
	}
	mockStream := &mockGenxStream{chunks: chunks}
	gen := &mockGenerator{stream: mockStream}

	rt := NewWithOptions(state, WithGenxGenerator(gen))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = state.DoString(`
local stream, err = __builtin.generate("test-model", {
    system = "You are helpful",
    messages = {
        {role = "user", content = "Hello"},
        {role = "assistant", content = "Hi"},
    }
})
if err then
    error("generate failed: " .. err)
end
if not stream then
    error("stream is nil")
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	if !gen.called {
		t.Error("expected generator to be called")
	}
	if gen.model != "test-model" {
		t.Errorf("expected model 'test-model', got %s", gen.model)
	}
}

// Test builtinGenerate error from generator
func TestBuiltinGenerate_GeneratorError(t *testing.T) {
	state, err := NewLuauState()
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	gen := &mockGenerator{err: errors.New("generation failed")}

	rt := NewWithOptions(state, WithGenxGenerator(gen))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = state.DoString(`
local stream, err = __builtin.generate("test-model", {})
if err ~= "generation failed" then
    error("expected 'generation failed', got: " .. tostring(err))
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}

// Test builtinGenerate with legacy GeneratorFunc
func TestBuiltinGenerate_WithLegacyGeneratorFunc(t *testing.T) {
	state, err := NewLuauState()
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	called := false
	legacyGen := func(ctx context.Context, model string, mctx map[string]any) (Stream, error) {
		called = true
		return NewSimpleStream([]*MessageChunk{{Part: "test"}}), nil
	}

	rt := NewWithOptions(state, WithGenerator(legacyGen))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = state.DoString(`
local stream, err = __builtin.generate("test-model", {})
if err then
    error("generate failed: " .. err)
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	if !called {
		t.Error("expected legacy generator to be called")
	}
}

// Test WithGenerator option
func TestWithGenerator(t *testing.T) {
	gen := func(ctx context.Context, model string, mctx map[string]any) (Stream, error) {
		return nil, nil
	}

	rt := &Runtime{}
	opt := WithGenerator(gen)
	opt(rt)

	if rt.generator == nil {
		t.Error("expected generator to be set")
	}
}

// Test message roles in generateWithGenx
func TestGenerateWithGenx_MessageRoles(t *testing.T) {
	stream := &mockGenxStream{chunks: []*genx.MessageChunk{{Part: genx.Text("ok")}}}
	gen := &mockGenerator{stream: stream}

	rt := &Runtime{
		ctx:           context.Background(),
		genxGenerator: gen,
	}

	// Test all role types
	mctx := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "msg1"},
			map[string]any{"role": "assistant", "content": "msg2"},
			map[string]any{"role": "model", "content": "msg3"},
			map[string]any{"role": "system", "content": "msg4"},
			map[string]any{"role": "unknown", "content": "msg5"}, // should be ignored
			map[string]any{"content": "no role"},                 // missing role
			map[string]any{"role": "user"},                       // missing content
		},
	}

	_, err := rt.generateWithGenx("test", mctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
