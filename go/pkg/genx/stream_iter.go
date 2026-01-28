package genx

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/buffer"
)

type IterElement interface {
	isStreamElement()
}

type ToolCallElement struct {
	Role     Role
	Name     string
	ToolCall ToolCall
}

func (*ToolCallElement) isStreamElement() {}

type StreamElement struct {
	Name     string
	Role     Role
	MIMEType string
	reader   io.Reader
}

func (ms *StreamElement) Read(b []byte) (int, error) {
	return ms.reader.Read(b)
}

func (*StreamElement) isStreamElement() {}

type iterStreamKey struct {
	Name     string
	Role     Role
	MIMEType string
}

type StreamIter struct {
	mu      sync.Mutex
	streams map[iterStreamKey]buffer.BytesBuffer
	ch      chan IterElement
	err     error
}

func Iter(str Stream) *StreamIter {
	iter := &StreamIter{
		streams: make(map[iterStreamKey]buffer.BytesBuffer),
		ch:      make(chan IterElement, 2),
	}
	go iter.pull(str)
	return iter
}

func (itr *StreamIter) Next() (IterElement, error) {
	el, ok := <-itr.ch
	if !ok {
		itr.mu.Lock()
		defer itr.mu.Unlock()
		if itr.err != nil {
			return nil, itr.err
		}
		return nil, ErrDone
	}
	return el, nil
}

func (itr *StreamIter) Where(test func(IterElement) bool) iter.Seq2[IterElement, error] {
	return func(yield func(IterElement, error) bool) {
		for {
			el, err := itr.Next()
			if err != nil {
				if errors.Is(err, ErrDone) {
					return
				}
				yield(nil, err)
				return
			}
			if test(el) {
				if !yield(el, nil) {
					return
				}
			}
		}
	}
}

func (itr *StreamIter) FirstWhere(test func(IterElement) bool) (IterElement, error) {
	for {
		el, err := itr.Next()
		if err != nil {
			if errors.Is(err, ErrDone) {
				return nil, nil
			}
			return nil, err
		}
		if test(el) {
			go itr.discard()
			return el, nil
		}
		if s, ok := el.(*StreamElement); ok {
			go io.Copy(io.Discard, s)
		}
	}
}

func (itr *StreamIter) WriteTo(mimeType string, w io.Writer) (int64, error) {
	el, err := itr.FirstWhere(func(el IterElement) bool {
		if s, ok := el.(*StreamElement); ok {
			return s.MIMEType == mimeType
		}
		return false
	})
	if err != nil {
		return 0, err
	}
	if el == nil {
		return 0, nil
	}
	return io.Copy(w, el.(*StreamElement))
}

func (itr *StreamIter) discard() {
	for {
		el, err := itr.Next()
		if err != nil {
			return
		}

		if s, ok := el.(*StreamElement); ok {
			go io.Copy(io.Discard, s)
		}
	}
}

func (itr *StreamIter) pull(str Stream) (re error) {
	defer func() {
		itr.mu.Lock()
		defer itr.mu.Unlock()
		for _, stream := range itr.streams {
			if re == nil {
				stream.CloseWrite()
			} else {
				stream.CloseWithError(re)
			}
		}
		itr.streams = nil
		itr.err = re
		close(itr.ch)
		for range itr.ch {
			// discard remaining elements
		}
	}()
	for {
		chunk, err := str.Next()
		if err != nil {
			if errors.Is(err, ErrDone) {
				return nil
			}
			return err
		}
		switch {
		case chunk.ToolCall != nil:
			select {
			case itr.ch <- &ToolCallElement{
				Role:     chunk.Role,
				Name:     chunk.Name,
				ToolCall: *chunk.ToolCall,
			}:
			default:
			}
		case chunk.Part != nil:
			key := iterStreamKey{
				Name: chunk.Name,
				Role: chunk.Role,
			}
			var (
				data []byte
				cap  int
			)
			switch v := chunk.Part.(type) {
			case *Blob:
				key.MIMEType = v.MIMEType
				data = v.Data
				cap = 16 * 1024
			case Text:
				key.MIMEType = "text/plain"
				data = []byte(v)
				cap = 1024
			default:
				return fmt.Errorf("invalid part type: %T", v)
			}
			rb, ok := itr.streams[key]
			if !ok {
				rb = buffer.BlockN[byte](cap)
				itr.streams[key] = rb
				select {
				case itr.ch <- &StreamElement{
					Name:     key.Name,
					Role:     key.Role,
					MIMEType: key.MIMEType,
					reader:   rb,
				}:
				default:
				}
			}
			if _, err := rb.Write(data); err != nil {
				return fmt.Errorf("failed to write to stream: %w", err)
			}
		default:
			return errors.New("invalid message chunk: no tool call or data part")
		}
	}
}
