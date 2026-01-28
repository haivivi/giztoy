// Package modelcontexts provides a multiplexer for ModelContextProvider routing.
package modelcontexts

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/trie"
)

var _ ModelContextProvider = (*Mux)(nil)

// DefaultMux is the default model context provider multiplexer.
var DefaultMux = NewMux()

// Handle registers a provider for the given pattern to the default mux.
func Handle(pattern string, provider ModelContextProvider) error {
	return DefaultMux.Handle(pattern, provider)
}

// HandleFunc registers a provider function for the given pattern to the default mux.
func HandleFunc(pattern string, f ModelContextProviderFunc) error {
	return DefaultMux.HandleFunc(pattern, f)
}

// ModelContext returns a model context using the default mux.
func ModelContext(ctx context.Context, pattern string) (genx.ModelContext, error) {
	return DefaultMux.ModelContext(ctx, pattern)
}

// ModelContextProviderFunc is a function type that implements ModelContextProvider.
type ModelContextProviderFunc func(context.Context, string) (genx.ModelContext, error)

// ModelContext implements ModelContextProvider.
func (f ModelContextProviderFunc) ModelContext(ctx context.Context, name string) (genx.ModelContext, error) {
	return f(ctx, name)
}

// ModelContextProvider provides model contexts for generation.
type ModelContextProvider interface {
	ModelContext(context.Context, string) (genx.ModelContext, error)
}

// Mux is a model context provider multiplexer that routes requests to registered
// providers based on pattern matching using a trie.
type Mux struct {
	mux *trie.Trie[ModelContextProvider]
}

// NewMux creates a new model context provider multiplexer.
func NewMux() *Mux {
	return &Mux{
		mux: &trie.Trie[ModelContextProvider]{},
	}
}

// Handle registers a provider for the given pattern.
func (mcp *Mux) Handle(name string, provider ModelContextProvider) error {
	return mcp.mux.Set(name, func(ptr *ModelContextProvider, existed bool) error {
		*ptr = provider
		return nil
	})
}

// HandleFunc registers a provider function for the given pattern.
func (mcp *Mux) HandleFunc(name string, f ModelContextProviderFunc) error {
	return mcp.Handle(name, f)
}

// ModelContext returns a model context by looking up the provider for the given pattern.
func (mcp *Mux) ModelContext(ctx context.Context, name string) (genx.ModelContext, error) {
	ptr, ok := mcp.mux.Get(name)
	if !ok {
		return nil, fmt.Errorf("model context provider not found for %s", name)
	}
	provider := *ptr
	if provider == nil {
		return nil, fmt.Errorf("model context provider not found for %s", name)
	}
	return provider.ModelContext(ctx, name)
}
