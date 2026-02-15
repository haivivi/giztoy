package segmentors

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/trie"
)

// DefaultMux is the default segmentor multiplexer.
var DefaultMux = NewMux()

// Handle registers a segmentor for the given pattern to the default mux.
func Handle(pattern string, s Segmentor) error {
	return DefaultMux.Handle(pattern, s)
}

// Get returns the segmentor registered for the given pattern from the default mux.
func Get(pattern string) (Segmentor, error) {
	return DefaultMux.Get(pattern)
}

// Process runs the segmentor registered for the given pattern in the default mux.
func Process(ctx context.Context, pattern string, input Input) (*Result, error) {
	return DefaultMux.Process(ctx, pattern, input)
}

// Mux is a segmentor multiplexer that routes processing requests to registered
// [Segmentor] implementations based on pattern matching using a trie.
type Mux struct {
	mux *trie.Trie[Segmentor]
}

// NewMux creates a new segmentor multiplexer.
func NewMux() *Mux {
	return &Mux{
		mux: trie.New[Segmentor](),
	}
}

// Handle registers a segmentor for the given pattern.
// Returns an error if a segmentor is already registered for the pattern.
func (m *Mux) Handle(pattern string, s Segmentor) error {
	return m.mux.Set(pattern, func(ptr *Segmentor, existed bool) error {
		if existed {
			return fmt.Errorf("segmentors: segmentor already registered for %s", pattern)
		}
		*ptr = s
		return nil
	})
}

// Get returns the segmentor registered for the given pattern.
func (m *Mux) Get(pattern string) (Segmentor, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("segmentors: segmentor not found for %s", pattern)
	}
	s := *ptr
	if s == nil {
		return nil, fmt.Errorf("segmentors: segmentor not found for %s", pattern)
	}
	return s, nil
}

// Process runs the segmentor registered for the given pattern.
func (m *Mux) Process(ctx context.Context, pattern string, input Input) (*Result, error) {
	s, err := m.Get(pattern)
	if err != nil {
		return nil, err
	}
	return s.Process(ctx, input)
}
