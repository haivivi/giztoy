package runtime

import (
	"context"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// mockStream implements Stream for testing.
type mockStream struct {
	chunks []*MessageChunk
	idx    int
	closed bool
}

func (m *mockStream) Recv(ctx context.Context) (*MessageChunk, error) {
	if m.closed {
		return nil, ErrStreamClosed
	}
	if m.idx >= len(m.chunks) {
		return nil, ErrStreamEOF
	}
	chunk := m.chunks[m.idx]
	m.idx++
	return chunk, nil
}

func (m *mockStream) Close() error {
	m.closed = true
	return nil
}

// mockBiStream implements BiStream for testing.
type mockBiStream struct {
	mockStream
	sent []*MessageChunk
}

func (m *mockBiStream) Send(ctx context.Context, chunk *MessageChunk) error {
	if m.closed {
		return ErrStreamClosed
	}
	m.sent = append(m.sent, chunk)
	return nil
}

func (m *mockBiStream) CloseSend() error {
	return nil
}

func TestStreamRegistry(t *testing.T) {
	reg := newStreamRegistry()

	t.Run("register and get stream", func(t *testing.T) {
		stream := &mockStream{chunks: []*MessageChunk{{Part: "test"}}}
		ctx := context.Background()

		id := reg.registerStream(ctx, stream)
		if id == 0 {
			t.Error("expected non-zero id")
		}

		handle, ok := reg.getStream(id)
		if !ok {
			t.Error("expected to find stream")
		}
		if handle.id != id {
			t.Errorf("expected id %d, got %d", id, handle.id)
		}
	})

	t.Run("get non-existent stream", func(t *testing.T) {
		_, ok := reg.getStream(99999)
		if ok {
			t.Error("expected not to find stream")
		}
	})

	t.Run("register and get bistream", func(t *testing.T) {
		bistream := &mockBiStream{mockStream: mockStream{chunks: []*MessageChunk{{Part: "test"}}}}
		ctx := context.Background()

		id := reg.registerBiStream(ctx, bistream)
		if id == 0 {
			t.Error("expected non-zero id")
		}

		handle, ok := reg.getBiStream(id)
		if !ok {
			t.Error("expected to find bistream")
		}
		if handle.id != id {
			t.Errorf("expected id %d, got %d", id, handle.id)
		}
	})

	t.Run("get non-existent bistream", func(t *testing.T) {
		_, ok := reg.getBiStream(99999)
		if ok {
			t.Error("expected not to find bistream")
		}
	})

	t.Run("close stream", func(t *testing.T) {
		stream := &mockStream{chunks: []*MessageChunk{{Part: "test"}}}
		ctx := context.Background()

		id := reg.registerStream(ctx, stream)
		reg.closeStream(id)

		_, ok := reg.getStream(id)
		if ok {
			t.Error("expected stream to be removed after close")
		}
		if !stream.closed {
			t.Error("expected stream.Close() to be called")
		}
	})

	t.Run("close bistream", func(t *testing.T) {
		bistream := &mockBiStream{mockStream: mockStream{chunks: []*MessageChunk{{Part: "test"}}}}
		ctx := context.Background()

		id := reg.registerBiStream(ctx, bistream)
		reg.closeBiStream(id)

		_, ok := reg.getBiStream(id)
		if ok {
			t.Error("expected bistream to be removed after close")
		}
		if !bistream.closed {
			t.Error("expected bistream.Close() to be called")
		}
	})

	t.Run("close non-existent stream", func(t *testing.T) {
		// Should not panic
		reg.closeStream(99999)
	})

	t.Run("close non-existent bistream", func(t *testing.T) {
		// Should not panic
		reg.closeBiStream(99999)
	})
}

func TestPushMessageChunk(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	t.Run("nil chunk", func(t *testing.T) {
		pushMessageChunk(state, nil)
		if !state.IsNil(-1) {
			t.Error("expected nil for nil chunk")
		}
		state.Pop(1)
	})

	t.Run("text part", func(t *testing.T) {
		chunk := &MessageChunk{Part: "hello"}
		pushMessageChunk(state, chunk)

		if !state.IsTable(-1) {
			t.Error("expected table")
		}

		state.GetField(-1, "part")
		if !state.IsTable(-1) {
			t.Error("expected part to be a table")
		}

		state.GetField(-1, "type")
		if state.ToString(-1) != "text" {
			t.Error("expected type to be 'text'")
		}
		state.Pop(1)

		state.GetField(-1, "value")
		if state.ToString(-1) != "hello" {
			t.Error("expected value to be 'hello'")
		}
		state.Pop(3)
	})

	t.Run("blob part", func(t *testing.T) {
		chunk := &MessageChunk{Part: []byte{1, 2, 3}}
		pushMessageChunk(state, chunk)

		if !state.IsTable(-1) {
			t.Error("expected table")
		}

		state.GetField(-1, "part")
		state.GetField(-1, "type")
		if state.ToString(-1) != "blob" {
			t.Error("expected type to be 'blob'")
		}
		state.Pop(1)

		state.GetField(-1, "data")
		data := state.ToBytes(-1)
		if len(data) != 3 || data[0] != 1 || data[1] != 2 || data[2] != 3 {
			t.Error("expected data to be [1,2,3]")
		}
		state.Pop(3)
	})

	t.Run("structured part", func(t *testing.T) {
		chunk := &MessageChunk{Part: map[string]any{"key": "value"}}
		pushMessageChunk(state, chunk)

		state.GetField(-1, "part")
		if !state.IsTable(-1) {
			t.Error("expected part to be a table")
		}
		state.Pop(2)
	})

	t.Run("with stream_id", func(t *testing.T) {
		chunk := &MessageChunk{StreamID: "stream-123"}
		pushMessageChunk(state, chunk)

		state.GetField(-1, "stream_id")
		if state.ToString(-1) != "stream-123" {
			t.Error("expected stream_id to be 'stream-123'")
		}
		state.Pop(2)
	})

	t.Run("with is_bos", func(t *testing.T) {
		chunk := &MessageChunk{IsBOS: true}
		pushMessageChunk(state, chunk)

		state.GetField(-1, "is_bos")
		if !state.ToBoolean(-1) {
			t.Error("expected is_bos to be true")
		}
		state.Pop(2)
	})

	t.Run("with is_eos", func(t *testing.T) {
		chunk := &MessageChunk{IsEOS: true}
		pushMessageChunk(state, chunk)

		state.GetField(-1, "is_eos")
		if !state.ToBoolean(-1) {
			t.Error("expected is_eos to be true")
		}
		state.Pop(2)
	})

	t.Run("nil part (no part field set)", func(t *testing.T) {
		chunk := &MessageChunk{StreamID: "test"}
		pushMessageChunk(state, chunk)

		state.GetField(-1, "part")
		if !state.IsNil(-1) {
			t.Error("expected part to be nil")
		}
		state.Pop(2)
	})
}

func TestParseMessageChunk(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	t.Run("nil input", func(t *testing.T) {
		state.PushNil()
		chunk := parseMessageChunk(state, -1)
		if chunk != nil {
			t.Error("expected nil chunk")
		}
		state.Pop(1)
	})

	t.Run("text part", func(t *testing.T) {
		state.NewTable()
		state.NewTable()
		state.PushString("text")
		state.SetField(-2, "type")
		state.PushString("hello")
		state.SetField(-2, "value")
		state.SetField(-2, "part")

		chunk := parseMessageChunk(state, -1)
		if chunk == nil {
			t.Fatal("expected non-nil chunk")
		}
		if chunk.Part != "hello" {
			t.Errorf("expected part to be 'hello', got %v", chunk.Part)
		}
		state.Pop(1)
	})

	t.Run("blob part", func(t *testing.T) {
		state.NewTable()
		state.NewTable()
		state.PushString("blob")
		state.SetField(-2, "type")
		state.PushBytes([]byte{1, 2, 3})
		state.SetField(-2, "data")
		state.SetField(-2, "part")

		chunk := parseMessageChunk(state, -1)
		if chunk == nil {
			t.Fatal("expected non-nil chunk")
		}
		data, ok := chunk.Part.([]byte)
		if !ok {
			t.Fatalf("expected part to be []byte, got %T", chunk.Part)
		}
		if len(data) != 3 || data[0] != 1 {
			t.Error("expected data to be [1,2,3]")
		}
		state.Pop(1)
	})

	t.Run("string part (not table)", func(t *testing.T) {
		state.NewTable()
		state.PushString("direct-string")
		state.SetField(-2, "part")

		chunk := parseMessageChunk(state, -1)
		if chunk == nil {
			t.Fatal("expected non-nil chunk")
		}
		if chunk.Part != "direct-string" {
			t.Errorf("expected part to be 'direct-string', got %v", chunk.Part)
		}
		state.Pop(1)
	})

	t.Run("with stream_id", func(t *testing.T) {
		state.NewTable()
		state.PushString("stream-456")
		state.SetField(-2, "stream_id")

		chunk := parseMessageChunk(state, -1)
		if chunk.StreamID != "stream-456" {
			t.Errorf("expected stream_id 'stream-456', got %s", chunk.StreamID)
		}
		state.Pop(1)
	})

	t.Run("with is_bos and is_eos", func(t *testing.T) {
		state.NewTable()
		state.PushBoolean(true)
		state.SetField(-2, "is_bos")
		state.PushBoolean(true)
		state.SetField(-2, "is_eos")

		chunk := parseMessageChunk(state, -1)
		if !chunk.IsBOS {
			t.Error("expected is_bos to be true")
		}
		if !chunk.IsEOS {
			t.Error("expected is_eos to be true")
		}
		state.Pop(1)
	})

	t.Run("negative index", func(t *testing.T) {
		state.PushInteger(123) // dummy value
		state.NewTable()
		state.PushString("test-stream")
		state.SetField(-2, "stream_id")

		// Parse using negative index
		chunk := parseMessageChunk(state, -1)
		if chunk.StreamID != "test-stream" {
			t.Errorf("expected stream_id 'test-stream', got %s", chunk.StreamID)
		}
		state.Pop(2)
	})

	t.Run("unknown part type", func(t *testing.T) {
		state.NewTable()
		state.NewTable()
		state.PushString("unknown")
		state.SetField(-2, "type")
		state.PushString("some-data")
		state.SetField(-2, "value")
		state.SetField(-2, "part")

		chunk := parseMessageChunk(state, -1)
		if chunk == nil {
			t.Fatal("expected non-nil chunk")
		}
		// Should fall through to luaToGo
		state.Pop(1)
	})
}
