package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutput_JSON(t *testing.T) {
	var buf bytes.Buffer

	data := map[string]any{
		"name":  "test",
		"value": 123,
	}

	err := Output(data, OutputOptions{
		Format: FormatJSON,
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	// Verify valid JSON
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("name = %v, want %q", result["name"], "test")
	}
}

func TestOutput_YAML(t *testing.T) {
	var buf bytes.Buffer

	data := map[string]any{
		"name":  "test",
		"value": 123,
	}

	err := Output(data, OutputOptions{
		Format: FormatYAML,
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "name: test") {
		t.Errorf("Output should contain 'name: test', got: %s", output)
	}
}

func TestOutput_DefaultFormat(t *testing.T) {
	var buf bytes.Buffer

	data := map[string]string{"key": "value"}

	// Empty format should default to YAML
	err := Output(data, OutputOptions{
		Format: "",
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "key: value") {
		t.Errorf("Default format should be YAML, got: %s", output)
	}
}

func TestOutput_Raw_Bytes(t *testing.T) {
	var buf bytes.Buffer

	data := []byte("raw binary data")

	err := Output(data, OutputOptions{
		Format: FormatRaw,
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	if buf.String() != "raw binary data" {
		t.Errorf("Output = %q, want %q", buf.String(), "raw binary data")
	}
}

func TestOutput_Raw_String(t *testing.T) {
	var buf bytes.Buffer

	data := "raw string data"

	err := Output(data, OutputOptions{
		Format: FormatRaw,
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	if buf.String() != "raw string data" {
		t.Errorf("Output = %q, want %q", buf.String(), "raw string data")
	}
}

func TestOutput_Raw_Other(t *testing.T) {
	var buf bytes.Buffer

	// Non-string/bytes should fall back to YAML
	data := map[string]int{"count": 42}

	err := Output(data, OutputOptions{
		Format: FormatRaw,
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	if !strings.Contains(buf.String(), "count: 42") {
		t.Errorf("Output should contain YAML, got: %s", buf.String())
	}
}

func TestOutput_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer

	err := Output("data", OutputOptions{
		Format: "invalid",
		Writer: &buf,
	})
	if err == nil {
		t.Error("Output should fail for unsupported format")
	}
}

func TestOutput_ToFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "output.json")

	data := map[string]string{"key": "value"}

	err := Output(data, OutputOptions{
		Format: FormatJSON,
		File:   filePath,
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	// Read and verify file
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("Invalid JSON in file: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("key = %q, want %q", result["key"], "value")
	}
}

func TestOutput_JSONIndent(t *testing.T) {
	var buf bytes.Buffer

	data := map[string]string{"key": "value"}

	err := Output(data, OutputOptions{
		Format: FormatJSON,
		Writer: &buf,
		Indent: "    ", // 4 spaces
	})
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}

	// Should contain indentation
	if !strings.Contains(buf.String(), "    ") {
		t.Errorf("Output should be indented, got: %s", buf.String())
	}
}

func TestOutputBytes(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "data.bin")

	data := []byte{0x00, 0x01, 0x02, 0x03}

	err := OutputBytes(data, filePath)
	if err != nil {
		t.Fatalf("OutputBytes error: %v", err)
	}

	// Read and verify
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if !bytes.Equal(content, data) {
		t.Errorf("File content = %v, want %v", content, data)
	}
}

func TestOutputBytes_EmptyPath(t *testing.T) {
	err := OutputBytes([]byte{1, 2, 3}, "")
	if err == nil {
		t.Error("OutputBytes should fail for empty path")
	}
}

func TestOutputFormat_Constants(t *testing.T) {
	// Verify format constants
	if FormatYAML != "yaml" {
		t.Errorf("FormatYAML = %q, want %q", FormatYAML, "yaml")
	}

	if FormatJSON != "json" {
		t.Errorf("FormatJSON = %q, want %q", FormatJSON, "json")
	}

	if FormatTable != "table" {
		t.Errorf("FormatTable = %q, want %q", FormatTable, "table")
	}

	if FormatRaw != "raw" {
		t.Errorf("FormatRaw = %q, want %q", FormatRaw, "raw")
	}
}
