package segmentors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testdataPath(parts ...string) string {
	return filepath.Join("..", "..", "..", "..", "testdata", "genx", filepath.Join(parts...))
}

func TestGolden_SegmentorResult_ParseMockResponse(t *testing.T) {
	// Load mock LLM response (extractArg format with attrs as []extractAttr)
	data, err := os.ReadFile(testdataPath("segmentors", "mock_llm_response.json"))
	if err != nil {
		t.Fatalf("read mock response: %v", err)
	}

	var arg extractArg
	if err := json.Unmarshal(data, &arg); err != nil {
		t.Fatalf("unmarshal extractArg: %v", err)
	}

	if arg.Segment.Summary == "" {
		t.Error("summary should not be empty")
	}
	if len(arg.Entities) == 0 {
		t.Error("entities should not be empty")
	}
	if len(arg.Relations) == 0 {
		t.Error("relations should not be empty")
	}

	// Verify entity attrs conversion
	for _, e := range arg.Entities {
		if e.Label == "person:小明" {
			if len(e.Attrs) == 0 {
				t.Error("person:小明 should have attrs")
			}
			found := false
			for _, a := range e.Attrs {
				if a.Key == "age" && a.Value == "5" {
					found = true
				}
			}
			if !found {
				t.Error("person:小明 should have age=5")
			}
		}
	}
}

func TestGolden_SegmentorResult_ExpectedResult(t *testing.T) {
	data, err := os.ReadFile(testdataPath("segmentors", "expected_result.json"))
	if err != nil {
		t.Fatalf("read expected result: %v", err)
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal Result: %v", err)
	}

	if result.Segment.Summary != "小明和用户聊了恐龙，小明最喜欢霸王龙，小明5岁" {
		t.Errorf("summary = %q", result.Segment.Summary)
	}
	if len(result.Entities) != 2 {
		t.Errorf("entities count = %d, want 2", len(result.Entities))
	}
	if len(result.Relations) != 1 {
		t.Errorf("relations count = %d, want 1", len(result.Relations))
	}
}

func TestGolden_InputBasic(t *testing.T) {
	data, err := os.ReadFile(testdataPath("segmentors", "input_basic.json"))
	if err != nil {
		t.Fatalf("read input: %v", err)
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("unmarshal Input: %v", err)
	}

	if len(input.Messages) != 3 {
		t.Errorf("messages count = %d, want 3", len(input.Messages))
	}
	if input.Schema != nil {
		t.Error("schema should be nil for basic input")
	}
}

func TestGolden_InputWithSchema(t *testing.T) {
	data, err := os.ReadFile(testdataPath("segmentors", "input_with_schema.json"))
	if err != nil {
		t.Fatalf("read input: %v", err)
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("unmarshal Input: %v", err)
	}

	if len(input.Messages) != 3 {
		t.Errorf("messages count = %d, want 3", len(input.Messages))
	}
	if input.Schema == nil {
		t.Fatal("schema should not be nil")
	}
	if len(input.Schema.EntityTypes) != 2 {
		t.Errorf("entity_types count = %d, want 2", len(input.Schema.EntityTypes))
	}
	personSchema, ok := input.Schema.EntityTypes["person"]
	if !ok {
		t.Fatal("person schema missing")
	}
	if len(personSchema.Attrs) != 2 {
		t.Errorf("person attrs count = %d, want 2", len(personSchema.Attrs))
	}
}

func TestGolden_PromptBasic(t *testing.T) {
	input := Input{
		Messages: []string{
			"user: 今天和小明聊了恐龙",
			"assistant: 小明最喜欢霸王龙",
			"user: 他说他5岁了",
		},
	}

	prompt := buildPrompt(input)
	if prompt == "" {
		t.Error("prompt should not be empty")
	}
	if len(prompt) < 100 {
		t.Errorf("prompt too short: %d chars", len(prompt))
	}
	// Verify key sections present
	if !contains(prompt, "conversation segmentor") {
		t.Error("prompt should contain 'conversation segmentor'")
	}
	if !contains(prompt, "## Output") {
		t.Error("prompt should contain '## Output'")
	}
	if contains(prompt, "Entity Schema Hint") {
		t.Error("basic prompt should not contain schema hint")
	}
}

func TestGolden_PromptWithSchema(t *testing.T) {
	input := Input{
		Messages: []string{
			"user: 今天和小明聊了恐龙",
		},
		Schema: &Schema{
			EntityTypes: map[string]EntitySchema{
				"person": {
					Desc: "A person",
					Attrs: map[string]AttrDef{
						"age": {Type: "string", Desc: "Age of the person"},
					},
				},
			},
		},
	}

	prompt := buildPrompt(input)
	if !contains(prompt, "Entity Schema Hint") {
		t.Error("prompt should contain schema hint")
	}
	if !contains(prompt, "### person") {
		t.Error("prompt should contain person schema")
	}
	if !contains(prompt, "`age`") {
		t.Error("prompt should contain age attribute")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
