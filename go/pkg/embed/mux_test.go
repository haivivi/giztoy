package embed_test

import (
	"context"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/embed"
)

func TestMux_Handle_and_Get(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	mux := embed.NewMux()

	ds := embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)
	if err := mux.Handle("dashscope/v4", ds); err != nil {
		t.Fatalf("Handle dashscope/v4: %v", err)
	}

	got, err := mux.Get("dashscope/v4")
	if err != nil {
		t.Fatalf("Get dashscope/v4: %v", err)
	}
	if got != ds {
		t.Fatal("Get returned different embedder instance")
	}
}

func TestMux_Handle_Duplicate(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	mux := embed.NewMux()

	ds := embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)
	if err := mux.Handle("dashscope/v4", ds); err != nil {
		t.Fatalf("Handle first: %v", err)
	}
	if err := mux.Handle("dashscope/v4", ds); err == nil {
		t.Fatal("Handle duplicate: expected error, got nil")
	}
}

func TestMux_Get_NotFound(t *testing.T) {
	mux := embed.NewMux()
	_, err := mux.Get("nonexistent")
	if err == nil {
		t.Fatal("Get nonexistent: expected error, got nil")
	}
}

func TestMux_Embed(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	mux := embed.NewMux()
	if err := mux.Handle("dashscope/v4", embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)); err != nil {
		t.Fatal(err)
	}

	vec, err := mux.Embed(context.Background(), "dashscope/v4", "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != dim {
		t.Fatalf("len(vec) = %d, want %d", len(vec), dim)
	}
}

func TestMux_EmbedBatch(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	mux := embed.NewMux()
	if err := mux.Handle("openai/3-small", embed.NewOpenAI("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)); err != nil {
		t.Fatal(err)
	}

	texts := []string{"a", "b", "c"}
	vecs, err := mux.EmbedBatch(context.Background(), "openai/3-small", texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("len(vecs) = %d, want %d", len(vecs), len(texts))
	}
	for i, v := range vecs {
		if len(v) != dim {
			t.Errorf("vecs[%d]: len = %d, want %d", i, len(v), dim)
		}
	}
}

func TestMux_Dimension(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	mux := embed.NewMux()
	if err := mux.Handle("dashscope/v4", embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)); err != nil {
		t.Fatal(err)
	}

	d, err := mux.Dimension("dashscope/v4")
	if err != nil {
		t.Fatalf("Dimension: %v", err)
	}
	if d != dim {
		t.Fatalf("Dimension = %d, want %d", d, dim)
	}
}

func TestMux_MultipleProviders(t *testing.T) {
	const (
		dsDim = 4
		oaDim = 8
	)
	dsSrv := newFakeServer(t, dsDim)
	defer dsSrv.Close()
	oaSrv := newFakeServer(t, oaDim)
	defer oaSrv.Close()

	mux := embed.NewMux()
	if err := mux.Handle("dashscope/v4", embed.NewDashScope("test-key",
		embed.WithBaseURL(dsSrv.URL),
		embed.WithDimension(dsDim),
	)); err != nil {
		t.Fatal(err)
	}
	if err := mux.Handle("openai/3-small", embed.NewOpenAI("test-key",
		embed.WithBaseURL(oaSrv.URL),
		embed.WithDimension(oaDim),
	)); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// DashScope returns 4-dim vectors.
	vec, err := mux.Embed(ctx, "dashscope/v4", "hello")
	if err != nil {
		t.Fatalf("Embed dashscope: %v", err)
	}
	if len(vec) != dsDim {
		t.Fatalf("dashscope vec len = %d, want %d", len(vec), dsDim)
	}

	// OpenAI returns 8-dim vectors.
	vec, err = mux.Embed(ctx, "openai/3-small", "hello")
	if err != nil {
		t.Fatalf("Embed openai: %v", err)
	}
	if len(vec) != oaDim {
		t.Fatalf("openai vec len = %d, want %d", len(vec), oaDim)
	}

	// Dimensions match.
	d, err := mux.Dimension("dashscope/v4")
	if err != nil {
		t.Fatal(err)
	}
	if d != dsDim {
		t.Fatalf("dashscope dim = %d, want %d", d, dsDim)
	}
	d, err = mux.Dimension("openai/3-small")
	if err != nil {
		t.Fatal(err)
	}
	if d != oaDim {
		t.Fatalf("openai dim = %d, want %d", d, oaDim)
	}
}

func TestMux_Wildcard(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	mux := embed.NewMux()
	ds := embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)
	// Register with single-level wildcard.
	if err := mux.Handle("dashscope/+", ds); err != nil {
		t.Fatal(err)
	}

	// Should match dashscope/v4, dashscope/v3, etc.
	for _, pat := range []string{"dashscope/v4", "dashscope/v3", "dashscope/anything"} {
		got, err := mux.Get(pat)
		if err != nil {
			t.Fatalf("Get(%q): %v", pat, err)
		}
		if got != ds {
			t.Fatalf("Get(%q): wrong instance", pat)
		}
	}

	// Should NOT match dashscope (no sub-segment).
	_, err := mux.Get("dashscope")
	if err == nil {
		t.Fatal("Get(dashscope) without sub-segment: expected error")
	}
}

func TestDefaultMux(t *testing.T) {
	// Verify DefaultMux is usable (non-nil).
	_, err := embed.Get("nonexistent")
	if err == nil {
		t.Fatal("DefaultMux.Get nonexistent: expected error")
	}
}
