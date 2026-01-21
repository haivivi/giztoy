package mqtt

import (
	"testing"
)

// mockHandler is a simple handler for testing
type mockHandler struct {
	name string
}

func (h *mockHandler) HandleMessage(msg Message) error {
	return nil
}

func TestTrie_ExactMatch(t *testing.T) {
	tr := &trie{}

	handler := &mockHandler{name: "exact"}
	err := tr.Set("device/gear-001/state", func(node *trie) {
		node.handlers = append(node.handlers, handler)
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Should match exact topic
	handlers, ok := tr.Get("device/gear-001/state")
	if !ok {
		t.Error("expected to match exact topic")
	}
	if len(handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(handlers))
	}

	// Should not match different topic
	_, ok = tr.Get("device/gear-002/state")
	if ok {
		t.Error("should not match different topic")
	}

	// Should not match partial topic
	_, ok = tr.Get("device/gear-001")
	if ok {
		t.Error("should not match partial topic")
	}
}

func TestTrie_SingleLevelWildcard(t *testing.T) {
	tr := &trie{}

	handler := &mockHandler{name: "single-wildcard"}
	err := tr.Set("device/+/state", func(node *trie) {
		node.handlers = append(node.handlers, handler)
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Should match any single level
	testCases := []struct {
		topic   string
		matches bool
	}{
		{"device/gear-001/state", true},
		{"device/gear-002/state", true},
		{"device/abc/state", true},
		{"device/state", false},         // Missing middle level
		{"device/a/b/state", false},     // Too many levels
		{"other/gear-001/state", false}, // Wrong prefix
	}

	for _, tc := range testCases {
		_, ok := tr.Get(tc.topic)
		if ok != tc.matches {
			t.Errorf("topic %q: expected matches=%v, got %v", tc.topic, tc.matches, ok)
		}
	}
}

func TestTrie_MultiLevelWildcard(t *testing.T) {
	tr := &trie{}

	handler := &mockHandler{name: "multi-wildcard"}
	err := tr.Set("device/#", func(node *trie) {
		node.handlers = append(node.handlers, handler)
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Should match any number of levels after device/
	testCases := []struct {
		topic   string
		matches bool
	}{
		{"device/gear-001", true},
		{"device/gear-001/state", true},
		{"device/gear-001/state/value", true},
		{"device/a/b/c/d/e", true},
		{"other/gear-001", false}, // Wrong prefix
	}

	for _, tc := range testCases {
		_, ok := tr.Get(tc.topic)
		if ok != tc.matches {
			t.Errorf("topic %q: expected matches=%v, got %v", tc.topic, tc.matches, ok)
		}
	}
}

func TestTrie_MultiLevelWildcardMustBeLast(t *testing.T) {
	tr := &trie{}

	// # must be the last segment
	err := tr.Set("device/#/state", func(node *trie) {
		node.handlers = append(node.handlers, &mockHandler{})
	})
	if err != ErrInvalidTopicPattern {
		t.Errorf("expected ErrInvalidTopicPattern, got %v", err)
	}
}

func TestTrie_CombinedWildcards(t *testing.T) {
	tr := &trie{}

	// Pattern: device/+/events/#
	handler := &mockHandler{name: "combined"}
	err := tr.Set("device/+/events/#", func(node *trie) {
		node.handlers = append(node.handlers, handler)
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	testCases := []struct {
		topic   string
		matches bool
	}{
		{"device/gear-001/events/click", true},
		{"device/gear-002/events/touch/start", true},
		{"device/abc/events/a/b/c", true},
		{"device/gear-001/state", false},   // Wrong after +
		{"device/events/click", false},     // Missing + level
		{"device/a/b/events/click", false}, // Too many levels before events
	}

	for _, tc := range testCases {
		_, ok := tr.Get(tc.topic)
		if ok != tc.matches {
			t.Errorf("topic %q: expected matches=%v, got %v", tc.topic, tc.matches, ok)
		}
	}
}

func TestTrie_MultiplePatterns(t *testing.T) {
	tr := &trie{}

	// Register multiple patterns
	patterns := []string{
		"device/+/state",
		"device/+/stats",
		"device/+/input_audio_stream",
		"server/push/#",
	}

	for _, p := range patterns {
		pattern := p
		err := tr.Set(pattern, func(node *trie) {
			node.handlers = append(node.handlers, &mockHandler{name: pattern})
		})
		if err != nil {
			t.Fatalf("Set %q error: %v", pattern, err)
		}
	}

	testCases := []struct {
		topic   string
		matches bool
	}{
		{"device/gear-001/state", true},
		{"device/gear-001/stats", true},
		{"device/gear-001/input_audio_stream", true},
		{"server/push/vd/mode_changed", true},
		{"server/push/user/command", true},
		{"device/gear-001/unknown", false},
		{"other/topic", false},
	}

	for _, tc := range testCases {
		_, ok := tr.Get(tc.topic)
		if ok != tc.matches {
			t.Errorf("topic %q: expected matches=%v, got %v", tc.topic, tc.matches, ok)
		}
	}
}

func TestTrie_ShareSubscription(t *testing.T) {
	tr := &trie{}

	handler := &mockHandler{name: "shared"}
	err := tr.Set("$share/group1/device/+/state", func(node *trie) {
		node.handlers = append(node.handlers, handler)
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Shared subscription should match the underlying topic pattern
	testCases := []struct {
		topic   string
		matches bool
	}{
		{"device/gear-001/state", true},
		{"device/gear-002/state", true},
		{"device/gear-001/stats", false},
	}

	for _, tc := range testCases {
		_, ok := tr.Get(tc.topic)
		if ok != tc.matches {
			t.Errorf("topic %q: expected matches=%v, got %v", tc.topic, tc.matches, ok)
		}
	}
}

func TestTrie_InvalidShareSubscription(t *testing.T) {
	tr := &trie{}

	// $share requires at least group name and topic
	err := tr.Set("$share/group1", func(node *trie) {
		node.handlers = append(node.handlers, &mockHandler{})
	})
	if err != ErrInvalidShareSubscription {
		t.Errorf("expected ErrInvalidShareSubscription, got %v", err)
	}
}

func TestTrie_QueueSubscription(t *testing.T) {
	tr := &trie{}

	handler := &mockHandler{name: "queue"}
	err := tr.Set("$queue/device/+/state", func(node *trie) {
		node.handlers = append(node.handlers, handler)
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Queue subscription should match the underlying topic pattern
	_, ok := tr.Get("device/gear-001/state")
	if !ok {
		t.Error("expected to match queue subscription topic")
	}
}

func TestTrie_EmptyPattern(t *testing.T) {
	tr := &trie{}

	handler := &mockHandler{name: "root"}
	err := tr.Set("", func(node *trie) {
		node.handlers = append(node.handlers, handler)
	})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Empty topic should match empty pattern
	handlers, ok := tr.Get("")
	if !ok {
		t.Error("expected to match empty topic")
	}
	if len(handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(handlers))
	}
}

func TestTrie_MatchPriority(t *testing.T) {
	tr := &trie{}

	// Register patterns in different order
	// Exact match should take priority over wildcards
	patterns := []string{
		"device/#",
		"device/+/state",
		"device/gear-001/state",
	}

	for _, p := range patterns {
		pattern := p
		err := tr.Set(pattern, func(node *trie) {
			node.handlers = append(node.handlers, &mockHandler{name: pattern})
		})
		if err != nil {
			t.Fatalf("Set %q error: %v", pattern, err)
		}
	}

	// Exact match should be returned first (based on trie traversal order)
	handlers, ok := tr.Get("device/gear-001/state")
	if !ok {
		t.Fatal("expected to match")
	}

	// Should find the exact match first
	if len(handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(handlers))
	}
	if h, ok := handlers[0].(*mockHandler); ok {
		if h.name != "device/gear-001/state" {
			t.Errorf("expected exact match handler, got %s", h.name)
		}
	}
}

func TestTrie_MultipleHandlersPerPattern(t *testing.T) {
	tr := &trie{}

	// Add multiple handlers to the same pattern
	for i := 0; i < 3; i++ {
		err := tr.Set("device/+/state", func(node *trie) {
			node.handlers = append(node.handlers, &mockHandler{name: "handler"})
		})
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}
	}

	handlers, ok := tr.Get("device/gear-001/state")
	if !ok {
		t.Fatal("expected to match")
	}
	if len(handlers) != 3 {
		t.Errorf("expected 3 handlers, got %d", len(handlers))
	}
}

func TestTrie_String(t *testing.T) {
	tr := &trie{}

	patterns := []string{
		"device/+/state",
		"device/+/stats",
		"server/#",
	}

	for _, p := range patterns {
		pattern := p
		err := tr.Set(pattern, func(node *trie) {
			node.handlers = append(node.handlers, &mockHandler{name: pattern})
		})
		if err != nil {
			t.Fatalf("Set %q error: %v", pattern, err)
		}
	}

	str := tr.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
	t.Logf("Trie structure:\n%s", str)
}
