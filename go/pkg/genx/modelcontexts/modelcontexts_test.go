package modelcontexts

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/haivivi/giztoy/pkg/genx"
)

// mockModelContext is a simple mock implementation of genx.ModelContext for testing.
type mockModelContext struct {
	name string
}

func (m *mockModelContext) Prompts() iter.Seq[*genx.Prompt]   { return func(yield func(*genx.Prompt) bool) {} }
func (m *mockModelContext) Messages() iter.Seq[*genx.Message] { return func(yield func(*genx.Message) bool) {} }
func (m *mockModelContext) CoTs() iter.Seq[string]            { return func(yield func(string) bool) {} }
func (m *mockModelContext) Tools() iter.Seq[genx.Tool]        { return func(yield func(genx.Tool) bool) {} }
func (m *mockModelContext) Params() *genx.ModelParams         { return nil }

// mockProvider is a simple provider for testing.
type mockProvider struct {
	name string
}

func (p *mockProvider) ModelContext(ctx context.Context, name string) (genx.ModelContext, error) {
	return &mockModelContext{name: p.name}, nil
}

func TestMux_Handle(t *testing.T) {
	mux := NewMux()
	provider := &mockProvider{name: "test"}

	// First registration should succeed
	if err := mux.Handle("openai/gpt-4", provider); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Second registration should overwrite (unlike generators)
	if err := mux.Handle("openai/gpt-4", provider); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Different pattern should succeed
	if err := mux.Handle("anthropic/claude", provider); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
}

func TestMux_HandleFunc(t *testing.T) {
	mux := NewMux()

	fn := ModelContextProviderFunc(func(ctx context.Context, name string) (genx.ModelContext, error) {
		return &mockModelContext{name: name}, nil
	})

	if err := mux.HandleFunc("test/model", fn); err != nil {
		t.Fatalf("HandleFunc() error = %v", err)
	}

	ctx := context.Background()
	mctx, err := mux.ModelContext(ctx, "test/model")
	if err != nil {
		t.Errorf("ModelContext() error = %v", err)
	}
	if mctx == nil {
		t.Error("ModelContext() returned nil")
	}
}

func TestMux_ModelContext(t *testing.T) {
	mux := NewMux()
	provider := &mockProvider{name: "test"}

	if err := mux.Handle("openai/gpt-4", provider); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	// Registered pattern should work
	mctx, err := mux.ModelContext(ctx, "openai/gpt-4")
	if err != nil {
		t.Errorf("ModelContext() error = %v", err)
	}
	if mctx == nil {
		t.Error("ModelContext() returned nil")
	}

	// Unregistered pattern should fail
	_, err = mux.ModelContext(ctx, "anthropic/claude")
	if err == nil {
		t.Error("ModelContext() expected error for unregistered pattern")
	}
}

func TestMux_WildcardPattern(t *testing.T) {
	mux := NewMux()
	provider := &mockProvider{name: "openai-all"}

	// Register with wildcard pattern
	if err := mux.Handle("openai/+", provider); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	// Should match any openai model
	mctx, err := mux.ModelContext(ctx, "openai/gpt-4")
	if err != nil {
		t.Errorf("ModelContext() error = %v", err)
	}
	if mctx == nil {
		t.Error("ModelContext() returned nil for wildcard match")
	}

	mctx, err = mux.ModelContext(ctx, "openai/gpt-3.5-turbo")
	if err != nil {
		t.Errorf("ModelContext() error = %v", err)
	}
	if mctx == nil {
		t.Error("ModelContext() returned nil for wildcard match")
	}
}

func TestDefaultMux(t *testing.T) {
	// Reset DefaultMux for testing
	DefaultMux = NewMux()

	provider := &mockProvider{name: "default-test"}

	if err := Handle("test/model", provider); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	mctx, err := ModelContext(ctx, "test/model")
	if err != nil {
		t.Errorf("ModelContext() error = %v", err)
	}
	if mctx == nil {
		t.Error("ModelContext() returned nil")
	}
}

func TestDefaultMux_HandleFunc(t *testing.T) {
	// Reset DefaultMux for testing
	DefaultMux = NewMux()

	fn := ModelContextProviderFunc(func(ctx context.Context, name string) (genx.ModelContext, error) {
		return &mockModelContext{name: name}, nil
	})

	if err := HandleFunc("func/model", fn); err != nil {
		t.Fatalf("HandleFunc() error = %v", err)
	}

	ctx := context.Background()

	mctx, err := ModelContext(ctx, "func/model")
	if err != nil {
		t.Errorf("ModelContext() error = %v", err)
	}
	if mctx == nil {
		t.Error("ModelContext() returned nil")
	}
}

// errorProvider always returns an error.
type errorProvider struct{}

func (e *errorProvider) ModelContext(ctx context.Context, name string) (genx.ModelContext, error) {
	return nil, errors.New("provider error")
}

func TestMux_ProviderError(t *testing.T) {
	mux := NewMux()
	provider := &errorProvider{}

	if err := mux.Handle("error/model", provider); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	_, err := mux.ModelContext(ctx, "error/model")
	if err == nil || err.Error() != "provider error" {
		t.Errorf("ModelContext() expected 'provider error', got %v", err)
	}
}

func TestModelContextProviderFunc(t *testing.T) {
	fn := ModelContextProviderFunc(func(ctx context.Context, name string) (genx.ModelContext, error) {
		return &mockModelContext{name: name}, nil
	})

	ctx := context.Background()
	mctx, err := fn.ModelContext(ctx, "test")
	if err != nil {
		t.Errorf("ModelContext() error = %v", err)
	}
	if mctx == nil {
		t.Error("ModelContext() returned nil")
	}
}
