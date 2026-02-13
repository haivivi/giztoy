package embed

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAI embedding models.
const (
	// ModelOpenAI3Small is the small embedding model (1536 dims, customizable).
	ModelOpenAI3Small = "text-embedding-3-small"

	// ModelOpenAI3Large is the large embedding model (3072 dims, customizable).
	ModelOpenAI3Large = "text-embedding-3-large"

	// ModelOpenAIAda002 is the legacy model (1536 dims, fixed).
	ModelOpenAIAda002 = "text-embedding-ada-002"
)

const (
	openAIMaxBatch     = 2048 // OpenAI supports up to 2048 inputs per request
	openAIDefaultDim   = 1536
	openAIDefaultModel = ModelOpenAI3Small
)

// OpenAI implements [Embedder] using the OpenAI embeddings API.
//
// This can also be used with any OpenAI-compatible provider (e.g. SiliconFlow)
// by setting WithBaseURL.
type OpenAI struct {
	client *openai.Client
	model  string
	dim    int
}

var _ Embedder = (*OpenAI)(nil)

// NewOpenAI creates an OpenAI embedder.
//
// The apiKey is required and can be obtained from:
// https://platform.openai.com/api-keys
func NewOpenAI(apiKey string, opts ...Option) *OpenAI {
	cfg := config{
		model:      openAIDefaultModel,
		dim:        openAIDefaultDim,
		httpClient: http.DefaultClient,
	}
	for _, o := range opts {
		o(&cfg)
	}

	clientOpts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(cfg.httpClient),
	}
	if cfg.baseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(cfg.baseURL))
	}
	client := openai.NewClient(clientOpts...)

	return &OpenAI{
		client: &client,
		model:  cfg.model,
		dim:    cfg.dim,
	}
}

// Embed returns the embedding for a single text.
func (o *OpenAI) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, ErrEmptyInput
	}
	vecs, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedBatch returns embeddings for multiple texts.
// Batches larger than 2048 are automatically split into multiple API calls.
func (o *OpenAI) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrEmptyInput
	}

	result := make([][]float32, len(texts))
	for i := 0; i < len(texts); i += openAIMaxBatch {
		end := min(i+openAIMaxBatch, len(texts))
		batch := texts[i:end]

		vecs, err := o.callAPI(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		copy(result[i:], vecs)
	}
	return result, nil
}

// Dimension returns the configured vector dimensionality.
func (o *OpenAI) Dimension() int {
	return o.dim
}

// Model returns the OpenAI model identifier (e.g., "text-embedding-3-small").
func (o *OpenAI) Model() string {
	return o.model
}

func (o *OpenAI) callAPI(ctx context.Context, texts []string) ([][]float32, error) {
	params := openai.EmbeddingNewParams{
		Model:          o.model,
		Input:          openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
		Dimensions:     openai.Int(int64(o.dim)),
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	}

	resp, err := o.client.Embeddings.New(ctx, params)
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
