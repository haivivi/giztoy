package generators

import (
	"context"
	"errors"
	"testing"

	"github.com/haivivi/giztoy/pkg/genx"
)

// mockGenerator is a simple mock implementation of genx.Generator for testing.
type mockGenerator struct {
	name string
}

func (m *mockGenerator) GenerateStream(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
	return nil, nil
}

func (m *mockGenerator) Invoke(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return genx.Usage{}, &genx.FuncCall{Name: m.name}, nil
}

func TestMux_Handle(t *testing.T) {
	mux := NewMux()

	gen := &mockGenerator{name: "test"}

	// First registration should succeed
	if err := mux.Handle("openai/gpt-4", gen); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Duplicate registration should fail
	if err := mux.Handle("openai/gpt-4", gen); err == nil {
		t.Error("Handle() expected error for duplicate registration")
	}

	// Different pattern should succeed
	if err := mux.Handle("openai/gpt-3.5", gen); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
}

func TestMux_GenerateStream(t *testing.T) {
	mux := NewMux()
	gen := &mockGenerator{name: "test"}

	if err := mux.Handle("openai/gpt-4", gen); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	// Registered pattern should work
	_, err := mux.GenerateStream(ctx, "openai/gpt-4", nil)
	if err != nil {
		t.Errorf("GenerateStream() error = %v", err)
	}

	// Unregistered pattern should fail
	_, err = mux.GenerateStream(ctx, "anthropic/claude", nil)
	if err == nil {
		t.Error("GenerateStream() expected error for unregistered pattern")
	}
}

func TestMux_Invoke(t *testing.T) {
	mux := NewMux()
	gen := &mockGenerator{name: "test-invoke"}

	if err := mux.Handle("openai/gpt-4", gen); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	// Registered pattern should work
	_, call, err := mux.Invoke(ctx, "openai/gpt-4", nil, nil)
	if err != nil {
		t.Errorf("Invoke() error = %v", err)
	}
	if call == nil || call.Name != "test-invoke" {
		t.Error("Invoke() returned unexpected result")
	}

	// Unregistered pattern should fail
	_, _, err = mux.Invoke(ctx, "anthropic/claude", nil, nil)
	if err == nil {
		t.Error("Invoke() expected error for unregistered pattern")
	}
}

func TestMux_WildcardPattern(t *testing.T) {
	mux := NewMux()
	gen := &mockGenerator{name: "openai-all"}

	// Register with wildcard pattern
	if err := mux.Handle("openai/+", gen); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	// Should match any openai model
	_, call, err := mux.Invoke(ctx, "openai/gpt-4", nil, nil)
	if err != nil {
		t.Errorf("Invoke() error = %v", err)
	}
	if call == nil || call.Name != "openai-all" {
		t.Error("Invoke() did not match wildcard pattern")
	}

	_, call, err = mux.Invoke(ctx, "openai/gpt-3.5-turbo", nil, nil)
	if err != nil {
		t.Errorf("Invoke() error = %v", err)
	}
	if call == nil || call.Name != "openai-all" {
		t.Error("Invoke() did not match wildcard pattern")
	}
}

func TestDefaultMux(t *testing.T) {
	// Reset DefaultMux for testing
	DefaultMux = NewMux()

	gen := &mockGenerator{name: "default-test"}

	if err := Handle("test/model", gen); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	_, err := GenerateStream(ctx, "test/model", nil)
	if err != nil {
		t.Errorf("GenerateStream() error = %v", err)
	}

	_, call, err := Invoke(ctx, "test/model", nil, nil)
	if err != nil {
		t.Errorf("Invoke() error = %v", err)
	}
	if call == nil || call.Name != "default-test" {
		t.Error("Invoke() returned unexpected result")
	}
}

// errorGenerator always returns an error.
type errorGenerator struct{}

func (e *errorGenerator) GenerateStream(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
	return nil, errors.New("stream error")
}

func (e *errorGenerator) Invoke(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return genx.Usage{}, nil, errors.New("invoke error")
}

func TestMux_GeneratorError(t *testing.T) {
	mux := NewMux()
	gen := &errorGenerator{}

	if err := mux.Handle("error/model", gen); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ctx := context.Background()

	_, err := mux.GenerateStream(ctx, "error/model", nil)
	if err == nil || err.Error() != "stream error" {
		t.Errorf("GenerateStream() expected 'stream error', got %v", err)
	}

	_, _, err = mux.Invoke(ctx, "error/model", nil, nil)
	if err == nil || err.Error() != "invoke error" {
		t.Errorf("Invoke() expected 'invoke error', got %v", err)
	}
}
