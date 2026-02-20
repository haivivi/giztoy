package cortex

import (
	"os"
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
