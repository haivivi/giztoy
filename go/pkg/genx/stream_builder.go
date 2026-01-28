package genx

import (
	"fmt"
	"log/slog"

	"github.com/haivivi/giztoy/go/pkg/buffer"
)

type Status int

const (
	StatusOK Status = iota
	StatusDone
	StatusTruncated
	StatusBlocked
	StatusError
)

type StreamEvent struct {
	Chunk   *MessageChunk
	Status  Status
	Usage   Usage
	Refusal string
	Error   error
}

type StreamBuilder struct {
	rb        *buffer.BlockBuffer[*StreamEvent]
	funcTools map[string]*FuncTool
}

func NewStreamBuilder(mctx ModelContext, size int) *StreamBuilder {
	sb := &StreamBuilder{
		rb:        buffer.BlockN[*StreamEvent](size),
		funcTools: make(map[string]*FuncTool),
	}
	for tool := range mctx.Tools() {
		switch t := tool.(type) {
		case *FuncTool:
			sb.funcTools[t.Name] = t
		}
	}
	return sb
}

func (sb *StreamBuilder) Done(stats Usage) error {
	if err := sb.rb.Add(&StreamEvent{
		Status: StatusDone,
		Usage:  stats,
	}); err != nil {
		return err
	}
	return sb.rb.CloseWrite()
}

func (sb *StreamBuilder) Truncated(stats Usage) error {
	if err := sb.rb.Add(&StreamEvent{
		Status: StatusTruncated,
		Usage:  stats,
	}); err != nil {
		return err
	}
	return sb.rb.CloseWrite()
}

func (sb *StreamBuilder) Blocked(stats Usage, refusal string) error {
	if err := sb.rb.Add(&StreamEvent{
		Status:  StatusBlocked,
		Usage:   stats,
		Refusal: refusal,
	}); err != nil {
		return err
	}
	return sb.rb.CloseWrite()
}

func (sb *StreamBuilder) Unexpected(stats Usage, err error) error {
	if err := sb.rb.Add(&StreamEvent{
		Status: StatusError,
		Usage:  stats,
		Error:  err,
	}); err != nil {
		return err
	}
	return sb.rb.CloseWrite()
}

func (sb *StreamBuilder) Add(evt ...*MessageChunk) error {
	for _, e := range evt {
		if e.ToolCall != nil && e.ToolCall.FuncCall != nil {
			t, ok := sb.funcTools[e.ToolCall.FuncCall.Name]
			if !ok {
				slog.Warn("genx/stream_builder: tool call not found", "name", e.ToolCall.FuncCall.Name)
				continue
			}
			e.ToolCall.FuncCall.tool = t
		}
		if err := sb.rb.Add(&StreamEvent{Chunk: e}); err != nil {
			return err
		}
	}
	return nil
}

func (sb *StreamBuilder) Abort(err error) error {
	return sb.rb.CloseWithError(err)
}

func (sb *StreamBuilder) Stream() Stream {
	return (*streamImpl)(sb)
}

type streamImpl StreamBuilder

func (s *streamImpl) Next() (*MessageChunk, error) {
	evt, err := s.rb.Next()
	if err != nil {
		return nil, err
	}
	switch evt.Status {
	case StatusOK:
		return evt.Chunk, nil
	case StatusDone:
		err = Done(evt.Usage)
	case StatusTruncated:
		err = Truncated(evt.Usage)
	case StatusBlocked:
		err = Blocked(evt.Usage, evt.Refusal)
	case StatusError:
		err = Error(evt.Usage, evt.Error)
	default:
		err = fmt.Errorf("unexpected stream status: %v", evt.Status)
	}
	s.rb.CloseWithError(err)
	return nil, err
}

func (s *streamImpl) Close() error {
	return s.rb.Close()
}

func (s *streamImpl) CloseWithError(err error) error {
	return s.rb.CloseWithError(err)
}
