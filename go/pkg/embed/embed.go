// Package embed provides a text embedding interface and remote API implementations.
//
// An Embedder converts text into dense vector representations (embeddings)
// suitable for semantic search, clustering, and classification tasks.
//
// # Implementations
//
// Two remote API implementations are provided:
//
//   - [DashScope] — Aliyun DashScope text-embedding-v4 (and v1/v2/v3)
//   - [OpenAI] — OpenAI text-embedding-3-small / text-embedding-3-large
//
// Both use the OpenAI-compatible HTTP API under the hood.
//
// # Quick Start
//
//	e := embed.NewDashScope("sk-xxx", embed.WithModel("text-embedding-v4"))
//	vec, err := e.Embed(ctx, "hello world")
//
//	vecs, err := e.EmbedBatch(ctx, []string{"hello", "world"})
package embed

import (
	"context"
	"errors"
)

// Embedder converts text into dense float32 vectors.
type Embedder interface {
	// Embed returns the embedding vector for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch returns embedding vectors for multiple texts.
	// Implementations may split large batches into smaller API calls
	// transparently.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the dimensionality of the output vectors.
	Dimension() int
}

// Common errors.
var (
	// ErrEmptyInput is returned when the input text is empty.
	ErrEmptyInput = errors.New("embed: empty input")
)
