package segmentors

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"
)

// ---------------------------------------------------------------------------
// Mock generator: captures ModelContext and returns canned JSON
// ---------------------------------------------------------------------------

type mockGenerator struct {
	name string

	// capturedMCtx is the last ModelContext passed to Invoke.
	capturedMCtx genx.ModelContext

	// response is the JSON to return as FuncCall.Arguments.
	response string

	// err is returned by Invoke if non-nil.
	err error
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

// ---------------------------------------------------------------------------
// Helper: build a valid extractArg JSON
// ---------------------------------------------------------------------------

func validExtractJSON() string {
	arg := extractArg{
		Segment: SegmentOutput{
			Summary:  "小明和爸爸聊了恐龙，小明最喜欢霸王龙。",
			Keywords: []string{"恐龙", "霸王龙", "小明"},
			Labels:   []string{"person:小明", "person:爸爸", "topic:恐龙"},
		},
		Entities: []EntityOutput{
			{Label: "person:小明", Attrs: map[string]any{"age": float64(5), "favorite_dinosaur": "霸王龙"}},
			{Label: "person:爸爸", Attrs: map[string]any{}},
			{Label: "topic:恐龙", Attrs: map[string]any{"category": "古生物"}},
		},
		Relations: []RelationOutput{
			{From: "person:小明", To: "topic:恐龙", RelType: "likes"},
			{From: "person:爸爸", To: "person:小明", RelType: "parent"},
		},
	}
	b, _ := json.Marshal(arg)
	return string(b)
}

// ---------------------------------------------------------------------------
// Tests: GenX implementation
// ---------------------------------------------------------------------------

func TestGenX_Process(t *testing.T) {
	mock := &mockGenerator{name: "test", response: validExtractJSON()}

	mux := newTestGeneratorMux(t, "test/model", mock)
	seg := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	input := Input{
		Messages: []string{
			"user: 小明今天和爸爸聊恐龙",
			"assistant: 小明说他最喜欢霸王龙",
		},
	}

	result, err := seg.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify segment output.
	if result.Segment.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if len(result.Segment.Keywords) != 3 {
		t.Errorf("expected 3 keywords, got %d", len(result.Segment.Keywords))
	}
	if len(result.Segment.Labels) != 3 {
		t.Errorf("expected 3 labels, got %d", len(result.Segment.Labels))
	}

	// Verify entities.
	if len(result.Entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(result.Entities))
	}
	found := false
	for _, e := range result.Entities {
		if e.Label == "person:小明" {
			found = true
			if e.Attrs["favorite_dinosaur"] != "霸王龙" {
				t.Errorf("expected favorite_dinosaur=霸王龙, got %v", e.Attrs["favorite_dinosaur"])
			}
		}
	}
	if !found {
		t.Error("expected entity person:小明")
	}

	// Verify relations.
	if len(result.Relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(result.Relations))
	}
}

func TestGenX_PromptContainsMessages(t *testing.T) {
	mock := &mockGenerator{name: "test", response: validExtractJSON()}

	mux := newTestGeneratorMux(t, "test/model", mock)
	seg := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	input := Input{
		Messages: []string{
			"user: 你好世界",
			"assistant: 世界你好",
		},
	}

	_, err := seg.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Check that the captured ModelContext contains the messages.
	if mock.capturedMCtx == nil {
		t.Fatal("no ModelContext captured")
	}

	var prompts []string
	for p := range mock.capturedMCtx.Prompts() {
		prompts = append(prompts, p.Text)
	}
	if len(prompts) == 0 {
		t.Error("expected at least one prompt")
	}

	// Prompt should contain segmentor instructions.
	if !strings.Contains(prompts[0], "conversation segmentor") {
		t.Error("expected prompt to contain 'conversation segmentor'")
	}

	// Messages should be included in the user message.
	var msgs []string
	for m := range mock.capturedMCtx.Messages() {
		if m.Role == genx.RoleUser {
			if cs, ok := m.Payload.(genx.Contents); ok {
				for _, p := range cs {
					if txt, ok := p.(genx.Text); ok {
						msgs = append(msgs, string(txt))
					}
				}
			}
		}
	}
	if len(msgs) == 0 {
		t.Fatal("no user messages found")
	}
	if !strings.Contains(msgs[0], "你好世界") {
		t.Errorf("expected user message to contain '你好世界', got %q", msgs[0])
	}
}

func TestGenX_SchemaHint(t *testing.T) {
	mock := &mockGenerator{name: "test", response: validExtractJSON()}

	mux := newTestGeneratorMux(t, "test/model", mock)
	seg := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	input := Input{
		Messages: []string{"user: hello"},
		Schema: &Schema{
			EntityTypes: map[string]EntitySchema{
				"person": {
					Desc: "A human person",
					Attrs: map[string]AttrDef{
						"age":    {Type: "int", Desc: "Age in years"},
						"gender": {Type: "string", Desc: "Gender"},
					},
				},
			},
		},
	}

	_, err := seg.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Check that the prompt includes the schema hint.
	var prompts []string
	for p := range mock.capturedMCtx.Prompts() {
		prompts = append(prompts, p.Text)
	}
	combined := strings.Join(prompts, "\n")
	if !strings.Contains(combined, "Entity Schema Hint") {
		t.Error("expected prompt to contain schema hint")
	}
	if !strings.Contains(combined, "person") {
		t.Error("expected prompt to mention 'person' entity type")
	}
	if !strings.Contains(combined, "age") {
		t.Error("expected prompt to mention 'age' attribute")
	}
}

func TestGenX_NoSchema(t *testing.T) {
	mock := &mockGenerator{name: "test", response: validExtractJSON()}

	mux := newTestGeneratorMux(t, "test/model", mock)
	seg := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	input := Input{
		Messages: []string{"user: hello"},
	}

	_, err := seg.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	var prompts []string
	for p := range mock.capturedMCtx.Prompts() {
		prompts = append(prompts, p.Text)
	}
	combined := strings.Join(prompts, "\n")
	if strings.Contains(combined, "Entity Schema Hint") {
		t.Error("expected prompt to NOT contain schema hint when no schema provided")
	}
}

func TestGenX_InvalidJSON(t *testing.T) {
	mock := &mockGenerator{name: "test", response: `{invalid json`}

	mux := newTestGeneratorMux(t, "test/model", mock)
	seg := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	_, err := seg.Process(context.Background(), Input{Messages: []string{"hello"}})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGenX_InvokeError(t *testing.T) {
	mock := &mockGenerator{name: "test", err: context.DeadlineExceeded}

	mux := newTestGeneratorMux(t, "test/model", mock)
	seg := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	_, err := seg.Process(context.Background(), Input{Messages: []string{"hello"}})
	if err == nil {
		t.Error("expected error when generator fails")
	}
}

func TestGenX_Model(t *testing.T) {
	seg := NewGenX(Config{Generator: "qwen/turbo"})
	if got := seg.Model(); got != "qwen/turbo" {
		t.Errorf("Model() = %q, want %q", got, "qwen/turbo")
	}
}

// ---------------------------------------------------------------------------
// Tests: Mux
// ---------------------------------------------------------------------------

func TestMux_Handle(t *testing.T) {
	mux := NewMux()
	mock := &mockSegmentor{model: "test"}

	if err := mux.Handle("test/seg", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Duplicate should fail.
	if err := mux.Handle("test/seg", mock); err == nil {
		t.Error("Handle() expected error for duplicate registration")
	}

	// Different pattern should succeed.
	if err := mux.Handle("test/seg2", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
}

func TestMux_Get(t *testing.T) {
	mux := NewMux()
	mock := &mockSegmentor{model: "test"}

	if err := mux.Handle("test/seg", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	s, err := mux.Get("test/seg")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if s.Model() != "test" {
		t.Errorf("Get() returned segmentor with model %q, want %q", s.Model(), "test")
	}

	// Not found.
	_, err = mux.Get("nonexistent")
	if err == nil {
		t.Error("Get() expected error for nonexistent pattern")
	}
}

func TestMux_Process(t *testing.T) {
	mux := NewMux()
	mock := &mockSegmentor{
		model: "test",
		result: &Result{
			Segment: SegmentOutput{Summary: "test summary"},
		},
	}

	if err := mux.Handle("test/seg", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	result, err := mux.Process(context.Background(), "test/seg", Input{Messages: []string{"hello"}})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result.Segment.Summary != "test summary" {
		t.Errorf("unexpected summary: %q", result.Segment.Summary)
	}
}

func TestMux_ProcessNotFound(t *testing.T) {
	mux := NewMux()

	_, err := mux.Process(context.Background(), "nonexistent", Input{Messages: []string{"hello"}})
	if err == nil {
		t.Error("Process() expected error for nonexistent pattern")
	}
}

func TestDefaultMux_PackageFunctions(t *testing.T) {
	// Reset DefaultMux for test isolation.
	DefaultMux = NewMux()

	mock := &mockSegmentor{
		model: "default-test",
		result: &Result{
			Segment: SegmentOutput{Summary: "default summary"},
		},
	}

	if err := Handle("default/seg", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	s, err := Get("default/seg")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if s.Model() != "default-test" {
		t.Errorf("unexpected model: %q", s.Model())
	}

	result, err := Process(context.Background(), "default/seg", Input{Messages: []string{"hello"}})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result.Segment.Summary != "default summary" {
		t.Errorf("unexpected summary: %q", result.Segment.Summary)
	}
}

// ---------------------------------------------------------------------------
// Tests: prompt construction
// ---------------------------------------------------------------------------

func TestBuildPrompt_Base(t *testing.T) {
	prompt := buildPrompt(Input{Messages: []string{"hello"}})
	if !strings.Contains(prompt, "conversation segmentor") {
		t.Error("prompt should contain base instructions")
	}
	if !strings.Contains(prompt, "Output") {
		t.Error("prompt should contain output format")
	}
}

func TestBuildPrompt_WithSchema(t *testing.T) {
	prompt := buildPrompt(Input{
		Messages: []string{"hello"},
		Schema: &Schema{
			EntityTypes: map[string]EntitySchema{
				"animal": {
					Desc: "An animal species",
					Attrs: map[string]AttrDef{
						"habitat": {Type: "string", Desc: "Where it lives"},
					},
				},
			},
		},
	})
	if !strings.Contains(prompt, "Entity Schema Hint") {
		t.Error("prompt should contain schema hint")
	}
	if !strings.Contains(prompt, "animal") {
		t.Error("prompt should mention 'animal'")
	}
	if !strings.Contains(prompt, "habitat") {
		t.Error("prompt should mention 'habitat'")
	}
}

func TestBuildConversationText(t *testing.T) {
	text := buildConversationText([]string{"line1", "line2", "line3"})
	if text != "line1\nline2\nline3" {
		t.Errorf("unexpected conversation text: %q", text)
	}
}

// ---------------------------------------------------------------------------
// Tests: JSON parsing edge cases
// ---------------------------------------------------------------------------

func TestParseResult_EmptyEntities(t *testing.T) {
	arg := extractArg{
		Segment: SegmentOutput{
			Summary:  "just a summary",
			Keywords: []string{"test"},
			Labels:   []string{},
		},
		Entities:  []EntityOutput{},
		Relations: []RelationOutput{},
	}
	b, _ := json.Marshal(arg)

	g := &GenX{}
	call := &genx.FuncCall{Name: "extract", Arguments: string(b)}
	result, err := g.parseResult(call)
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if result.Segment.Summary != "just a summary" {
		t.Error("unexpected summary")
	}
	if len(result.Entities) != 0 {
		t.Error("expected empty entities")
	}
}

func TestParseResult_NilCall(t *testing.T) {
	g := &GenX{}
	_, err := g.parseResult(nil)
	if err == nil {
		t.Error("expected error for nil call")
	}
}

func TestParseResult_MalformedJSON(t *testing.T) {
	g := &GenX{}
	call := &genx.FuncCall{Name: "extract", Arguments: `{broken`}
	_, err := g.parseResult(call)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// Mock segmentor for mux tests
// ---------------------------------------------------------------------------

type mockSegmentor struct {
	model  string
	result *Result
	err    error
}

func (m *mockSegmentor) Process(_ context.Context, _ Input) (*Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockSegmentor) Model() string { return m.model }

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestGeneratorMux(t *testing.T, pattern string, gen genx.Generator) *generators.Mux {
	t.Helper()
	mux := generators.NewMux()
	if err := mux.Handle(pattern, gen); err != nil {
		t.Fatal(err)
	}
	return mux
}
