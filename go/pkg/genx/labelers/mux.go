package labelers

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/trie"
)

// DefaultMux is the default labeler multiplexer.
var DefaultMux = NewMux()

// Handle registers a labeler on the default mux.
func Handle(pattern string, l Labeler) error {
	return DefaultMux.Handle(pattern, l)
}

// Get finds a labeler from the default mux.
func Get(pattern string) (Labeler, error) {
	return DefaultMux.Get(pattern)
}

// Process runs labeler.Process on the default mux.
func Process(ctx context.Context, pattern string, input Input) (*Result, error) {
	return DefaultMux.Process(ctx, pattern, input)
}

// Mux routes labeler calls by pattern.
type Mux struct {
	mux *trie.Trie[Labeler]
}

// NewMux creates a new labeler mux.
func NewMux() *Mux {
	return &Mux{mux: trie.New[Labeler]()}
}

// Handle registers a labeler for a pattern.
func (m *Mux) Handle(pattern string, l Labeler) error {
	if pattern == "" {
		return fmt.Errorf("labelers: empty pattern")
	}
	if l == nil {
		return fmt.Errorf("labelers: nil labeler for %s", pattern)
	}
	return m.mux.Set(pattern, func(ptr *Labeler, existed bool) error {
		if existed {
			return fmt.Errorf("labelers: labeler already registered for %s", pattern)
		}
		*ptr = l
		return nil
	})
}

// Get returns a labeler by pattern.
func (m *Mux) Get(pattern string) (Labeler, error) {
	if pattern == "" {
		return nil, fmt.Errorf("labelers: empty pattern")
	}
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("labelers: labeler not found for %s", pattern)
	}
	l := *ptr
	if l == nil {
		return nil, fmt.Errorf("labelers: labeler not found for %s", pattern)
	}
	return l, nil
}

// Process runs a labeler by pattern.
func (m *Mux) Process(ctx context.Context, pattern string, input Input) (*Result, error) {
	l, err := m.Get(pattern)
	if err != nil {
		return nil, err
	}
	return l.Process(ctx, input)
}
