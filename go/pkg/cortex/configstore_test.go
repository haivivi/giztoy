package cortex

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *ConfigStore {
	t.Helper()
	s, err := OpenConfigStoreAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// ---------------------------------------------------------------------------
// Ctx tests
// ---------------------------------------------------------------------------

func TestCtxAdd(t *testing.T) {
	s := newTestStore(t)
	if err := s.CtxAdd("dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.ctxDir("dev")); err != nil {
		t.Fatal("context dir not created")
	}
}

func TestCtxAddDuplicate(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	if err := s.CtxAdd("dev"); err == nil {
		t.Fatal("expected error for duplicate context")
	}
}

func TestCtxAddEmptyName(t *testing.T) {
	s := newTestStore(t)
	if err := s.CtxAdd(""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCtxListEmpty(t *testing.T) {
	s := newTestStore(t)
	infos, err := s.CtxList()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected 0 contexts, got %d", len(infos))
	}
}

func TestCtxListMultiple(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	s.CtxAdd("prod")
	infos, err := s.CtxList()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(infos))
	}
}

func TestCtxUse(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	if err := s.CtxUse("dev"); err != nil {
		t.Fatal(err)
	}
	cur, err := s.CtxCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if cur != "dev" {
		t.Fatalf("expected current=dev, got %q", cur)
	}
}

func TestCtxUseNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.CtxUse("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent context")
	}
}

func TestCtxCurrentUnset(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CtxCurrent()
	if err == nil {
		t.Fatal("expected error when no context set")
	}
}

func TestCtxListShowsCurrent(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	s.CtxAdd("prod")
	s.CtxUse("dev")
	infos, _ := s.CtxList()
	found := false
	for _, info := range infos {
		if info.Name == "dev" && info.Current {
			found = true
		}
		if info.Name == "prod" && info.Current {
			t.Fatal("prod should not be current")
		}
	}
	if !found {
		t.Fatal("dev should be marked as current")
	}
}

func TestCtxShow(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	s.CtxUse("dev")
	s.CtxConfigSet("kv", "badger:///tmp/test")
	name, cfg, err := s.CtxShow("")
	if err != nil {
		t.Fatal(err)
	}
	if name != "dev" {
		t.Fatalf("expected name=dev, got %q", name)
	}
	if cfg.KV != "badger:///tmp/test" {
		t.Fatalf("expected kv=badger:///tmp/test, got %q", cfg.KV)
	}
}

func TestCtxShowNamed(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	s.CtxAdd("prod")
	s.CtxUse("dev")
	s.CtxConfigSet("kv", "badger:///dev")

	// Show prod (empty config)
	name, cfg, err := s.CtxShow("prod")
	if err != nil {
		t.Fatal(err)
	}
	if name != "prod" {
		t.Fatalf("expected name=prod, got %q", name)
	}
	if cfg.KV != "" {
		t.Fatalf("expected empty kv for prod, got %q", cfg.KV)
	}
}

func TestCtxRemove(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("staging")
	if err := s.CtxRemove("staging"); err != nil {
		t.Fatal(err)
	}
	infos, _ := s.CtxList()
	if len(infos) != 0 {
		t.Fatal("expected 0 contexts after remove")
	}
}

func TestCtxRemoveCurrentFails(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	s.CtxUse("dev")
	if err := s.CtxRemove("dev"); err == nil {
		t.Fatal("expected error when removing current context")
	}
}

func TestCtxConfigSetAll(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	s.CtxUse("dev")

	for _, kv := range [][2]string{
		{"kv", "badger:///data"},
		{"storage", "local:///files"},
		{"vecstore", "hnsw:///idx"},
		{"embed", "dashscope://sk-xxx"},
	} {
		if err := s.CtxConfigSet(kv[0], kv[1]); err != nil {
			t.Fatalf("set %s: %v", kv[0], err)
		}
	}

	_, cfg, err := s.CtxShow("dev")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KV != "badger:///data" {
		t.Fatalf("kv mismatch: %q", cfg.KV)
	}
	if cfg.Storage != "local:///files" {
		t.Fatalf("storage mismatch: %q", cfg.Storage)
	}
	if cfg.VecStore != "hnsw:///idx" {
		t.Fatalf("vecstore mismatch: %q", cfg.VecStore)
	}
	if cfg.Embed != "dashscope://sk-xxx" {
		t.Fatalf("embed mismatch: %q", cfg.Embed)
	}
}

func TestCtxConfigSetUnknownKey(t *testing.T) {
	s := newTestStore(t)
	s.CtxAdd("dev")
	s.CtxUse("dev")
	if err := s.CtxConfigSet("foo", "bar"); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestCtxConfigList(t *testing.T) {
	s := newTestStore(t)
	keys := s.CtxConfigList()
	if len(keys) != 4 {
		t.Fatalf("expected 4 config keys, got %d", len(keys))
	}
}

// ---------------------------------------------------------------------------
// App tests
// ---------------------------------------------------------------------------

func setupCtx(t *testing.T, s *ConfigStore) {
	t.Helper()
	s.CtxAdd("dev")
	s.CtxUse("dev")
}

func TestAppAdd(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	err := s.AppAdd("minimax", "cn", map[string]any{
		"api_key":  "test-key",
		"base_url": "https://api.minimaxi.com",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppAddDuplicate(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	s.AppAdd("minimax", "cn", map[string]any{"api_key": "k"})
	if err := s.AppAdd("minimax", "cn", map[string]any{"api_key": "k2"}); err == nil {
		t.Fatal("expected error for duplicate app")
	}
}

func TestAppListEmpty(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	infos, err := s.AppList("minimax")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(infos))
	}
}

func TestAppListMultiple(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	s.AppAdd("minimax", "cn", map[string]any{"api_key": "k1"})
	s.AppAdd("minimax", "global", map[string]any{"api_key": "k2"})
	infos, err := s.AppList("minimax")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(infos))
	}
}

func TestAppUse(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	s.AppAdd("minimax", "cn", map[string]any{"api_key": "k"})
	if err := s.AppUse("minimax", "cn"); err != nil {
		t.Fatal(err)
	}
	cur, err := s.AppCurrent("minimax")
	if err != nil {
		t.Fatal(err)
	}
	if cur != "cn" {
		t.Fatalf("expected current=cn, got %q", cur)
	}
}

func TestAppUseNotFound(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	if err := s.AppUse("minimax", "nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAppShow(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	s.AppAdd("minimax", "cn", map[string]any{
		"api_key":  "test-key",
		"base_url": "https://api.minimaxi.com",
	})
	cfg, err := s.AppShow("minimax", "cn")
	if err != nil {
		t.Fatal(err)
	}
	if cfg["api_key"] != "test-key" {
		t.Fatalf("api_key mismatch: %v", cfg["api_key"])
	}
	if cfg["base_url"] != "https://api.minimaxi.com" {
		t.Fatalf("base_url mismatch: %v", cfg["base_url"])
	}
}

func TestAppRemove(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	s.AppAdd("minimax", "cn", map[string]any{"api_key": "k"})
	s.AppAdd("minimax", "global", map[string]any{"api_key": "k2"})
	if err := s.AppRemove("minimax", "global"); err != nil {
		t.Fatal(err)
	}
	infos, _ := s.AppList("minimax")
	if len(infos) != 1 {
		t.Fatalf("expected 1 app after remove, got %d", len(infos))
	}
}

func TestAppRemoveClearsCurrent(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	s.AppAdd("minimax", "cn", map[string]any{"api_key": "k"})
	s.AppUse("minimax", "cn")
	s.AppRemove("minimax", "cn")
	_, err := s.AppCurrent("minimax")
	if err == nil {
		t.Fatal("expected error after removing current app")
	}
}

func TestAppLoad(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	s.AppAdd("minimax", "cn", map[string]any{
		"api_key":  "test-key",
		"base_url": "https://api.minimaxi.com",
	})
	type MiniMaxCfg struct {
		APIKey  string `yaml:"api_key"`
		BaseURL string `yaml:"base_url"`
	}
	cfg, err := AppLoad[MiniMaxCfg](s, "minimax", "cn")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "test-key" {
		t.Fatalf("api_key mismatch: %q", cfg.APIKey)
	}
}

// ---------------------------------------------------------------------------
// GenX tests
// ---------------------------------------------------------------------------

func writeGenXTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenXAdd(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)

	tmpDir := t.TempDir()
	path := writeGenXTestFile(t, tmpDir, "generator-test.json", `{
		"schema": "openai/chat/v1",
		"type": "generator",
		"api_key": "$TEST_KEY",
		"models": [{"name": "test/model", "model": "gpt-4"}]
	}`)

	if err := s.GenXAdd(path); err != nil {
		t.Fatal(err)
	}

	infos, err := s.GenXList("")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(infos))
	}
	if infos[0].Pattern != "test/model" {
		t.Fatalf("pattern mismatch: %q", infos[0].Pattern)
	}
	if infos[0].Type != "generator" {
		t.Fatalf("type mismatch: %q", infos[0].Type)
	}
}

func TestGenXAddTTS(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)

	tmpDir := t.TempDir()
	path := writeGenXTestFile(t, tmpDir, "tts-test.json", `{
		"schema": "minimax/speech/v1",
		"type": "tts",
		"api_key": "$TEST_KEY",
		"voices": [
			{"name": "test/voice1", "voice_id": "female-test"},
			{"name": "test/voice2", "voice_id": "male-test"}
		]
	}`)

	if err := s.GenXAdd(path); err != nil {
		t.Fatal(err)
	}

	infos, err := s.GenXList("")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(infos))
	}
}

func TestGenXListByType(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)

	tmpDir := t.TempDir()
	writeGenXTestFile(t, tmpDir, "gen.json", `{
		"type": "generator", "schema": "openai/chat/v1",
		"models": [{"name": "g/1", "model": "m"}]
	}`)
	writeGenXTestFile(t, tmpDir, "tts.json", `{
		"type": "tts", "schema": "minimax/speech/v1",
		"voices": [{"name": "t/1", "voice_id": "v"}]
	}`)
	s.GenXAdd(filepath.Join(tmpDir, "gen.json"))
	s.GenXAdd(filepath.Join(tmpDir, "tts.json"))

	gens, _ := s.GenXList("generator")
	if len(gens) != 1 || gens[0].Pattern != "g/1" {
		t.Fatalf("generator filter failed: %v", gens)
	}

	ttsList, _ := s.GenXList("tts")
	if len(ttsList) != 1 || ttsList[0].Pattern != "t/1" {
		t.Fatalf("tts filter failed: %v", ttsList)
	}
}

func TestGenXListEmpty(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	infos, err := s.GenXList("")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(infos))
	}
}

func TestGenXRemove(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)

	tmpDir := t.TempDir()
	path := writeGenXTestFile(t, tmpDir, "gen.json", `{
		"type": "generator", "schema": "openai/chat/v1",
		"models": [{"name": "test/rm", "model": "m"}]
	}`)
	s.GenXAdd(path)

	if err := s.GenXRemove("test/rm"); err != nil {
		t.Fatal(err)
	}

	infos, _ := s.GenXList("")
	if len(infos) != 0 {
		t.Fatalf("expected 0 after remove, got %d", len(infos))
	}
}

func TestGenXRemoveNotFound(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	if err := s.GenXRemove("nonexistent/pattern"); err == nil {
		t.Fatal("expected error for nonexistent pattern")
	}
}

func TestGenXAddNotFoundFile(t *testing.T) {
	s := newTestStore(t)
	setupCtx(t, s)
	if err := s.GenXAdd("/nonexistent/file.json"); err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
