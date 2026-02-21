package genx

import (
	"context"
	"fmt"
	"slices"
)

// Role constants define the producer of a message.
const (
	// RoleUser indicates the message is from a user.
	RoleUser Role = "user"
	// RoleModel indicates the message is from an AI model.
	RoleModel Role = "model"
	// RoleTool indicates the message is a tool result.
	RoleTool Role = "tool"
)

var (
	_ Payload = (*Contents)(nil)
	_ Payload = (*ToolCall)(nil)
	_ Payload = (*ToolResult)(nil)

	_ Part = (*Blob)(nil)
	_ Part = (*Text)(nil)
)

// MessageChunk represents a chunk in a genx Stream.
//
// Fields:
//   - Role: The producer of this message (user, model, or tool)
//   - Name: The name of the producer (e.g., "alice", "assistant", "weather")
//   - Part: The content payload (Text or Blob)
//   - ToolCall: Tool invocation data (for model calling tools)
//   - Ctrl: Stream control signals (optional, for routing and state)
//
// Transformer Contract:
//
// When a MessageChunk passes through a Transformer, the Transformer MUST:
//   - Preserve Role, Name, ToolCall, and Ctrl fields unchanged
//   - Only modify the Part field (content payload)
type MessageChunk struct {
	Role     Role
	Name     string
	Part     Part
	ToolCall *ToolCall
	Ctrl     *StreamCtrl
}

// StreamCtrl controls Stream routing and state.
//
// Used by input/output modules to:
//   - Route chunks to different streams via StreamID
//   - Signal begin/end of a logical stream via BeginOfStream/EndOfStream
type StreamCtrl struct {
	// StreamID is a sub-stream identifier for mux/demux routing.
	// When set, output modules can route chunks to different destinations
	// based on this identifier combined with Role and MIME type.
	StreamID string `json:"stream_id,omitempty"`

	// Label is an optional human-readable tag for debugging.
	// Not used for routing or logic, only for logging/tracing.
	Label string `json:"label,omitempty"`

	// BeginOfStream marks the start of a logical stream.
	// When a transformer emits a BOS marker, downstream can initialize state
	// and potentially interrupt previous streams with different StreamIDs.
	BeginOfStream bool `json:"begin_of_stream,omitempty"`

	// EndOfStream marks the end of a logical stream.
	// When a transformer receives an EOS marker matching its input type,
	// it should finish current processing and emit an EOS with its output type.
	// The Part field should have the same MIME type as other outputs from the transformer.
	EndOfStream bool `json:"end_of_stream,omitempty"`

	// Timestamp is the Unix epoch time in milliseconds when this chunk was created.
	// Used for packet loss detection and timing synchronization in real-time streams.
	// When set, receivers can detect gaps in the stream by comparing timestamps.
	Timestamp int64 `json:"timestamp,omitempty"`
}

// IsBeginOfStream returns true if this chunk is a begin-of-stream marker.
func (c *MessageChunk) IsBeginOfStream() bool {
	return c != nil && c.Ctrl != nil && c.Ctrl.BeginOfStream
}

// IsEndOfStream returns true if this chunk is an end-of-stream boundary marker.
func (c *MessageChunk) IsEndOfStream() bool {
	return c != nil && c.Ctrl != nil && c.Ctrl.EndOfStream
}

// NewBeginOfStream creates a BOS marker with the given StreamID.
// This is used by transformers to signal the start of a new logical stream.
func NewBeginOfStream(streamID string) *MessageChunk {
	return &MessageChunk{
		Ctrl: &StreamCtrl{StreamID: streamID, BeginOfStream: true},
	}
}

// NewEndOfStream creates an EOS marker with the given MIME type.
// This is used by transformers to emit EOS markers with their output MIME type.
func NewEndOfStream(mimeType string) *MessageChunk {
	return &MessageChunk{
		Part: &Blob{MIMEType: mimeType, Data: nil},
		Ctrl: &StreamCtrl{EndOfStream: true},
	}
}

// NewTextEndOfStream creates a text EoS marker.
// This is used by ASR transformers to emit EoS after text output.
func NewTextEndOfStream() *MessageChunk {
	return &MessageChunk{
		Part: Text(""),
		Ctrl: &StreamCtrl{EndOfStream: true},
	}
}

// Clone returns a deep copy of the MessageChunk.
func (c *MessageChunk) Clone() *MessageChunk {
	chk := &MessageChunk{
		Role: c.Role,
		Name: c.Name,
	}
	if c.Part != nil {
		chk.Part = c.Part.clone()
	}
	if c.ToolCall != nil {
		t := *c.ToolCall
		chk.ToolCall = &t
	}
	if c.Ctrl != nil {
		ctrl := *c.Ctrl
		chk.Ctrl = &ctrl
	}
	return chk
}

type Message struct {
	Role    Role
	Name    string
	Payload Payload
}

// Role identifies the producer of a message.
// See RoleUser, RoleModel, RoleTool constants.
type Role string

func (r Role) String() string {
	return string(r)
}

type Payload interface {
	isPayload()
}

type FuncCall struct {
	Name      string
	Arguments string

	tool *FuncTool
}

func (f *FuncCall) Invoke(ctx context.Context) (any, error) {
	if f.tool == nil {
		return nil, fmt.Errorf("tool not found: name=%s", f.Name)
	}
	if f.tool.Invoke == nil {
		return nil, fmt.Errorf("invoke function not set: name=%s", f.Name)
	}
	return f.tool.Invoke(ctx, f, f.Arguments)
}

type ToolCall struct {
	ID       string
	FuncCall *FuncCall
}

func (*ToolCall) isPayload() {}

func (tool *ToolCall) Invoke(ctx context.Context) (any, error) {
	if tool.FuncCall == nil {
		return nil, fmt.Errorf("invoke can only be called on function call: id=%s", tool.ID)
	}
	return tool.FuncCall.Invoke(ctx)
}

type ToolResult struct {
	ID     string
	Result string
}

func (*ToolResult) isPayload() {}

type Contents []Part

func (Contents) isPayload() {}

// Part is the content payload of a MessageChunk.
// Implementations: Text (string content) and Blob (binary data with MIME type).
type Part interface {
	isPart()
	clone() Part
}

// Blob represents binary data with a MIME type.
// Common MIME types:
//   - audio/opus, audio/pcm, audio/mp3: Audio data
//   - image/png, image/jpeg: Image data
//   - text/plain: Plain text (prefer using Text type instead)
type Blob struct {
	MIMEType string
	Data     []byte
}

func (b *Blob) clone() Part {
	return &Blob{
		MIMEType: b.MIMEType,
		Data:     slices.Clone(b.Data),
	}
}

func (*Blob) isPart() {}

// Text represents string content in a MessageChunk.
// This is the most common Part type for LLM conversations.
type Text string

func (t Text) clone() Part {
	return t
}

func (Text) isPart() {}
