package modelloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandEnv(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_API_KEY", "test-key-123")
	defer os.Unsetenv("TEST_API_KEY")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"plain value", "plain-api-key", "plain-api-key"},
		{"env var with $", "$TEST_API_KEY", "test-key-123"},
		{"env var with ${}", "${TEST_API_KEY}", "test-key-123"},
		{"unset env var", "$UNSET_VAR", ""},
		{"mixed content", "prefix-$TEST_API_KEY-suffix", "prefix-$TEST_API_KEY-suffix"}, // Only expands if starts with $
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnv(tt.input)
			if result != tt.expected {
				t.Errorf("expandEnv(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseConfig_JSON(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Write test JSON config
	jsonContent := `{
		"schema": "openai/chat/v1",
		"type": "generator",
		"api_key": "test-key",
		"base_url": "https://api.example.com",
		"models": [
			{
				"name": "test/model",
				"model": "gpt-4"
			}
		]
	}`
	jsonPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig(jsonPath)
	if err != nil {
		t.Fatalf("parseConfig failed: %v", err)
	}

	if cfg.Schema != "openai/chat/v1" {
		t.Errorf("Schema = %q, want %q", cfg.Schema, "openai/chat/v1")
	}
	if cfg.Type != "generator" {
		t.Errorf("Type = %q, want %q", cfg.Type, "generator")
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key")
	}
	if len(cfg.Models) != 1 {
		t.Errorf("len(Models) = %d, want 1", len(cfg.Models))
	}
	if cfg.Models[0].Name != "test/model" {
		t.Errorf("Models[0].Name = %q, want %q", cfg.Models[0].Name, "test/model")
	}
}

func TestParseConfig_YAML(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
schema: doubao/seed_tts/v2
type: tts
app_id: test-app-id
token: test-token
voices:
  - name: doubao/voice1
    voice_id: zh_female_test
    desc: Test Voice
`
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig(yamlPath)
	if err != nil {
		t.Fatalf("parseConfig failed: %v", err)
	}

	if cfg.Schema != "doubao/seed_tts/v2" {
		t.Errorf("Schema = %q, want %q", cfg.Schema, "doubao/seed_tts/v2")
	}
	if cfg.Type != "tts" {
		t.Errorf("Type = %q, want %q", cfg.Type, "tts")
	}
	if cfg.AppID != "test-app-id" {
		t.Errorf("AppID = %q, want %q", cfg.AppID, "test-app-id")
	}
	if len(cfg.Voices) != 1 {
		t.Errorf("len(Voices) = %d, want 1", len(cfg.Voices))
	}
}

func TestParseConfig_UnsupportedExtension(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "config.txt")
	if err := os.WriteFile(txtPath, []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := parseConfig(txtPath)
	if err == nil {
		t.Error("expected error for unsupported extension")
	}
}

func TestRegisterConfig_LegacyKind(t *testing.T) {
	// Test that legacy format with missing API key returns error
	cfg := ConfigFile{
		Kind: "openai",
		// No APIKey set
	}

	_, err := registerConfig(cfg)
	if err == nil {
		t.Error("expected error for missing api_key")
	}
}

func TestRegisterConfig_SchemaType(t *testing.T) {
	// Test unknown type returns error
	cfg := ConfigFile{
		Schema: "test/schema/v1",
		Type:   "unknown_type",
	}

	_, err := registerConfig(cfg)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestRegisterConfig_InvalidSchema(t *testing.T) {
	cfg := ConfigFile{
		Schema: "invalid", // Missing parts
		Type:   "generator",
	}

	_, err := registerConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid schema")
	}
}

func TestLoadFromDir_SkipsMissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config with env var that's not set
	jsonContent := `{
		"schema": "openai/chat/v1",
		"type": "generator",
		"api_key": "$NONEXISTENT_API_KEY",
		"models": [{"name": "test/model", "model": "gpt-4"}]
	}`
	jsonPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Should not error, but should return empty names (config skipped)
	names, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 names (skipped), got %d", len(names))
	}
}

func TestLoadFromDir_IgnoresNonConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create non-config files
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# README"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "script.sh"), []byte("#!/bin/bash"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should not error
	names, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

func TestLoadFromDir_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	names, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

func TestVoiceEntry(t *testing.T) {
	v := VoiceEntry{
		Name:    "test/voice",
		VoiceID: "zh_female_test",
		Desc:    "Test description",
		Cluster: "test-cluster",
	}

	if v.Name != "test/voice" {
		t.Errorf("Name = %q, want %q", v.Name, "test/voice")
	}
	if v.VoiceID != "zh_female_test" {
		t.Errorf("VoiceID = %q, want %q", v.VoiceID, "zh_female_test")
	}
}

func TestEntry(t *testing.T) {
	e := Entry{
		Name:              "test/model",
		Model:             "gpt-4",
		SupportJSONOutput: true,
		SupportToolCalls:  true,
		SupportTextOnly:   false,
		UseSystemRole:     true,
	}

	if e.Name != "test/model" {
		t.Errorf("Name = %q, want %q", e.Name, "test/model")
	}
	if !e.SupportJSONOutput {
		t.Error("SupportJSONOutput should be true")
	}
	if !e.SupportToolCalls {
		t.Error("SupportToolCalls should be true")
	}
}
