package agent_test

import (
	"context"
	"os"
	"testing"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/agent"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/pkg/genx/playground"
)

func setupGeneratorToolTestRuntime(t *testing.T) *playground.Runtime {
	t.Helper()
	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/tool_generator_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}
	return playground.NewRuntime(playground.WithStore(store))
}

func TestGeneratorTool_BuildModelContextWithInput(t *testing.T) {
	ctx := context.Background()

	t.Run("simple_prompt", func(t *testing.T) {
		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "test",
				Description: "test tool",
			},
			Prompt: "You are a helpful assistant.",
			Model:  "gpt-4o",
			Mode:   "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "Hello!")
		if err != nil {
			t.Fatalf("BuildModelContextWithInput error: %v", err)
		}

		// Check prompts
		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		if len(prompts) != 1 {
			t.Errorf("got %d prompts, want 1", len(prompts))
		}
		if prompts[0].Text != "You are a helpful assistant." {
			t.Errorf("prompt text = %q, want %q", prompts[0].Text, "You are a helpful assistant.")
		}

		// Check user input (via Messages)
		var messages []*genx.Message
		for m := range mctx.Messages() {
			messages = append(messages, m)
		}
		if len(messages) != 1 {
			t.Errorf("got %d messages, want 1", len(messages))
		}
		if messages[0].Role != "user" {
			t.Errorf("message role = %q, want %q", messages[0].Role, "user")
		}
	})

	t.Run("context_layers_inline_prompt", func(t *testing.T) {
		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			ContextLayers: []agentcfg.ContextLayer{
				{Prompt: "Layer 1"},
				{Prompt: "Layer 2"},
			},
			Model: "gpt-4o",
			Mode:  "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "test input")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		if len(prompts) != 2 {
			t.Errorf("got %d prompts, want 2", len(prompts))
		}
	})

	t.Run("context_layers_ref", func(t *testing.T) {
		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			ContextLayers: []agentcfg.ContextLayer{
				{Ref: "character_elsa"},
			},
			Model: "gpt-4o",
			Mode:  "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "test input")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		if len(prompts) != 1 {
			t.Errorf("got %d prompts, want 1", len(prompts))
		}
		if prompts[0].Text != "You are Elsa from Frozen." {
			t.Errorf("prompt text = %q, want %q", prompts[0].Text, "You are Elsa from Frozen.")
		}
	})

	t.Run("context_layers_multiple_refs", func(t *testing.T) {
		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			ContextLayers: []agentcfg.ContextLayer{
				{Ref: "system_prompt"},
				{Ref: "personality_friendly"},
				{Ref: "output_format_json"},
			},
			Model: "gpt-4o",
			Mode:  "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "test input")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		if len(prompts) != 3 {
			t.Errorf("got %d prompts, want 3", len(prompts))
		}

		// Check prompts in order
		expected := []string{
			"You are a helpful AI assistant.",
			"You should be friendly and warm in your responses.",
			"Always respond in valid JSON format.",
		}
		for i, want := range expected {
			if i >= len(prompts) {
				break
			}
			if prompts[i].Text != want {
				t.Errorf("prompt[%d] = %q, want %q", i, prompts[i].Text, want)
			}
		}
	})

	t.Run("context_layers_ref_with_base_prompt", func(t *testing.T) {
		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			Prompt: "Base system prompt.",
			ContextLayers: []agentcfg.ContextLayer{
				{Ref: "character_elsa"},
				{Prompt: "Additional inline instruction."},
			},
			Model: "gpt-4o",
			Mode:  "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "test input")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		// Base prompt + 2 context layers
		if len(prompts) != 3 {
			t.Errorf("got %d prompts, want 3", len(prompts))
		}

		expected := []string{
			"Base system prompt.",
			"You are Elsa from Frozen.",
			"Additional inline instruction.",
		}
		for i, want := range expected {
			if i >= len(prompts) {
				break
			}
			if prompts[i].Text != want {
				t.Errorf("prompt[%d] = %q, want %q", i, prompts[i].Text, want)
			}
		}
	})

	t.Run("context_layers_env", func(t *testing.T) {
		// Set test env var
		os.Setenv("TEST_GENERATOR_PROMPT", "Prompt from env")
		defer os.Unsetenv("TEST_GENERATOR_PROMPT")

		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			ContextLayers: []agentcfg.ContextLayer{
				{Env: "TEST_GENERATOR_PROMPT"},
			},
			Model: "gpt-4o",
			Mode:  "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "test input")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		if len(prompts) != 1 {
			t.Errorf("got %d prompts, want 1", len(prompts))
		}
		if prompts[0].Text != "Prompt from env" {
			t.Errorf("prompt text = %q, want %q", prompts[0].Text, "Prompt from env")
		}
	})

	t.Run("context_layers_this", func(t *testing.T) {
		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "story_writer",
				Description: "A creative story writer",
			},
			ContextLayers: []agentcfg.ContextLayer{
				{This: ".description"},
			},
			Model: "gpt-4o",
			Mode:  "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "test input")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		if len(prompts) != 1 {
			t.Errorf("got %d prompts, want 1", len(prompts))
		}
		if prompts[0].Text != "A creative story writer" {
			t.Errorf("prompt text = %q, want %q", prompts[0].Text, "A creative story writer")
		}
	})

	t.Run("context_layers_this_fields", func(t *testing.T) {
		// Test various .this field resolutions
		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		tests := []struct {
			field    string
			expected string
		}{
			{".name", "test_tool"},
			{"name", "test_tool"}, // without leading dot
			{".description", "Test description"},
			{".prompt", "Test prompt"},
			{".model", "gpt-4o"},
			{".mode", "generate"},
		}

		for _, tt := range tests {
			t.Run(tt.field, func(t *testing.T) {
				def := &agentcfg.GeneratorTool{
					ToolBase: agentcfg.ToolBase{
						Name:        "test_tool",
						Description: "Test description",
					},
					Prompt: "Test prompt",
					Model:  "gpt-4o",
					Mode:   "generate",
					ContextLayers: []agentcfg.ContextLayer{
						{This: tt.field},
					},
				}

				mctx, err := gt.BuildModelContextWithInput(ctx, def, "input")
				if err != nil {
					t.Fatalf("error: %v", err)
				}

				var prompts []*genx.Prompt
				for p := range mctx.Prompts() {
					prompts = append(prompts, p)
				}

				// First prompt is from Prompt field, second is from ContextLayers
				if len(prompts) < 2 {
					t.Fatalf("got %d prompts, want at least 2", len(prompts))
				}
				// The context layer prompt should be the second one
				if prompts[1].Text != tt.expected {
					t.Errorf("context layer prompt text = %q, want %q", prompts[1].Text, tt.expected)
				}
			})
		}
	})

	t.Run("mixed_context_layers", func(t *testing.T) {
		os.Setenv("TEST_EXTRA_PROMPT", "Extra from env")
		defer os.Unsetenv("TEST_EXTRA_PROMPT")

		rt := setupGeneratorToolTestRuntime(t)
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "mixed_test",
				Description: "Mixed test tool",
			},
			Prompt: "Base prompt",
			ContextLayers: []agentcfg.ContextLayer{
				{Ref: "character_helper"},
				{Env: "TEST_EXTRA_PROMPT"},
				{Prompt: "Inline layer"},
				{This: ".name"},
			},
			Model: "gpt-4o",
			Mode:  "generate",
		}

		mctx, err := gt.BuildModelContextWithInput(ctx, def, "Hello world")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var prompts []*genx.Prompt
		for p := range mctx.Prompts() {
			prompts = append(prompts, p)
		}
		// Base prompt + 4 context layers
		if len(prompts) != 5 {
			t.Errorf("got %d prompts, want 5", len(prompts))
		}

		// Check user input (via Messages)
		var messages []*genx.Message
		for m := range mctx.Messages() {
			messages = append(messages, m)
		}
		if len(messages) != 1 {
			t.Errorf("got %d messages, want 1", len(messages))
		}
		if messages[0].Role != "user" {
			t.Errorf("message role = %q, want %q", messages[0].Role, "user")
		}
	})
}

// mockGeneratorStream implements genx.Stream for testing.
type mockGeneratorStream struct {
	chunks []string
	idx    int
}

func (s *mockGeneratorStream) Next() (*genx.MessageChunk, error) {
	if s.idx >= len(s.chunks) {
		return nil, genx.Done(genx.Usage{})
	}
	chunk := &genx.MessageChunk{Part: genx.Text(s.chunks[s.idx])}
	s.idx++
	return chunk, nil
}

func (s *mockGeneratorStream) Close() error               { return nil }
func (s *mockGeneratorStream) CloseWithError(error) error { return nil }

// mockGeneratorRuntime provides a mock runtime for GeneratorTool testing.
type mockGeneratorRuntime struct {
	playground.Runtime
	streamChunks []string
}

func (r *mockGeneratorRuntime) GenerateStream(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
	return &mockGeneratorStream{chunks: r.streamChunks}, nil
}

func TestGeneratorTool_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("Execute_string_input", func(t *testing.T) {
		store := playground.NewStore(nil)
		baseRt := playground.NewRuntime(playground.WithStore(store))
		mockRt := &mockGeneratorRuntime{
			Runtime:      *baseRt,
			streamChunks: []string{"Hello", " ", "World"},
		}

		gt := agent.NewGeneratorTool(mockRt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "test",
				Description: "Test generator",
			},
			Prompt: "You are a helpful assistant.",
			Model:  "test-model",
			Mode:   "generate",
		}

		result, err := gt.Execute(ctx, def, "test input")
		if err != nil {
			t.Fatalf("Execute error: %v", err)
		}
		if result != "Hello World" {
			t.Errorf("got %q, want %q", result, "Hello World")
		}
	})

	t.Run("Execute_TextProcessorInput", func(t *testing.T) {
		store := playground.NewStore(nil)
		baseRt := playground.NewRuntime(playground.WithStore(store))
		mockRt := &mockGeneratorRuntime{
			Runtime:      *baseRt,
			streamChunks: []string{"Processed", " result"},
		}

		gt := agent.NewGeneratorTool(mockRt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			Prompt: "Process this",
			Model:  "test-model",
			Mode:   "generate",
		}

		input := agentcfg.TextProcessorInput{Content: "input content"}
		result, err := gt.Execute(ctx, def, input)
		if err != nil {
			t.Fatalf("Execute error: %v", err)
		}
		if result != "Processed result" {
			t.Errorf("got %q, want %q", result, "Processed result")
		}
	})

	t.Run("Execute_map_input", func(t *testing.T) {
		store := playground.NewStore(nil)
		baseRt := playground.NewRuntime(playground.WithStore(store))
		mockRt := &mockGeneratorRuntime{
			Runtime:      *baseRt,
			streamChunks: []string{"Result"},
		}

		gt := agent.NewGeneratorTool(mockRt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			Prompt: "Process",
			Model:  "test-model",
			Mode:   "generate",
		}

		input := map[string]any{"key": "value"}
		result, err := gt.Execute(ctx, def, input)
		if err != nil {
			t.Fatalf("Execute error: %v", err)
		}
		if result != "Result" {
			t.Errorf("got %q, want %q", result, "Result")
		}
	})

	t.Run("Execute_missing_model", func(t *testing.T) {
		store := playground.NewStore(nil)
		rt := playground.NewRuntime(playground.WithStore(store))
		gt := agent.NewGeneratorTool(rt)

		def := &agentcfg.GeneratorTool{
			ToolBase: agentcfg.ToolBase{
				Name: "test",
			},
			Prompt: "Test",
			Mode:   "generate",
			// Model is missing
		}

		_, err := gt.Execute(ctx, def, "input")
		if err == nil {
			t.Error("Expected error for missing model")
		}
	})
}

// TestGeneratorTool_JSONOutputMode tests the json_output mode via CreateFuncTool.
func TestGeneratorTool_JSONOutputMode(t *testing.T) {
	ctx := context.Background()

	// Create a mock generator that supports Invoke for json_output mode
	mockGen := &mockInvokeGenerator{
		funcCallResult: &genx.FuncCall{
			Name:      "output",
			Arguments: `{"name": "test", "value": 42}`,
		},
	}

	store := playground.NewStore(nil)
	rt := playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
	)

	gt := agent.NewGeneratorTool(rt)

	// Parse JSON schema
	schemaJSON := `{"type": "object", "properties": {"name": {"type": "string"}, "value": {"type": "number"}}}`
	var schema agentcfg.JSONSchema
	if err := schema.UnmarshalJSON([]byte(schemaJSON)); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	def := &agentcfg.GeneratorTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "json_extractor",
			Description: "Extract structured data",
		},
		Prompt:       "Extract data from input",
		Model:        "test-model",
		Mode:         agentcfg.GeneratorModeJSONOutput,
		OutputSchema: &schema,
	}

	// Create the FuncTool
	tool, err := gt.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	// Invoke the tool
	result, err := tool.Invoke(ctx, nil, `{"input": "some input text"}`)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	// Result should be the JSON arguments from the function call
	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}
	if resultStr != `{"name": "test", "value": 42}` {
		t.Errorf("Result = %q, want %q", resultStr, `{"name": "test", "value": 42}`)
	}
}

// mockInvokeGenerator is a mock generator that supports Invoke for structured output.
type mockInvokeGenerator struct {
	funcCallResult *genx.FuncCall
	invokeErr      error
}

func (g *mockInvokeGenerator) GenerateStream(ctx context.Context, model string, mc genx.ModelContext) (genx.Stream, error) {
	return &mockGeneratorStream{chunks: []string{"mock response"}}, nil
}

func (g *mockInvokeGenerator) Invoke(ctx context.Context, model string, mc genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	if g.invokeErr != nil {
		return genx.Usage{}, nil, g.invokeErr
	}
	return genx.Usage{}, g.funcCallResult, nil
}
