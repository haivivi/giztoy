package embed

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/trie"
)

// DefaultMux is the default embedder multiplexer.
var DefaultMux = NewMux()

// Handle registers an embedder for the given pattern to the default mux.
func Handle(pattern string, e Embedder) error {
	return DefaultMux.Handle(pattern, e)
}

// Get returns the embedder registered for the given pattern from the default mux.
func Get(pattern string) (Embedder, error) {
	return DefaultMux.Get(pattern)
}

// Embed embeds a single text using the embedder registered for the given pattern
// in the default mux.
func Embed(ctx context.Context, pattern string, text string) ([]float32, error) {
	return DefaultMux.Embed(ctx, pattern, text)
}

// EmbedBatch embeds multiple texts using the embedder registered for the given
// pattern in the default mux.
func EmbedBatch(ctx context.Context, pattern string, texts []string) ([][]float32, error) {
	return DefaultMux.EmbedBatch(ctx, pattern, texts)
}

// Dimension returns the vector dimensionality of the embedder registered for
// the given pattern in the default mux.
func Dimension(pattern string) (int, error) {
	return DefaultMux.Dimension(pattern)
}

// Mux is an embedder multiplexer that routes embedding requests to registered
// [Embedder] implementations based on pattern matching using a trie.
//
// Patterns follow the trie path convention with "/" separators:
//
//	embed.Handle("dashscope/v4", embed.NewDashScope(key))
//	embed.Handle("openai/3-small", embed.NewOpenAI(key))
//
// Wildcards are supported:
//
//	embed.Handle("dashscope/+", defaultDashScope) // matches dashscope/<anything>
type Mux struct {
	mux *trie.Trie[Embedder]
}

// NewMux creates a new embedder multiplexer.
func NewMux() *Mux {
	return &Mux{
		mux: trie.New[Embedder](),
	}
}

// Handle registers an embedder for the given pattern.
// Returns an error if an embedder is already registered for the pattern.
func (m *Mux) Handle(pattern string, e Embedder) error {
	return m.mux.Set(pattern, func(ptr *Embedder, existed bool) error {
		if existed {
			return fmt.Errorf("embed: embedder already registered for %s", pattern)
		}
		*ptr = e
		return nil
	})
}

// Get returns the embedder registered for the given pattern.
func (m *Mux) Get(pattern string) (Embedder, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("embed: embedder not found for %s", pattern)
	}
	e := *ptr
	if e == nil {
		return nil, fmt.Errorf("embed: embedder not found for %s", pattern)
	}
	return e, nil
}

// Embed embeds a single text using the embedder registered for the given pattern.
func (m *Mux) Embed(ctx context.Context, pattern string, text string) ([]float32, error) {
	e, err := m.Get(pattern)
	if err != nil {
		return nil, err
	}
	return e.Embed(ctx, text)
}

// EmbedBatch embeds multiple texts using the embedder registered for the given pattern.
func (m *Mux) EmbedBatch(ctx context.Context, pattern string, texts []string) ([][]float32, error) {
	e, err := m.Get(pattern)
	if err != nil {
		return nil, err
	}
	return e.EmbedBatch(ctx, texts)
}

// Dimension returns the vector dimensionality of the embedder registered for
// the given pattern.
func (m *Mux) Dimension(pattern string) (int, error) {
	e, err := m.Get(pattern)
	if err != nil {
		return 0, err
	}
	return e.Dimension(), nil
}
