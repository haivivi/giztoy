package embed_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/embed"
)

// fakeEmbeddingResponse builds a minimal OpenAI-compatible embedding response.
func fakeEmbeddingResponse(dim int, texts []string) []byte {
	type embItem struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	}
	type usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	}
	type resp struct {
		Object string     `json:"object"`
		Model  string     `json:"model"`
		Data   []embItem  `json:"data"`
		Usage  usage      `json:"usage"`
	}

	data := make([]embItem, len(texts))
	for i := range texts {
		vec := make([]float64, dim)
		for j := range vec {
			vec[j] = float64(i+1) * 0.01 * float64(j+1)
		}
		data[i] = embItem{
			Object:    "embedding",
			Index:     i,
			Embedding: vec,
		}
	}

	r := resp{
		Object: "list",
		Model:  "test-model",
		Data:   data,
		Usage:  usage{PromptTokens: 10, TotalTokens: 10},
	}
	b, _ := json.Marshal(r)
	return b
}

// newFakeServer creates a test HTTP server that returns fake embeddings.
func newFakeServer(t *testing.T, dim int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Input interface{} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Determine the number of inputs.
		var texts []string
		switch v := req.Input.(type) {
		case string:
			texts = []string{v}
		case []interface{}:
			for _, item := range v {
				texts = append(texts, fmt.Sprint(item))
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(fakeEmbeddingResponse(dim, texts))
	}))
}

func TestDashScope_Embed(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	e := embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)

	if e.Dimension() != dim {
		t.Fatalf("Dimension() = %d, want %d", e.Dimension(), dim)
	}

	vec, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != dim {
		t.Fatalf("len(vec) = %d, want %d", len(vec), dim)
	}
}

func TestDashScope_EmbedBatch(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	e := embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)

	texts := []string{"hello", "world", "foo"}
	vecs, err := e.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("len(vecs) = %d, want %d", len(vecs), len(texts))
	}
	for i, vec := range vecs {
		if len(vec) != dim {
			t.Errorf("vecs[%d]: len = %d, want %d", i, len(vec), dim)
		}
	}
}

func TestDashScope_EmbedBatch_LargeBatch(t *testing.T) {
	// Verify that batches > 10 are split automatically.
	const dim = 2
	srv := newFakeServer(t, dim)
	defer srv.Close()

	e := embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)

	texts := make([]string, 25)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	vecs, err := e.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 25 {
		t.Fatalf("len(vecs) = %d, want 25", len(vecs))
	}
}

func TestOpenAI_Embed(t *testing.T) {
	const dim = 8
	srv := newFakeServer(t, dim)
	defer srv.Close()

	e := embed.NewOpenAI("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)

	if e.Dimension() != dim {
		t.Fatalf("Dimension() = %d, want %d", e.Dimension(), dim)
	}

	vec, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != dim {
		t.Fatalf("len(vec) = %d, want %d", len(vec), dim)
	}
}

func TestOpenAI_EmbedBatch(t *testing.T) {
	const dim = 8
	srv := newFakeServer(t, dim)
	defer srv.Close()

	e := embed.NewOpenAI("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)

	texts := []string{"a", "b", "c", "d"}
	vecs, err := e.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("len(vecs) = %d, want %d", len(vecs), len(texts))
	}
}

func TestEmbed_EmptyInput(t *testing.T) {
	const dim = 4
	srv := newFakeServer(t, dim)
	defer srv.Close()

	e := embed.NewDashScope("test-key",
		embed.WithBaseURL(srv.URL),
		embed.WithDimension(dim),
	)

	_, err := e.Embed(context.Background(), "")
	if err != embed.ErrEmptyInput {
		t.Fatalf("Embed empty: got %v, want ErrEmptyInput", err)
	}

	_, err = e.EmbedBatch(context.Background(), nil)
	if err != embed.ErrEmptyInput {
		t.Fatalf("EmbedBatch nil: got %v, want ErrEmptyInput", err)
	}

	_, err = e.EmbedBatch(context.Background(), []string{})
	if err != embed.ErrEmptyInput {
		t.Fatalf("EmbedBatch empty: got %v, want ErrEmptyInput", err)
	}
}

func TestEmbedder_Interface(t *testing.T) {
	// Compile-time check that both types implement Embedder.
	var _ embed.Embedder = (*embed.DashScope)(nil)
	var _ embed.Embedder = (*embed.OpenAI)(nil)
}
