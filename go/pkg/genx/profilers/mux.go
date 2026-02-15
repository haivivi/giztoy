package profilers

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/trie"
)

// DefaultMux is the default profiler multiplexer.
var DefaultMux = NewMux()

// Handle registers a profiler for the given pattern to the default mux.
func Handle(pattern string, p Profiler) error {
	return DefaultMux.Handle(pattern, p)
}

// Get returns the profiler registered for the given pattern from the default mux.
func Get(pattern string) (Profiler, error) {
	return DefaultMux.Get(pattern)
}

// Process runs the profiler registered for the given pattern in the default mux.
func Process(ctx context.Context, pattern string, input Input) (*Result, error) {
	return DefaultMux.Process(ctx, pattern, input)
}

// Mux is a profiler multiplexer that routes processing requests to registered
// [Profiler] implementations based on pattern matching using a trie.
type Mux struct {
	mux *trie.Trie[Profiler]
}

// NewMux creates a new profiler multiplexer.
func NewMux() *Mux {
	return &Mux{
		mux: trie.New[Profiler](),
	}
}

// Handle registers a profiler for the given pattern.
// Returns an error if a profiler is already registered for the pattern.
func (m *Mux) Handle(pattern string, p Profiler) error {
	return m.mux.Set(pattern, func(ptr *Profiler, existed bool) error {
		if existed {
			return fmt.Errorf("profilers: profiler already registered for %s", pattern)
		}
		*ptr = p
		return nil
	})
}

// Get returns the profiler registered for the given pattern.
func (m *Mux) Get(pattern string) (Profiler, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("profilers: profiler not found for %s", pattern)
	}
	p := *ptr
	if p == nil {
		return nil, fmt.Errorf("profilers: profiler not found for %s", pattern)
	}
	return p, nil
}

// Process runs the profiler registered for the given pattern.
func (m *Mux) Process(ctx context.Context, pattern string, input Input) (*Result, error) {
	p, err := m.Get(pattern)
	if err != nil {
		return nil, err
	}
	return p.Process(ctx, input)
}
