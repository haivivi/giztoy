package profilers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"
	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

// ---------------------------------------------------------------------------
// Mock generator: captures ModelContext and returns canned JSON
// ---------------------------------------------------------------------------

type mockGenerator struct {
	name         string
	capturedMCtx genx.ModelContext
	response     string
	err          error
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
// Helper: build valid profileArg JSON
// ---------------------------------------------------------------------------

func validProfileJSON() string {
	arg := profileArg{
		SchemaChanges: []SchemaChange{
			{
				EntityType: "person",
				Field:      "school",
				Def:        segmentors.AttrDef{Type: "string", Desc: "学校名称"},
				Action:     "add",
			},
		},
		ProfileUpdates: map[string]map[string]any{
			"person:小明": {
				"age":               float64(5),
				"favorite_dinosaur": "霸王龙",
				"school":            "阳光幼儿园",
			},
		},
		Relations: []segmentors.RelationOutput{
			{From: "person:小明", To: "place:阳光幼儿园", RelType: "attends"},
		},
	}
	b, _ := json.Marshal(arg)
	return string(b)
}

func sampleExtracted() *segmentors.Result {
	return &segmentors.Result{
		Segment: segmentors.SegmentOutput{
			Summary:  "小明和爸爸聊了恐龙和幼儿园。",
			Keywords: []string{"恐龙", "幼儿园"},
			Labels:   []string{"person:小明", "person:爸爸", "topic:恐龙"},
		},
		Entities: []segmentors.EntityOutput{
			{Label: "person:小明", Attrs: map[string]any{"age": float64(5)}},
			{Label: "person:爸爸", Attrs: map[string]any{}},
			{Label: "topic:恐龙", Attrs: map[string]any{}},
		},
		Relations: []segmentors.RelationOutput{
			{From: "person:小明", To: "topic:恐龙", RelType: "likes"},
		},
	}
}

func sampleSchema() *segmentors.Schema {
	return &segmentors.Schema{
		EntityTypes: map[string]segmentors.EntitySchema{
			"person": {
				Desc: "A human person",
				Attrs: map[string]segmentors.AttrDef{
					"age":    {Type: "int", Desc: "Age in years"},
					"gender": {Type: "string", Desc: "Gender"},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Tests: GenX implementation
// ---------------------------------------------------------------------------

func TestGenX_Process(t *testing.T) {
	mock := &mockGenerator{name: "test", response: validProfileJSON()}
	mux := newTestGeneratorMux(t, "test/model", mock)
	prof := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	input := Input{
		Messages: []string{
			"user: 小明今天在幼儿园学了恐龙知识",
			"assistant: 小明说他最喜欢霸王龙",
		},
		Extracted: sampleExtracted(),
		Schema:    sampleSchema(),
		Profiles: map[string]map[string]any{
			"person:小明": {"age": float64(5)},
		},
	}

	result, err := prof.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify schema changes.
	if len(result.SchemaChanges) != 1 {
		t.Fatalf("expected 1 schema change, got %d", len(result.SchemaChanges))
	}
	sc := result.SchemaChanges[0]
	if sc.EntityType != "person" || sc.Field != "school" || sc.Action != "add" {
		t.Errorf("unexpected schema change: %+v", sc)
	}

	// Verify profile updates.
	if len(result.ProfileUpdates) != 1 {
		t.Fatalf("expected 1 profile update, got %d", len(result.ProfileUpdates))
	}
	xm, ok := result.ProfileUpdates["person:小明"]
	if !ok {
		t.Fatal("expected profile update for person:小明")
	}
	if xm["school"] != "阳光幼儿园" {
		t.Errorf("expected school=阳光幼儿园, got %v", xm["school"])
	}

	// Verify relations.
	if len(result.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(result.Relations))
	}
	if result.Relations[0].RelType != "attends" {
		t.Errorf("expected rel_type=attends, got %s", result.Relations[0].RelType)
	}
}

func TestGenX_PromptContainsAllSections(t *testing.T) {
	mock := &mockGenerator{name: "test", response: validProfileJSON()}
	mux := newTestGeneratorMux(t, "test/model", mock)
	prof := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	input := Input{
		Messages:  []string{"user: test message"},
		Extracted: sampleExtracted(),
		Schema:    sampleSchema(),
		Profiles: map[string]map[string]any{
			"person:小明": {"age": float64(5)},
		},
	}

	_, err := prof.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	var prompts []string
	for p := range mock.capturedMCtx.Prompts() {
		prompts = append(prompts, p.Text)
	}
	combined := strings.Join(prompts, "\n")

	checks := []string{
		"entity profile analyst",      // base prompt
		"Current Entity Schema",       // schema section
		"Existing Entity Profiles",    // profiles section
		"Extracted Metadata",          // extracted section
		"person:小明",                   // from extracted entities
		"schema_changes",              // output format
	}
	for _, check := range checks {
		if !strings.Contains(combined, check) {
			t.Errorf("prompt missing expected text: %q", check)
		}
	}
}

func TestGenX_NoSchemaNoProfiles(t *testing.T) {
	mock := &mockGenerator{name: "test", response: validProfileJSON()}
	mux := newTestGeneratorMux(t, "test/model", mock)
	prof := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	input := Input{
		Messages:  []string{"user: hello"},
		Extracted: sampleExtracted(),
	}

	_, err := prof.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	var prompts []string
	for p := range mock.capturedMCtx.Prompts() {
		prompts = append(prompts, p.Text)
	}
	combined := strings.Join(prompts, "\n")

	if strings.Contains(combined, "Current Entity Schema") {
		t.Error("should not include schema section when no schema provided")
	}
	if strings.Contains(combined, "Existing Entity Profiles") {
		t.Error("should not include profiles section when no profiles provided")
	}
}

func TestGenX_InvalidJSON(t *testing.T) {
	mock := &mockGenerator{name: "test", response: `{broken`}
	mux := newTestGeneratorMux(t, "test/model", mock)
	prof := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	_, err := prof.Process(context.Background(), Input{
		Messages:  []string{"hello"},
		Extracted: sampleExtracted(),
	})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGenX_InvokeError(t *testing.T) {
	mock := &mockGenerator{name: "test", err: context.DeadlineExceeded}
	mux := newTestGeneratorMux(t, "test/model", mock)
	prof := NewGenXWithMux(Config{Generator: "test/model"}, mux)

	_, err := prof.Process(context.Background(), Input{
		Messages:  []string{"hello"},
		Extracted: sampleExtracted(),
	})
	if err == nil {
		t.Error("expected error when generator fails")
	}
}

func TestGenX_Model(t *testing.T) {
	prof := NewGenX(Config{Generator: "qwen/turbo"})
	if got := prof.Model(); got != "qwen/turbo" {
		t.Errorf("Model() = %q, want %q", got, "qwen/turbo")
	}
}

// ---------------------------------------------------------------------------
// Tests: Mux
// ---------------------------------------------------------------------------

func TestMux_Handle(t *testing.T) {
	mux := NewMux()
	mock := &mockProfiler{model: "test"}

	if err := mux.Handle("test/prof", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Duplicate should fail.
	if err := mux.Handle("test/prof", mock); err == nil {
		t.Error("Handle() expected error for duplicate registration")
	}

	// Different pattern should succeed.
	if err := mux.Handle("test/prof2", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
}

func TestMux_Get(t *testing.T) {
	mux := NewMux()
	mock := &mockProfiler{model: "test"}

	if err := mux.Handle("test/prof", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	p, err := mux.Get("test/prof")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if p.Model() != "test" {
		t.Errorf("Get() model = %q, want %q", p.Model(), "test")
	}

	_, err = mux.Get("nonexistent")
	if err == nil {
		t.Error("Get() expected error for nonexistent pattern")
	}
}

func TestMux_Process(t *testing.T) {
	mux := NewMux()
	mock := &mockProfiler{
		model: "test",
		result: &Result{
			ProfileUpdates: map[string]map[string]any{
				"person:test": {"key": "value"},
			},
		},
	}

	if err := mux.Handle("test/prof", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	result, err := mux.Process(context.Background(), "test/prof", Input{Messages: []string{"hello"}})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if _, ok := result.ProfileUpdates["person:test"]; !ok {
		t.Error("expected profile update for person:test")
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
	DefaultMux = NewMux()

	mock := &mockProfiler{
		model: "default-test",
		result: &Result{
			ProfileUpdates: map[string]map[string]any{
				"person:test": {"key": "value"},
			},
		},
	}

	if err := Handle("default/prof", mock); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	p, err := Get("default/prof")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if p.Model() != "default-test" {
		t.Errorf("unexpected model: %q", p.Model())
	}

	result, err := Process(context.Background(), "default/prof", Input{Messages: []string{"hello"}})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if _, ok := result.ProfileUpdates["person:test"]; !ok {
		t.Error("expected profile update for person:test")
	}
}

// ---------------------------------------------------------------------------
// Tests: prompt construction
// ---------------------------------------------------------------------------

func TestBuildPrompt_AllSections(t *testing.T) {
	prompt := buildPrompt(Input{
		Messages:  []string{"hello"},
		Extracted: sampleExtracted(),
		Schema:    sampleSchema(),
		Profiles: map[string]map[string]any{
			"person:小明": {"age": float64(5)},
		},
	})

	checks := []string{
		"entity profile analyst",
		"Current Entity Schema",
		"Existing Entity Profiles",
		"Extracted Metadata",
		"schema_changes",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing: %q", check)
		}
	}
}

func TestBuildPrompt_MinimalInput(t *testing.T) {
	prompt := buildPrompt(Input{
		Messages: []string{"hello"},
	})

	if !strings.Contains(prompt, "entity profile analyst") {
		t.Error("prompt should contain base instructions")
	}
	if strings.Contains(prompt, "Current Entity Schema") {
		t.Error("should not include schema section")
	}
	if strings.Contains(prompt, "Existing Entity Profiles") {
		t.Error("should not include profiles section")
	}
}

func TestBuildExtractedSection(t *testing.T) {
	section := buildExtractedSection(sampleExtracted())

	if !strings.Contains(section, "person:小明") {
		t.Error("expected entity label in extracted section")
	}
	if !strings.Contains(section, "likes") {
		t.Error("expected relation type in extracted section")
	}
}

// ---------------------------------------------------------------------------
// Tests: JSON parsing edge cases
// ---------------------------------------------------------------------------

func TestParseResult_EmptyResult(t *testing.T) {
	arg := profileArg{
		SchemaChanges:  []SchemaChange{},
		ProfileUpdates: map[string]map[string]any{},
		Relations:      []segmentors.RelationOutput{},
	}
	b, _ := json.Marshal(arg)

	g := &GenX{}
	call := &genx.FuncCall{Name: "update_profiles", Arguments: string(b)}
	result, err := g.parseResult(call)
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if len(result.SchemaChanges) != 0 {
		t.Error("expected empty schema changes")
	}
	if len(result.ProfileUpdates) != 0 {
		t.Error("expected empty profile updates")
	}
}

func TestParseResult_NilCall(t *testing.T) {
	g := &GenX{}
	_, err := g.parseResult(nil)
	if err == nil {
		t.Error("expected error for nil call")
	}
}

// ---------------------------------------------------------------------------
// Mock profiler for mux tests
// ---------------------------------------------------------------------------

type mockProfiler struct {
	model  string
	result *Result
	err    error
}

func (m *mockProfiler) Process(_ context.Context, _ Input) (*Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockProfiler) Model() string { return m.model }

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
