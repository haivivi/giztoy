package agent

import (
	"context"

	"github.com/haivivi/giztoy/go/pkg/genx"
)

// NewMatchAgentGenerator creates a matchAgentGenerator for testing.
// This is exported only for testing purposes.
func NewMatchAgentGenerator(rt Runtime, model string) genx.Generator {
	return &matchAgentGenerator{rt: rt, model: model}
}

// InvokeMatchAgentGenerator calls matchAgentGenerator.Invoke for testing.
func InvokeMatchAgentGenerator(ctx context.Context, rt Runtime, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	gen := &matchAgentGenerator{rt: rt, model: model}
	return gen.Invoke(ctx, "", mctx, tool)
}
