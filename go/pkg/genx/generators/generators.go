// Package generators provides a multiplexer for genx.Generator routing.
package generators

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/trie"
)

var _ genx.Generator = (*Mux)(nil)

// DefaultMux is the default generator multiplexer.
var DefaultMux = NewMux()

// Handle registers a generator for the given pattern to the default mux.
func Handle(pattern string, gen genx.Generator) error {
	return DefaultMux.Handle(pattern, gen)
}

// GenerateStream generates a stream using the default mux.
func GenerateStream(ctx context.Context, pattern string, mctx genx.ModelContext) (genx.Stream, error) {
	return DefaultMux.GenerateStream(ctx, pattern, mctx)
}

// Invoke invokes a function tool using the default mux.
func Invoke(ctx context.Context, pattern string, mctx genx.ModelContext, fn *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return DefaultMux.Invoke(ctx, pattern, mctx, fn)
}

// Mux is a generator multiplexer that routes requests to registered generators
// based on pattern matching using a trie.
type Mux struct {
	mux *trie.Trie[genx.Generator]
}

// NewMux creates a new generator multiplexer.
func NewMux() *Mux {
	return &Mux{
		mux: &trie.Trie[genx.Generator]{},
	}
}

// Handle registers a generator for the given pattern.
// Returns an error if a generator is already registered for the pattern.
func (gm *Mux) Handle(name string, gen genx.Generator) error {
	return gm.mux.Set(name, func(ptr *genx.Generator, existed bool) error {
		if existed {
			return fmt.Errorf("generator already registered for %s", name)
		}
		*ptr = gen
		return nil
	})
}

// GenerateStream generates a stream by looking up the generator for the given pattern.
func (gm *Mux) GenerateStream(ctx context.Context, name string, mctx genx.ModelContext) (genx.Stream, error) {
	gen, err := gm.get(name)
	if err != nil {
		return nil, err
	}
	return gen.GenerateStream(ctx, name, mctx)
}

// Invoke invokes a function tool by looking up the generator for the given pattern.
func (gm *Mux) Invoke(ctx context.Context, name string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	gen, err := gm.get(name)
	if err != nil {
		return genx.Usage{}, nil, err
	}
	return gen.Invoke(ctx, name, mctx, tool)
}

func (gm *Mux) get(pattern string) (genx.Generator, error) {
	ptr, ok := gm.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("generator not found for %s", pattern)
	}
	gen := *ptr
	if gen == nil {
		return nil, fmt.Errorf("generator not found for %s", pattern)
	}
	return gen, nil
}
