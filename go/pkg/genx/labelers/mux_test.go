package labelers

import (
	"context"
	"testing"
)

type stubLabeler struct {
	model  string
	result *Result
	err    error
}

func (s *stubLabeler) Process(_ context.Context, _ Input) (*Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *stubLabeler) Model() string { return s.model }

func TestMuxHandleAndGet(t *testing.T) {
	mux := NewMux()
	labeler := &stubLabeler{model: "mock"}

	if err := mux.Handle("labeler/qwen-flash", labeler); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got, err := mux.Get("labeler/qwen-flash")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Model() != "mock" {
		t.Fatalf("Model() = %q, want %q", got.Model(), "mock")
	}
}

func TestMuxDuplicateHandle(t *testing.T) {
	mux := NewMux()
	labeler := &stubLabeler{model: "mock"}
	if err := mux.Handle("labeler/qwen-flash", labeler); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := mux.Handle("labeler/qwen-flash", labeler); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestMuxGetNotFound(t *testing.T) {
	mux := NewMux()
	if _, err := mux.Get("labeler/not-found"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestMuxHandleEmptyPattern(t *testing.T) {
	mux := NewMux()
	if err := mux.Handle("", &stubLabeler{model: "mock"}); err == nil {
		t.Fatal("expected empty pattern error")
	}
}

func TestMuxHandleNilLabeler(t *testing.T) {
	mux := NewMux()
	if err := mux.Handle("labeler/x", nil); err == nil {
		t.Fatal("expected nil labeler error")
	}
}
