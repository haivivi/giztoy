package profilers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"
	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

var _ Profiler = (*GenX)(nil)

// profileArg is the typed argument for the FuncTool, matching the expected
// JSON output from the LLM.
type profileArg struct {
	SchemaChanges  []SchemaChange            `json:"schema_changes"`
	ProfileUpdates map[string]map[string]any `json:"profile_updates"`
	Relations      []segmentors.RelationOutput `json:"relations"`
}

// profileTool is the FuncTool that defines the JSON schema for the LLM output.
var profileTool = genx.MustNewFuncTool[profileArg](
	"update_profiles",
	"Update entity profiles and propose schema changes based on conversation analysis.",
)

// GenX implements [Profiler] using a genx.Generator (LLM) for profile analysis.
type GenX struct {
	generator string // pattern for generators.DefaultMux
	mux       *generators.Mux
}

// NewGenX creates a new GenX profiler.
//
// cfg.Generator must be a pattern registered in generators.DefaultMux.
func NewGenX(cfg Config) *GenX {
	return &GenX{
		generator: cfg.Generator,
	}
}

// NewGenXWithMux creates a new GenX profiler using a specific generator Mux.
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

// Process analyzes the extracted metadata and conversation to produce
// schema changes and profile updates.
func (g *GenX) Process(ctx context.Context, input Input) (*Result, error) {
	mctx := g.buildModelContext(input)

	var (
		usage genx.Usage
		call  *genx.FuncCall
		err   error
	)
	if g.mux != nil {
		usage, call, err = g.mux.Invoke(ctx, g.generator, mctx, profileTool)
	} else {
		usage, call, err = generators.Invoke(ctx, g.generator, mctx, profileTool)
	}
	if err != nil {
		return nil, fmt.Errorf("profilers: invoke failed: %w", err)
	}
	_ = usage

	return g.parseResult(call)
}

// buildModelContext constructs the genx.ModelContext for the LLM call.
func (g *GenX) buildModelContext(input Input) genx.ModelContext {
	var mcb genx.ModelContextBuilder

	// System prompt with instructions.
	mcb.PromptText("profiler", buildPrompt(input))

	// Conversation as user message.
	mcb.UserText("conversation", buildConversationText(input.Messages))

	// Add the profile tool.
	mcb.AddTool(profileTool)

	return mcb.Build()
}

// parseResult parses the FuncCall JSON arguments into a Result.
func (g *GenX) parseResult(call *genx.FuncCall) (*Result, error) {
	if call == nil {
		return nil, fmt.Errorf("profilers: no function call returned")
	}

	var arg profileArg
	if err := json.Unmarshal([]byte(call.Arguments), &arg); err != nil {
		return nil, fmt.Errorf("profilers: failed to parse profile result: %w", err)
	}

	return &Result{
		SchemaChanges:  arg.SchemaChanges,
		ProfileUpdates: arg.ProfileUpdates,
		Relations:      arg.Relations,
	}, nil
}
