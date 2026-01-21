package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"", ""},
		{"1234", "****"},
		{"12345678", "********"},
		{"123456789", "1234*6789"},
		{"abcdefghij", "abcd**ghij"},
		{"sk-1234567890abcdef", "sk-1***********cdef"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := MaskAPIKey(tt.key)
			if got != tt.want {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestContext_GetExtra_NilMap(t *testing.T) {
	ctx := &Context{
		Name:  "test",
		Extra: nil,
	}

	result := ctx.GetExtra("key")
	if result != "" {
		t.Errorf("GetExtra on nil map = %q, want empty string", result)
	}
}

func TestContext_GetExtra(t *testing.T) {
	ctx := &Context{
		Name: "test",
		Extra: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	if got := ctx.GetExtra("key1"); got != "value1" {
		t.Errorf("GetExtra(key1) = %q, want %q", got, "value1")
	}

	if got := ctx.GetExtra("key2"); got != "value2" {
		t.Errorf("GetExtra(key2) = %q, want %q", got, "value2")
	}

	if got := ctx.GetExtra("nonexistent"); got != "" {
		t.Errorf("GetExtra(nonexistent) = %q, want empty string", got)
	}
}

func TestContext_SetExtra_NilMap(t *testing.T) {
	ctx := &Context{
		Name:  "test",
		Extra: nil,
	}

	ctx.SetExtra("key", "value")

	if ctx.Extra == nil {
		t.Fatal("SetExtra should initialize Extra map")
	}

	if got := ctx.Extra["key"]; got != "value" {
		t.Errorf("Extra[key] = %q, want %q", got, "value")
	}
}

func TestContext_SetExtra(t *testing.T) {
	ctx := &Context{
		Name:  "test",
		Extra: make(map[string]string),
	}

	ctx.SetExtra("key1", "value1")
	ctx.SetExtra("key2", "value2")

	if got := ctx.Extra["key1"]; got != "value1" {
		t.Errorf("Extra[key1] = %q, want %q", got, "value1")
	}

	if got := ctx.Extra["key2"]; got != "value2" {
		t.Errorf("Extra[key2] = %q, want %q", got, "value2")
	}
}

func TestLoadConfigWithPath_NewConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "testapp", "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	if cfg.AppName != "testapp" {
		t.Errorf("AppName = %q, want %q", cfg.AppName, "testapp")
	}

	if cfg.Contexts == nil {
		t.Error("Contexts should be initialized")
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should be created")
	}
}

func TestConfig_AddContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	ctx := &Context{
		APIKey:  "test-key",
		BaseURL: "https://api.example.com",
	}

	err = cfg.AddContext("production", ctx)
	if err != nil {
		t.Fatalf("AddContext error: %v", err)
	}

	if cfg.Contexts["production"] == nil {
		t.Fatal("Context not added")
	}

	if cfg.Contexts["production"].Name != "production" {
		t.Errorf("Context.Name = %q, want %q", cfg.Contexts["production"].Name, "production")
	}

	if cfg.Contexts["production"].APIKey != "test-key" {
		t.Errorf("Context.APIKey = %q, want %q", cfg.Contexts["production"].APIKey, "test-key")
	}
}

func TestConfig_DeleteContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	cfg.AddContext("ctx1", &Context{APIKey: "key1"})
	cfg.AddContext("ctx2", &Context{APIKey: "key2"})
	cfg.UseContext("ctx1")

	// Delete non-current context
	err = cfg.DeleteContext("ctx2")
	if err != nil {
		t.Fatalf("DeleteContext error: %v", err)
	}

	if _, ok := cfg.Contexts["ctx2"]; ok {
		t.Error("Context should be deleted")
	}

	// Delete current context
	err = cfg.DeleteContext("ctx1")
	if err != nil {
		t.Fatalf("DeleteContext error: %v", err)
	}

	if cfg.CurrentContext != "" {
		t.Errorf("CurrentContext should be cleared, got %q", cfg.CurrentContext)
	}
}

func TestConfig_DeleteContext_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	err = cfg.DeleteContext("nonexistent")
	if err == nil {
		t.Error("DeleteContext should fail for non-existent context")
	}
}

func TestConfig_UseContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}
	cfg.AddContext("production", &Context{APIKey: "prod-key"})

	err = cfg.UseContext("production")
	if err != nil {
		t.Fatalf("UseContext error: %v", err)
	}

	if cfg.CurrentContext != "production" {
		t.Errorf("CurrentContext = %q, want %q", cfg.CurrentContext, "production")
	}
}

func TestConfig_UseContext_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	err = cfg.UseContext("nonexistent")
	if err == nil {
		t.Error("UseContext should fail for non-existent context")
	}
}

func TestConfig_GetContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}
	cfg.AddContext("test", &Context{APIKey: "test-key"})

	ctx, err := cfg.GetContext("test")
	if err != nil {
		t.Fatalf("GetContext error: %v", err)
	}

	if ctx.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", ctx.APIKey, "test-key")
	}
}

func TestConfig_GetContext_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	_, err = cfg.GetContext("nonexistent")
	if err == nil {
		t.Error("GetContext should fail for non-existent context")
	}
}

func TestConfig_GetCurrentContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}
	cfg.AddContext("default", &Context{APIKey: "default-key"})
	cfg.UseContext("default")

	ctx, err := cfg.GetCurrentContext()
	if err != nil {
		t.Fatalf("GetCurrentContext error: %v", err)
	}

	if ctx.APIKey != "default-key" {
		t.Errorf("APIKey = %q, want %q", ctx.APIKey, "default-key")
	}
}

func TestConfig_GetCurrentContext_NotSet(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	_, err = cfg.GetCurrentContext()
	if err == nil {
		t.Error("GetCurrentContext should fail when no current context")
	}
}

func TestConfig_ResolveContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}
	cfg.AddContext("ctx1", &Context{APIKey: "key1"})
	cfg.AddContext("ctx2", &Context{APIKey: "key2"})
	cfg.UseContext("ctx1")

	// Resolve by name
	ctx, err := cfg.ResolveContext("ctx2")
	if err != nil {
		t.Fatalf("ResolveContext(ctx2) error: %v", err)
	}
	if ctx.APIKey != "key2" {
		t.Errorf("APIKey = %q, want %q", ctx.APIKey, "key2")
	}

	// Resolve current (empty name)
	ctx, err = cfg.ResolveContext("")
	if err != nil {
		t.Fatalf("ResolveContext('') error: %v", err)
	}
	if ctx.APIKey != "key1" {
		t.Errorf("APIKey = %q, want %q", ctx.APIKey, "key1")
	}
}

func TestConfig_ListContexts(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}
	cfg.AddContext("production", &Context{})
	cfg.AddContext("staging", &Context{})
	cfg.AddContext("development", &Context{})

	names := cfg.ListContexts()

	if len(names) != 3 {
		t.Fatalf("len(names) = %d, want 3", len(names))
	}

	// Check all contexts are present
	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}

	for _, expected := range []string{"production", "staging", "development"} {
		if !found[expected] {
			t.Errorf("Missing context: %s", expected)
		}
	}
}

func TestConfig_Path(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	if cfg.Path() != configPath {
		t.Errorf("Path() = %q, want %q", cfg.Path(), configPath)
	}
}

func TestConfig_Dir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	if cfg.Dir() != tmpDir {
		t.Errorf("Dir() = %q, want %q", cfg.Dir(), tmpDir)
	}
}

func TestConfig_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create and save config
	cfg1, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}
	cfg1.AddContext("test", &Context{
		APIKey:  "secret-key",
		BaseURL: "https://api.test.com",
	})
	cfg1.UseContext("test")

	// Load again
	cfg2, err := LoadConfigWithPath("testapp", configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithPath error: %v", err)
	}

	if cfg2.CurrentContext != "test" {
		t.Errorf("CurrentContext = %q, want %q", cfg2.CurrentContext, "test")
	}

	ctx, err := cfg2.GetContext("test")
	if err != nil {
		t.Fatalf("GetContext error: %v", err)
	}
	if ctx.APIKey != "secret-key" {
		t.Errorf("APIKey = %q, want %q", ctx.APIKey, "secret-key")
	}
}
