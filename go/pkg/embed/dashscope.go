package embed

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// DashScope embedding models.
const (
	// ModelDashScopeV4 is the latest DashScope embedding model.
	// Supports 100+ languages, dimensions: 64–2048, default 1024.
	ModelDashScopeV4 = "text-embedding-v4"

	// ModelDashScopeV3 supports 50+ languages, dimensions: 64–1024.
	ModelDashScopeV3 = "text-embedding-v3"

	// ModelDashScopeV2 has fixed 1536 dimensions.
	ModelDashScopeV2 = "text-embedding-v2"

	// ModelDashScopeV1 has fixed 1536 dimensions.
	ModelDashScopeV1 = "text-embedding-v1"
)

const (
	dashScopeBaseURL      = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	dashScopeMaxBatch     = 10 // v3/v4 max batch size
	dashScopeDefaultDim   = 1024
	dashScopeDefaultModel = ModelDashScopeV4
)

// DashScope implements [Embedder] using Aliyun DashScope's OpenAI-compatible
// embedding API.
type DashScope struct {
	client *openai.Client
	model  string
	dim    int
}

var _ Embedder = (*DashScope)(nil)

// NewDashScope creates a DashScope embedder.
//
// The apiKey is required and can be obtained from:
// https://bailian.console.aliyun.com/?apiKey=1
func NewDashScope(apiKey string, opts ...Option) *DashScope {
	cfg := config{
		model:      dashScopeDefaultModel,
		dim:        dashScopeDefaultDim,
		baseURL:    dashScopeBaseURL,
		httpClient: http.DefaultClient,
	}
	for _, o := range opts {
		o(&cfg)
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(cfg.baseURL),
		option.WithHTTPClient(cfg.httpClient),
	)

	return &DashScope{
		client: &client,
		model:  cfg.model,
		dim:    cfg.dim,
	}
}

// Embed returns the embedding for a single text.
func (d *DashScope) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, ErrEmptyInput
	}
	vecs, err := d.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedBatch returns embeddings for multiple texts.
// Batches larger than 10 are automatically split into multiple API calls.
func (d *DashScope) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrEmptyInput
	}

	result := make([][]float32, len(texts))
	for i := 0; i < len(texts); i += dashScopeMaxBatch {
		end := min(i+dashScopeMaxBatch, len(texts))
		batch := texts[i:end]

		vecs, err := d.callAPI(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		copy(result[i:], vecs)
	}
	return result, nil
}

// Dimension returns the configured vector dimensionality.
func (d *DashScope) Dimension() int {
	return d.dim
}

// Model returns the DashScope model identifier (e.g., "text-embedding-v4").
func (d *DashScope) Model() string {
	return d.model
}

func (d *DashScope) callAPI(ctx context.Context, texts []string) ([][]float32, error) {
	params := openai.EmbeddingNewParams{
		Model:          d.model,
		Input:          openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
		Dimensions:     openai.Int(int64(d.dim)),
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	}

	resp, err := d.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, err
	}

	vecs := make([][]float32, len(texts))
	for _, item := range resp.Data {
		idx := item.Index
		if idx < 0 || idx >= int64(len(texts)) {
			return nil, fmt.Errorf("unexpected embedding index %d for batch size %d", idx, len(texts))
		}
		vecs[idx] = float64sToFloat32s(item.Embedding)
	}

	// Verify all slots are filled.
	for i, v := range vecs {
		if v == nil {
			return nil, fmt.Errorf("missing embedding for index %d", i)
		}
	}
	return vecs, nil
}

// float64sToFloat32s converts a []float64 to []float32.
func float64sToFloat32s(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}
