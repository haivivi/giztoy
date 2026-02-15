package segmentors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"
)

var _ Segmentor = (*GenX)(nil)

// extractArg is the typed argument for the FuncTool, matching the expected
// JSON output from the LLM.
//
// NOTE: Attrs uses []extractAttr (key-value pairs) instead of map[string]any
// because OpenAI strict mode requires additionalProperties:false on all objects,
// which is incompatible with dynamic-key maps. The pairs are converted to
// map[string]any in parseResult.
type extractArg struct {
	Segment   extractSegment   `json:"segment"`
	Entities  []extractEntity  `json:"entities"`
	Relations []RelationOutput `json:"relations"`
}

type extractSegment struct {
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
	Labels   []string `json:"labels"`
}

type extractEntity struct {
	Label string        `json:"label"`
	Attrs []extractAttr `json:"attrs"`
}

type extractAttr struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// extractTool is the FuncTool that defines the JSON schema for the LLM output.
var extractTool = genx.MustNewFuncTool[extractArg](
	"extract",
	"Extract a compressed segment with entities and relations from the conversation.",
)

// GenX implements [Segmentor] using a genx.Generator (LLM) for extraction.
type GenX struct {
	generator string // pattern for generators.DefaultMux
	mux       *generators.Mux
}

// NewGenX creates a new GenX segmentor.
//
// cfg.Generator must be a pattern registered in generators.DefaultMux (e.g., "qwen/turbo").
func NewGenX(cfg Config) *GenX {
	return &GenX{
		generator: cfg.Generator,
	}
}

// NewGenXWithMux creates a new GenX segmentor using a specific generator Mux.
func NewGenXWithMux(cfg Config, mux *generators.Mux) *GenX {
	return &GenX{
		generator: cfg.Generator,
		mux:       mux,
	}
}

// Model returns the generator pattern.
func (g *GenX) Model() string {
	return g.generator
}

// Process compresses the input messages into a segment by calling the LLM.
func (g *GenX) Process(ctx context.Context, input Input) (*Result, error) {
	mctx := g.buildModelContext(input)

	var (
		usage genx.Usage
		call  *genx.FuncCall
		err   error
	)
	if g.mux != nil {
		usage, call, err = g.mux.Invoke(ctx, g.generator, mctx, extractTool)
	} else {
		usage, call, err = generators.Invoke(ctx, g.generator, mctx, extractTool)
	}
	if err != nil {
		return nil, fmt.Errorf("segmentors: invoke failed: %w", err)
	}
	_ = usage

	return g.parseResult(call)
}

// buildModelContext constructs the genx.ModelContext for the LLM call.
func (g *GenX) buildModelContext(input Input) genx.ModelContext {
	var mcb genx.ModelContextBuilder

	// System prompt with instructions.
	mcb.PromptText("segmentor", buildPrompt(input))

	// Conversation as user message.
	mcb.UserText("conversation", buildConversationText(input.Messages))

	// Note: extractTool is NOT added here — it is passed as the fn argument
	// to generators.Invoke, which handles tool registration. Adding it here
	// would cause duplicate tool definitions in tool-calls mode.

	return mcb.Build()
}

// parseResult parses the FuncCall JSON arguments into a Result.
func (g *GenX) parseResult(call *genx.FuncCall) (*Result, error) {
	if call == nil {
		return nil, fmt.Errorf("segmentors: no function call returned")
	}

	var arg extractArg
	if err := json.Unmarshal([]byte(call.Arguments), &arg); err != nil {
		return nil, fmt.Errorf("segmentors: failed to parse extraction result: %w", err)
	}

	// Convert extractArg to Result, converting []extractAttr to map[string]any.
	// Values are stored as-is (string) — no type guessing. Attrs are used as
	// prompt context for LLMs, so string representation is all that's needed.
	entities := make([]EntityOutput, len(arg.Entities))
	for i, e := range arg.Entities {
		attrs := make(map[string]any, len(e.Attrs))
		for _, a := range e.Attrs {
			attrs[a.Key] = a.Value
		}
		entities[i] = EntityOutput{
			Label: e.Label,
			Attrs: attrs,
		}
	}

	return &Result{
		Segment: SegmentOutput{
			Summary:  arg.Segment.Summary,
			Keywords: arg.Segment.Keywords,
			Labels:   arg.Segment.Labels,
		},
		Entities:  entities,
		Relations: arg.Relations,
	}, nil
}

