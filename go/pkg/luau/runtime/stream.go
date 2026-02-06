package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

var (
	// ErrStreamClosed is returned when operating on a closed stream.
	ErrStreamClosed = errors.New("stream is closed")
	// ErrStreamEOF is returned when the stream has no more data.
	ErrStreamEOF = errors.New("end of stream")
)

// MessageChunk represents a chunk in a stream.
// This is the Luau-side representation of genx.MessageChunk.
type MessageChunk struct {
	Part     any    // Text (string), Blob ([]byte with mime), or structured data
	StreamID string // Optional stream identifier
	IsBOS    bool   // Begin of stream marker
	IsEOS    bool   // End of stream marker
}

// Stream is a read-only stream of MessageChunks.
// Used for LLM generation output.
type Stream interface {
	// Recv receives the next chunk from the stream.
	// Returns nil, ErrStreamEOF when done.
	Recv(ctx context.Context) (*MessageChunk, error)
	// Close closes the stream.
	Close() error
}

// BiStream is a bidirectional stream for transformers.
// Supports both sending input and receiving output.
type BiStream interface {
	Stream
	// Send sends a chunk to the stream.
	Send(ctx context.Context, chunk *MessageChunk) error
	// CloseSend signals no more input will be sent.
	CloseSend() error
}

// streamHandle wraps a Stream for use in Luau.
type streamHandle struct {
	id     uint64
	stream Stream
	ctx    context.Context
	cancel context.CancelFunc
}

// biStreamHandle wraps a BiStream for use in Luau.
type biStreamHandle struct {
	streamHandle
	biStream BiStream
}

// streamRegistry manages active streams in the runtime.
type streamRegistry struct {
	mu       sync.Mutex
	streams  map[uint64]*streamHandle
	biStreams map[uint64]*biStreamHandle
	nextID   uint64
}

func newStreamRegistry() *streamRegistry {
	return &streamRegistry{
		streams:   make(map[uint64]*streamHandle),
		biStreams: make(map[uint64]*biStreamHandle),
	}
}

func (r *streamRegistry) registerStream(ctx context.Context, s Stream) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	id := r.nextID

	ctx, cancel := context.WithCancel(ctx)
	r.streams[id] = &streamHandle{
		id:     id,
		stream: s,
		ctx:    ctx,
		cancel: cancel,
	}

	return id
}

func (r *streamRegistry) registerBiStream(ctx context.Context, s BiStream) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	id := r.nextID

	ctx, cancel := context.WithCancel(ctx)
	handle := &biStreamHandle{
		streamHandle: streamHandle{
			id:     id,
			stream: s,
			ctx:    ctx,
			cancel: cancel,
		},
		biStream: s,
	}
	r.biStreams[id] = handle

	return id
}

func (r *streamRegistry) getStream(id uint64) (*streamHandle, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.streams[id]
	return h, ok
}

func (r *streamRegistry) getBiStream(id uint64) (*biStreamHandle, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.biStreams[id]
	return h, ok
}

func (r *streamRegistry) closeStream(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if h, ok := r.streams[id]; ok {
		h.cancel()
		h.stream.Close()
		delete(r.streams, id)
	}
}

func (r *streamRegistry) closeBiStream(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if h, ok := r.biStreams[id]; ok {
		h.cancel()
		h.biStream.Close()
		delete(r.biStreams, id)
	}
}

// pushMessageChunk pushes a MessageChunk onto the Lua stack as a table.
func pushMessageChunk(state *luau.State, chunk *MessageChunk) {
	if chunk == nil {
		state.PushNil()
		return
	}

	state.NewTable()

	// part
	if chunk.Part != nil {
		switch p := chunk.Part.(type) {
		case string:
			state.NewTable()
			state.PushString("text")
			state.SetField(-2, "type")
			state.PushString(p)
			state.SetField(-2, "value")
			state.SetField(-2, "part")
		case []byte:
			state.NewTable()
			state.PushString("blob")
			state.SetField(-2, "type")
			state.PushBytes(p)
			state.SetField(-2, "data")
			state.SetField(-2, "part")
		default:
			goToLua(state, p)
			state.SetField(-2, "part")
		}
	}

	// stream_id
	if chunk.StreamID != "" {
		state.PushString(chunk.StreamID)
		state.SetField(-2, "stream_id")
	}

	// is_bos
	if chunk.IsBOS {
		state.PushBoolean(true)
		state.SetField(-2, "is_bos")
	}

	// is_eos
	if chunk.IsEOS {
		state.PushBoolean(true)
		state.SetField(-2, "is_eos")
	}
}

// messageChunkToMap converts a MessageChunk to a map for Lua.
func messageChunkToMap(chunk *MessageChunk) map[string]any {
	if chunk == nil {
		return nil
	}

	m := make(map[string]any)

	// part
	if chunk.Part != nil {
		switch p := chunk.Part.(type) {
		case string:
			m["part"] = map[string]any{
				"type":  "text",
				"value": p,
			}
		case []byte:
			m["part"] = map[string]any{
				"type": "blob",
				"data": p,
			}
		default:
			m["part"] = p
		}
	}

	// stream_id
	if chunk.StreamID != "" {
		m["stream_id"] = chunk.StreamID
	}

	// is_bos
	if chunk.IsBOS {
		m["is_bos"] = true
	}

	// is_eos
	if chunk.IsEOS {
		m["is_eos"] = true
	}

	return m
}

// parseMessageChunk parses a Lua table at idx into a MessageChunk.
func parseMessageChunk(state *luau.State, idx int) *MessageChunk {
	if state.IsNil(idx) {
		return nil
	}

	if idx < 0 {
		idx = state.GetTop() + idx + 1
	}

	chunk := &MessageChunk{}

	// part
	state.GetField(idx, "part")
	if !state.IsNil(-1) {
		if state.IsTable(-1) {
			state.GetField(-1, "type")
			partType := state.ToString(-1)
			state.Pop(1)

			switch partType {
			case "text":
				state.GetField(-1, "value")
				chunk.Part = state.ToString(-1)
				state.Pop(1)
			case "blob":
				state.GetField(-1, "data")
				chunk.Part = state.ToBytes(-1)
				state.Pop(1)
			default:
				chunk.Part = luaToGo(state, -1)
			}
		} else if state.IsString(-1) {
			chunk.Part = state.ToString(-1)
		}
	}
	state.Pop(1)

	// stream_id
	state.GetField(idx, "stream_id")
	if state.IsString(-1) {
		chunk.StreamID = state.ToString(-1)
	}
	state.Pop(1)

	// is_bos
	state.GetField(idx, "is_bos")
	chunk.IsBOS = state.ToBoolean(-1)
	state.Pop(1)

	// is_eos
	state.GetField(idx, "is_eos")
	chunk.IsEOS = state.ToBoolean(-1)
	state.Pop(1)

	return chunk
}

// builtinStreamRecv implements stream:recv() -> Promise
// Returns a Promise that resolves to (chunk, err).
// Usage: local chunk, err = stream:recv():await()
func (rt *Runtime) builtinStreamRecv(state *luau.State) int {
	// Get stream ID from first argument (self)
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	handle, ok := rt.streams.getStream(id)
	if !ok {
		// Return immediately rejected promise
		promise := rt.promises.newPromise()
		promise.Resolve(nil, "stream not found")
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Create a promise for the recv operation
	promise := rt.promises.newPromise()

	// Run recv in goroutine
	go func() {
		chunk, err := handle.stream.Recv(handle.ctx)
		if err != nil {
			if errors.Is(err, ErrStreamEOF) {
				promise.Resolve(nil, nil) // nil chunk, nil error = EOF
			} else {
				promise.Resolve(nil, err.Error())
			}
			return
		}
		promise.Resolve(messageChunkToMap(chunk), nil)
	}()

	rt.pushPromiseObject(state, promise)
	return 1
}

// builtinStreamClose implements stream:close()
func (rt *Runtime) builtinStreamClose(state *luau.State) int {
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	rt.streams.closeStream(id)
	return 0
}

// builtinBiStreamSend implements bistream:send(chunk) -> Promise
// Returns a Promise that resolves to err (nil on success).
// Usage: local err = bistream:send(chunk):await()
func (rt *Runtime) builtinBiStreamSend(state *luau.State) int {
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	handle, ok := rt.streams.getBiStream(id)
	if !ok {
		promise := rt.promises.newPromise()
		promise.Resolve("stream not found")
		rt.pushPromiseObject(state, promise)
		return 1
	}

	chunk := parseMessageChunk(state, 2)
	if chunk == nil {
		promise := rt.promises.newPromise()
		promise.Resolve("invalid chunk")
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Create a promise for the send operation
	promise := rt.promises.newPromise()

	// Run send in goroutine
	go func() {
		if err := handle.biStream.Send(handle.ctx, chunk); err != nil {
			promise.Resolve(err.Error())
		} else {
			promise.Resolve(nil) // no error
		}
	}()

	rt.pushPromiseObject(state, promise)
	return 1
}

// builtinBiStreamCloseSend implements bistream:close_send()
func (rt *Runtime) builtinBiStreamCloseSend(state *luau.State) int {
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	handle, ok := rt.streams.getBiStream(id)
	if !ok {
		return 0
	}

	handle.biStream.CloseSend()
	return 0
}

// builtinBiStreamRecv implements bistream:recv() -> Promise
// Returns a Promise that resolves to (chunk, err).
// Usage: local chunk, err = bistream:recv():await()
func (rt *Runtime) builtinBiStreamRecv(state *luau.State) int {
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	handle, ok := rt.streams.getBiStream(id)
	if !ok {
		promise := rt.promises.newPromise()
		promise.Resolve(nil, "stream not found")
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Create a promise for the recv operation
	promise := rt.promises.newPromise()

	// Run recv in goroutine
	go func() {
		chunk, err := handle.biStream.Recv(handle.ctx)
		if err != nil {
			if errors.Is(err, ErrStreamEOF) {
				promise.Resolve(nil, nil) // nil chunk, nil error = EOF
			} else {
				promise.Resolve(nil, err.Error())
			}
			return
		}
		promise.Resolve(messageChunkToMap(chunk), nil)
	}()

	rt.pushPromiseObject(state, promise)
	return 1
}

// builtinBiStreamClose implements bistream:close()
func (rt *Runtime) builtinBiStreamClose(state *luau.State) int {
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	rt.streams.closeBiStream(id)
	return 0
}

// pushStreamObject creates a Lua stream object with methods.
// Stream methods (__stream_recv, __stream_close) must be pre-registered
// in RegisterBuiltins() before calling this function.
func (rt *Runtime) pushStreamObject(state *luau.State, id uint64) {
	state.NewTable()

	// _id field
	state.PushInteger(int64(id))
	state.SetField(-2, "_id")

	// recv method (pre-registered in RegisterBuiltins)
	state.GetGlobal("__stream_recv")
	state.SetField(-2, "recv")

	// close method (pre-registered in RegisterBuiltins)
	state.GetGlobal("__stream_close")
	state.SetField(-2, "close")
}

// pushBiStreamObject creates a Lua bistream object with methods.
// BiStream methods (__bistream_send, __bistream_close_send, __bistream_recv,
// __bistream_close) must be pre-registered in RegisterBuiltins() before
// calling this function.
func (rt *Runtime) pushBiStreamObject(state *luau.State, id uint64) {
	state.NewTable()

	// _id field
	state.PushInteger(int64(id))
	state.SetField(-2, "_id")

	// send method (pre-registered in RegisterBuiltins)
	state.GetGlobal("__bistream_send")
	state.SetField(-2, "send")

	// close_send method (pre-registered in RegisterBuiltins)
	state.GetGlobal("__bistream_close_send")
	state.SetField(-2, "close_send")

	// recv method (pre-registered in RegisterBuiltins)
	state.GetGlobal("__bistream_recv")
	state.SetField(-2, "recv")

	// close method (pre-registered in RegisterBuiltins)
	state.GetGlobal("__bistream_close")
	state.SetField(-2, "close")
}
