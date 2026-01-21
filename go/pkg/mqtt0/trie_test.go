package mqtt0

import (
	"testing"
)

func TestTrieExactMatch(t *testing.T) {
	trie := NewTrie[string]()

	if err := trie.Insert("device/gear-001/state", "handler1"); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Should match exact topic
	values := trie.Get("device/gear-001/state")
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	if values[0] != "handler1" {
		t.Errorf("expected handler1, got %s", values[0])
	}

	// Should not match different topic
	values = trie.Get("device/gear-002/state")
	if len(values) != 0 {
		t.Errorf("expected no match for different topic, got %d", len(values))
	}

	// Should not match partial topic
	values = trie.Get("device/gear-001")
	if len(values) != 0 {
		t.Errorf("expected no match for partial topic, got %d", len(values))
	}
}

func TestTrieSingleLevelWildcard(t *testing.T) {
	trie := NewTrie[string]()

	if err := trie.Insert("device/+/state", "wildcard"); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Should match any single level
	tests := []string{
		"device/gear-001/state",
		"device/gear-002/state",
		"device/abc/state",
	}

	for _, topic := range tests {
		values := trie.Get(topic)
		if len(values) != 1 {
			t.Errorf("expected match for %q, got %d values", topic, len(values))
		}
	}

	// Should not match
	noMatch := []string{
		"device/state",       // Missing middle level
		"device/a/b/state",   // Too many levels
		"other/gear-001/state", // Wrong prefix
	}

	for _, topic := range noMatch {
		values := trie.Get(topic)
		if len(values) != 0 {
			t.Errorf("expected no match for %q, got %d values", topic, len(values))
		}
	}
}

func TestTrieMultiLevelWildcard(t *testing.T) {
	trie := NewTrie[string]()

	if err := trie.Insert("device/#", "multi"); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Should match any number of levels after device/
	tests := []string{
		"device/gear-001",
		"device/gear-001/state",
		"device/gear-001/state/value",
		"device/a/b/c/d",
	}

	for _, topic := range tests {
		values := trie.Get(topic)
		if len(values) != 1 {
			t.Errorf("expected match for %q, got %d values", topic, len(values))
		}
	}

	// Should not match
	values := trie.Get("other/gear-001")
	if len(values) != 0 {
		t.Errorf("expected no match for wrong prefix, got %d values", len(values))
	}
}

func TestTrieMultiLevelWildcardMustBeLast(t *testing.T) {
	trie := NewTrie[string]()

	err := trie.Insert("device/#/state", "invalid")
	if err != ErrInvalidTopic {
		t.Errorf("expected ErrInvalidTopic, got %v", err)
	}
}

func TestTrieRemove(t *testing.T) {
	trie := NewTrie[string]()

	trie.Insert("device/+/state", "handler1")
	trie.Insert("device/+/state", "handler2")

	values := trie.Get("device/gear-001/state")
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}

	// Remove handler1
	trie.Remove("device/+/state", func(v string) bool {
		return v == "handler1"
	})

	values = trie.Get("device/gear-001/state")
	if len(values) != 1 {
		t.Fatalf("expected 1 value after remove, got %d", len(values))
	}
	if values[0] != "handler2" {
		t.Errorf("expected handler2, got %s", values[0])
	}
}

func TestTrieMatch(t *testing.T) {
	trie := NewTrie[string]()

	trie.Insert("sensor/+/temp", "temp-handler")
	trie.Insert("sensor/#", "all-handler")

	// Test + wildcard match
	pattern, values, ok := trie.Match("sensor/room1/temp")
	if !ok {
		t.Fatal("expected match")
	}
	if pattern != "sensor/+/temp" {
		t.Errorf("expected pattern sensor/+/temp, got %s", pattern)
	}
	if len(values) != 1 || values[0] != "temp-handler" {
		t.Errorf("unexpected values: %v", values)
	}

	// Test # wildcard match (when + doesn't match)
	pattern, values, ok = trie.Match("sensor/room1/humidity")
	if !ok {
		t.Fatal("expected match")
	}
	if pattern != "sensor/#" {
		t.Errorf("expected pattern sensor/#, got %s", pattern)
	}
	if len(values) != 1 || values[0] != "all-handler" {
		t.Errorf("unexpected values: %v", values)
	}
}

func TestTrieConcurrency(t *testing.T) {
	trie := NewTrie[int]()

	// Concurrent inserts
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			trie.Insert("topic/"+string(rune('a'+n%26)), n)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		go func(n int) {
			trie.Get("topic/" + string(rune('a'+n%26)))
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func BenchmarkTrieInsert(b *testing.B) {
	trie := NewTrie[string]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Insert("device/gear-001/state", "handler")
	}
}

func BenchmarkTrieGet(b *testing.B) {
	trie := NewTrie[string]()
	trie.Insert("device/+/state", "handler")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Get("device/gear-001/state")
	}
}

func BenchmarkTrieGetMultiLevel(b *testing.B) {
	trie := NewTrie[string]()
	trie.Insert("device/#", "handler")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Get("device/gear-001/state/value")
	}
}
