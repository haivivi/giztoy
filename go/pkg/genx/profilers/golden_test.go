package profilers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

func testdataPath(parts ...string) string {
	return filepath.Join("..", "..", "..", "..", "testdata", "genx", filepath.Join(parts...))
}

func TestGolden_ProfilerResult_ParseMockResponse(t *testing.T) {
	data, err := os.ReadFile(testdataPath("profilers", "mock_llm_response.json"))
	if err != nil {
		t.Fatalf("read mock response: %v", err)
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal Result: %v", err)
	}

	if len(result.SchemaChanges) != 1 {
		t.Fatalf("schema_changes count = %d, want 1", len(result.SchemaChanges))
	}
	if result.SchemaChanges[0].EntityType != "person" {
		t.Errorf("entity_type = %q, want %q", result.SchemaChanges[0].EntityType, "person")
	}
	if result.SchemaChanges[0].Field != "school" {
		t.Errorf("field = %q, want %q", result.SchemaChanges[0].Field, "school")
	}
	if result.SchemaChanges[0].Action != "add" {
		t.Errorf("action = %q, want %q", result.SchemaChanges[0].Action, "add")
	}

	updates, ok := result.ProfileUpdates["person:小明"]
	if !ok {
		t.Fatal("profile_updates missing person:小明")
	}
	if updates["school"] != "阳光幼儿园" {
		t.Errorf("school = %v, want %q", updates["school"], "阳光幼儿园")
	}

	if len(result.Relations) != 1 {
		t.Fatalf("relations count = %d, want 1", len(result.Relations))
	}
	if result.Relations[0].RelType != "attends" {
		t.Errorf("rel_type = %q, want %q", result.Relations[0].RelType, "attends")
	}
}

func TestGolden_ProfilerResult_ExpectedResult(t *testing.T) {
	data, err := os.ReadFile(testdataPath("profilers", "expected_result.json"))
	if err != nil {
		t.Fatalf("read expected result: %v", err)
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal Result: %v", err)
	}

	if len(result.SchemaChanges) != 1 {
		t.Errorf("schema_changes count = %d, want 1", len(result.SchemaChanges))
	}
	if len(result.ProfileUpdates) != 1 {
		t.Errorf("profile_updates count = %d, want 1", len(result.ProfileUpdates))
	}
	if len(result.Relations) != 1 {
		t.Errorf("relations count = %d, want 1", len(result.Relations))
	}
}

func TestGolden_ProfilerInput(t *testing.T) {
	data, err := os.ReadFile(testdataPath("profilers", "input.json"))
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
	if input.Extracted == nil {
		t.Fatal("extracted should not be nil")
	}
	if input.Extracted.Segment.Summary == "" {
		t.Error("extracted summary should not be empty")
	}
	if input.Schema == nil {
		t.Fatal("schema should not be nil")
	}
	if len(input.Profiles) != 1 {
		t.Errorf("profiles count = %d, want 1", len(input.Profiles))
	}
}

func TestGolden_ProfilerPrompt_HasKeySections(t *testing.T) {
	input := Input{
		Messages: []string{"user: hello", "assistant: hi"},
		Extracted: &segmentors.Result{
			Segment: segmentors.SegmentOutput{
				Summary: "Test conversation",
			},
		},
	}

	prompt := buildPrompt(input)
	sections := []string{
		"entity profile analyst",
		"## Extracted Metadata",
		"Test conversation",
		"## Output",
	}
	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt missing section: %q", section)
		}
	}
}

func TestGolden_ProfilerPrompt_WithSchemaAndProfiles(t *testing.T) {
	input := Input{
		Messages: []string{"user: hello"},
		Extracted: &segmentors.Result{
			Segment: segmentors.SegmentOutput{
				Summary: "Test",
			},
		},
		Schema: &segmentors.Schema{
			EntityTypes: map[string]segmentors.EntitySchema{
				"person": {
					Desc: "A person",
					Attrs: map[string]segmentors.AttrDef{
						"age": {Type: "string", Desc: "Age"},
					},
				},
			},
		},
		Profiles: map[string]map[string]any{
			"person:小明": {"age": "5"},
		},
	}

	prompt := buildPrompt(input)
	if !strings.Contains(prompt, "## Current Entity Schema") {
		t.Error("prompt should contain schema section")
	}
	if !strings.Contains(prompt, "## Existing Entity Profiles") {
		t.Error("prompt should contain profiles section")
	}
	if !strings.Contains(prompt, "person:小明") {
		t.Error("prompt should contain entity label")
	}
}
