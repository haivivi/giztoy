package labelers

import (
	"context"
	"errors"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"
)

type mockGenerator struct {
	response     string
	err          error
	capturedMCtx genx.ModelContext
}

func (m *mockGenerator) GenerateStream(_ context.Context, _ string, _ genx.ModelContext) (genx.Stream, error) {
	return nil, nil
}

func (m *mockGenerator) Invoke(_ context.Context, _ string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	m.capturedMCtx = mctx
	if m.err != nil {
		return genx.Usage{}, nil, m.err
	}
	return genx.Usage{}, tool.NewFuncCall(m.response), nil
}

func structCall(arguments string) genx.FuncCall {
	return genx.FuncCall{Name: "select_labels", Arguments: arguments}
}

func newTestGeneratorMux(t *testing.T, pattern string, gen genx.Generator) *generators.Mux {
	t.Helper()
	mux := generators.NewMux()
	if err := mux.Handle(pattern, gen); err != nil {
		t.Fatal(err)
	}
	return mux
}

func TestGenXProcessSuccess(t *testing.T) {
	mock := &mockGenerator{response: `{"matches":[{"label":"person:小明","score":0.92},{"label":"topic:恐龙","score":0.88}]}`}
	g := NewGenXWithMux(Config{Generator: "test/model"}, newTestGeneratorMux(t, "test/model", mock))

	res, err := g.Process(context.Background(), Input{
		Text:       "我昨天和小明聊恐龙",
		Candidates: []string{"person:小明", "person:小红", "topic:恐龙"},
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(res.Matches))
	}
}

func TestGenXPromptGeneration(t *testing.T) {
	mock := &mockGenerator{response: `{"matches":[]}`}
	g := NewGenXWithMux(Config{Generator: "test/model"}, newTestGeneratorMux(t, "test/model", mock))

	_, err := g.Process(context.Background(), Input{
		Text:       "小明在上海工作",
		Candidates: []string{"person:小明", "place:上海"},
		Aliases: map[string][]string{
			"person:小明": {"明明"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mock.capturedMCtx == nil {
		t.Fatal("expected captured model context")
	}
}

func TestGenXParseInvalidJSON(t *testing.T) {
	call := structCall(`{bad json`)
	_, err := parseAndValidate(&call, Input{Candidates: []string{"person:小明"}})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGenXParseMissingFields(t *testing.T) {
	call := structCall(`{"matches":[{"score":0.5}]}`)
	_, err := parseAndValidate(&call, Input{Candidates: []string{"person:小明"}})
	if err == nil {
		t.Fatal("expected missing label error")
	}
}

func TestGenXParseValidJSON(t *testing.T) {
	call := structCall(`{"matches":[{"label":"person:小明","score":0.75}]}`)
	res, err := parseAndValidate(&call, Input{Candidates: []string{"person:小明"}})
	if err != nil {
		t.Fatalf("parseAndValidate() error = %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0].Label != "person:小明" {
		t.Fatalf("matches = %+v, want person:小明", res.Matches)
	}
}

func TestGenXModelMethod(t *testing.T) {
	g := NewGenX(Config{Generator: "qwen/flash"})
	if got := g.Model(); got != "qwen/flash" {
		t.Fatalf("Model() = %q, want %q", got, "qwen/flash")
	}
}

func TestGenXInvokeError(t *testing.T) {
	mock := &mockGenerator{err: errors.New("llm failure")}
	g := NewGenXWithMux(Config{Generator: "test/model"}, newTestGeneratorMux(t, "test/model", mock))
	_, err := g.Process(context.Background(), Input{Text: "聊恐龙", Candidates: []string{"topic:恐龙"}})
	if err == nil {
		t.Fatal("expected invoke error")
	}
}
