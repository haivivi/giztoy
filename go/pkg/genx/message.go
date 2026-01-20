package genx

import (
	"context"
	"fmt"
	"slices"
)

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
	RoleTool  Role = "tool"
)

var (
	_ Payload = (*Contents)(nil)
	_ Payload = (*ToolCall)(nil)
	_ Payload = (*ToolResult)(nil)

	_ Part = (*Blob)(nil)
	_ Part = (*Text)(nil)
)

type MessageChunk struct {
	Role     Role
	Name     string
	Part     Part
	ToolCall *ToolCall
}

func (c *MessageChunk) Clone() *MessageChunk {
	chk := &MessageChunk{
		Role: c.Role,
		Name: c.Name,
		Part: c.Part.clone(),
	}
	if chk.ToolCall != nil {
		t := *chk.ToolCall
		chk.ToolCall = &t
	}
	return chk
}

type Message struct {
	Role    Role
	Name    string
	Payload Payload
}

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

type Part interface {
	isPart()
	clone() Part
}

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

type Text string

func (t Text) clone() Part {
	return t
}

func (Text) isPart() {}
