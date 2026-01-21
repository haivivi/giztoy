package trie

import (
	"testing"
)

func TestTrie_SetValue_Get(t *testing.T) {
	tr := New[string]()

	// Set exact paths
	if err := tr.SetValue("a/b/c", "value1"); err != nil {
		t.Fatalf("SetValue error: %v", err)
	}
	if err := tr.SetValue("a/b/d", "value2"); err != nil {
		t.Fatalf("SetValue error: %v", err)
	}

	// Get exact paths
	if val, ok := tr.GetValue("a/b/c"); !ok || val != "value1" {
		t.Errorf("GetValue(a/b/c) = %v, %v; want value1, true", val, ok)
	}
	if val, ok := tr.GetValue("a/b/d"); !ok || val != "value2" {
		t.Errorf("GetValue(a/b/d) = %v, %v; want value2, true", val, ok)
	}

	// Non-existent path
	if _, ok := tr.GetValue("a/b/e"); ok {
		t.Error("GetValue(a/b/e) should return false")
	}
}

func TestTrie_SingleLevelWildcard(t *testing.T) {
	tr := New[string]()

	if err := tr.SetValue("device/+/state", "handler1"); err != nil {
		t.Fatalf("SetValue error: %v", err)
	}

	tests := []struct {
		path    string
		want    string
		wantOK  bool
	}{
		{"device/gear-001/state", "handler1", true},
		{"device/gear-002/state", "handler1", true},
		{"device/abc/state", "handler1", true},
		{"device/state", "", false},          // Missing middle level
		{"device/a/b/state", "", false},      // Too many levels
		{"other/gear-001/state", "", false},  // Wrong prefix
	}

	for _, tc := range tests {
		val, ok := tr.GetValue(tc.path)
		if ok != tc.wantOK {
			t.Errorf("GetValue(%q) ok = %v; want %v", tc.path, ok, tc.wantOK)
		}
		if ok && val != tc.want {
			t.Errorf("GetValue(%q) = %v; want %v", tc.path, val, tc.want)
		}
	}
}

func TestTrie_MultiLevelWildcard(t *testing.T) {
	tr := New[string]()

	if err := tr.SetValue("device/#", "catchall"); err != nil {
		t.Fatalf("SetValue error: %v", err)
	}

	tests := []struct {
		path   string
		wantOK bool
	}{
		{"device/gear-001", true},
		{"device/gear-001/state", true},
		{"device/gear-001/state/value", true},
		{"device/a/b/c/d/e", true},
		{"other/gear-001", false}, // Wrong prefix
	}

	for _, tc := range tests {
		_, ok := tr.GetValue(tc.path)
		if ok != tc.wantOK {
			t.Errorf("GetValue(%q) ok = %v; want %v", tc.path, ok, tc.wantOK)
		}
	}
}

func TestTrie_MultiLevelWildcard_MustBeLast(t *testing.T) {
	tr := New[string]()

	err := tr.SetValue("device/#/state", "invalid")
	if err != ErrInvalidPattern {
		t.Errorf("SetValue with # not at end: got %v, want ErrInvalidPattern", err)
	}
}

func TestTrie_CombinedWildcards(t *testing.T) {
	tr := New[string]()

	if err := tr.SetValue("device/+/events/#", "combined"); err != nil {
		t.Fatalf("SetValue error: %v", err)
	}

	tests := []struct {
		path   string
		wantOK bool
	}{
		{"device/gear-001/events/click", true},
		{"device/gear-002/events/touch/start", true},
		{"device/abc/events/a/b/c", true},
		{"device/gear-001/state", false},   // Wrong after +
		{"device/events/click", false},     // Missing + level
		{"device/a/b/events/click", false}, // Too many levels before events
	}

	for _, tc := range tests {
		_, ok := tr.GetValue(tc.path)
		if ok != tc.wantOK {
			t.Errorf("GetValue(%q) ok = %v; want %v", tc.path, ok, tc.wantOK)
		}
	}
}

func TestTrie_MatchPriority(t *testing.T) {
	tr := New[string]()

	// Register in different order - exact should take priority
	tr.SetValue("device/#", "catchall")
	tr.SetValue("device/+/state", "wildcard")
	tr.SetValue("device/gear-001/state", "exact")

	// Exact match should be returned first
	val, ok := tr.GetValue("device/gear-001/state")
	if !ok {
		t.Fatal("expected to match")
	}
	if val != "exact" {
		t.Errorf("GetValue = %q; want %q", val, "exact")
	}
}

func TestTrie_Match(t *testing.T) {
	tr := New[string]()

	tr.SetValue("device/+/state", "handler")

	route, val, ok := tr.Match("device/gear-001/state")
	if !ok {
		t.Fatal("expected to match")
	}
	if route != "/device/+/state" {
		t.Errorf("Match route = %q; want /device/+/state", route)
	}
	if *val != "handler" {
		t.Errorf("Match value = %q; want handler", *val)
	}
}

func TestTrie_EmptyPath(t *testing.T) {
	tr := New[string]()

	if err := tr.SetValue("", "root"); err != nil {
		t.Fatalf("SetValue error: %v", err)
	}

	val, ok := tr.GetValue("")
	if !ok {
		t.Error("expected to match empty path")
	}
	if val != "root" {
		t.Errorf("GetValue = %q; want root", val)
	}
}

func TestTrie_Set_WithCallback(t *testing.T) {
	tr := New[int]()

	// First set
	err := tr.Set("counter", func(ptr *int, existed bool) error {
		if existed {
			t.Error("should not exist on first set")
		}
		*ptr = 1
		return nil
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Update existing
	err = tr.Set("counter", func(ptr *int, existed bool) error {
		if !existed {
			t.Error("should exist on second set")
		}
		*ptr = *ptr + 1
		return nil
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	val, ok := tr.GetValue("counter")
	if !ok || val != 2 {
		t.Errorf("GetValue = %d, %v; want 2, true", val, ok)
	}
}

func TestTrie_Walk(t *testing.T) {
	tr := New[string]()

	tr.SetValue("a/b", "value1")
	tr.SetValue("a/c", "value2")
	tr.SetValue("d", "value3")

	count := 0
	tr.Walk(func(path string, value string, set bool) {
		if set {
			count++
		}
	})

	if count != 3 {
		t.Errorf("Walk counted %d set values; want 3", count)
	}
}

func TestTrie_Len(t *testing.T) {
	tr := New[string]()

	if tr.Len() != 0 {
		t.Errorf("empty trie Len = %d; want 0", tr.Len())
	}

	tr.SetValue("a", "1")
	tr.SetValue("b", "2")
	tr.SetValue("c/d", "3")

	if tr.Len() != 3 {
		t.Errorf("trie Len = %d; want 3", tr.Len())
	}
}

func TestTrie_String(t *testing.T) {
	tr := New[string]()

	tr.SetValue("a/b", "value1")
	tr.SetValue("a/+", "value2")
	tr.SetValue("a/#", "value3")

	str := tr.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
	t.Logf("Trie structure:\n%s", str)
}

func TestTrie_IntValues(t *testing.T) {
	tr := New[int]()

	tr.SetValue("route/1", 100)
	tr.SetValue("route/2", 200)
	tr.SetValue("route/+", 999)

	if val, ok := tr.GetValue("route/1"); !ok || val != 100 {
		t.Errorf("GetValue(route/1) = %d, %v; want 100, true", val, ok)
	}
	if val, ok := tr.GetValue("route/3"); !ok || val != 999 {
		t.Errorf("GetValue(route/3) = %d, %v; want 999, true", val, ok)
	}
}

func TestTrie_StructValues(t *testing.T) {
	type Handler struct {
		Name    string
		Handler func()
	}

	tr := New[Handler]()

	tr.SetValue("api/users", Handler{Name: "users"})
	tr.SetValue("api/+/profile", Handler{Name: "profile"})

	if val, ok := tr.GetValue("api/users"); !ok || val.Name != "users" {
		t.Errorf("GetValue(api/users) = %v; want {Name: users}", val)
	}
	if val, ok := tr.GetValue("api/123/profile"); !ok || val.Name != "profile" {
		t.Errorf("GetValue(api/123/profile) = %v; want {Name: profile}", val)
	}
}

func TestTrie_TrailingSlash(t *testing.T) {
	tr := New[string]()

	// Paths with trailing slash should work
	tr.SetValue("a/b/", "value1")

	// Should match without trailing slash
	val, ok := tr.GetValue("a/b")
	if !ok {
		t.Error("expected to match path without trailing slash")
	}
	if val != "value1" {
		t.Errorf("GetValue = %q; want value1", val)
	}
}

func TestTrie_DoubleSlash(t *testing.T) {
	tr := New[string]()

	// Double slash creates empty segment
	tr.SetValue("a//b", "value1")

	// Should match with empty segment
	val, ok := tr.GetValue("a//b")
	if !ok {
		t.Error("expected to match path with empty segment")
	}
	if val != "value1" {
		t.Errorf("GetValue = %q; want value1", val)
	}
}
