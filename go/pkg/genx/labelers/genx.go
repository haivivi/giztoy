package labelers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"
)

var _ Labeler = (*GenX)(nil)

type selectArg struct {
	Matches []Match `json:"matches"`
}

var selectTool = genx.MustNewFuncTool[selectArg](
	"select_labels",
	"Select relevant labels from candidate labels for recall.",
)

// GenX implements [Labeler] via a genx.Generator.
type GenX struct {
	generator string
	mux       *generators.Mux
}

// NewGenX creates a labeler bound to generators.DefaultMux.
func NewGenX(cfg Config) *GenX {
	return &GenX{generator: cfg.Generator}
}

// NewGenXWithMux creates a labeler bound to provided mux.
func NewGenXWithMux(cfg Config, mux *generators.Mux) *GenX {
	return &GenX{generator: cfg.Generator, mux: mux}
}

// Model returns generator pattern.
func (g *GenX) Model() string {
	return g.generator
}

// Process selects labels from candidates for input.Text.
func (g *GenX) Process(ctx context.Context, input Input) (*Result, error) {
	if len(input.Candidates) == 0 || input.Text == "" {
		return &Result{Matches: nil}, nil
	}

	var mcb genx.ModelContextBuilder
	mcb.PromptText("labeler", buildPrompt(input))

	var (
		call *genx.FuncCall
		err  error
	)
	if g.mux != nil {
		_, call, err = g.mux.Invoke(ctx, g.generator, mcb.Build(), selectTool)
	} else {
		_, call, err = generators.Invoke(ctx, g.generator, mcb.Build(), selectTool)
	}
	if err != nil {
		return nil, fmt.Errorf("labelers: invoke failed: %w", err)
	}

	return parseAndValidate(call, input)
}

func parseAndValidate(call *genx.FuncCall, input Input) (*Result, error) {
	if call == nil {
		return nil, fmt.Errorf("labelers: no function call returned")
	}

	var arg selectArg
	if err := json.Unmarshal([]byte(call.Arguments), &arg); err != nil {
		return nil, fmt.Errorf("labelers: failed to parse result: %w", err)
	}

	if len(arg.Matches) == 0 {
		return &Result{Matches: nil}, nil
	}

	candidates := make(map[string]struct{}, len(input.Candidates))
	for _, c := range input.Candidates {
		candidates[c] = struct{}{}
	}

	maxN := input.TopK
	if maxN <= 0 || maxN > len(input.Candidates) {
		maxN = len(input.Candidates)
	}

	result := make([]Match, 0, len(arg.Matches))
	seen := make(map[string]struct{}, len(arg.Matches))
	for _, m := range arg.Matches {
		if m.Label == "" {
			return nil, fmt.Errorf("labelers: match.label is required")
		}
		if _, ok := candidates[m.Label]; !ok {
			return nil, fmt.Errorf("labelers: label %q is not in candidates", m.Label)
		}
		if m.Score < 0 || m.Score > 1 {
			return nil, fmt.Errorf("labelers: label %q has invalid score %.4f", m.Label, m.Score)
		}
		if _, ok := seen[m.Label]; ok {
			continue
		}
		seen[m.Label] = struct{}{}
		result = append(result, m)
		if len(result) >= maxN {
			break
		}
	}

	if len(result) == 0 {
		return &Result{Matches: nil}, nil
	}
	return &Result{Matches: result}, nil
}
