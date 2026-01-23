package mqtt0

import (
	"strings"
	"sync"
)

// Trie is a thread-safe trie data structure for MQTT topic pattern matching.
// It supports MQTT wildcards:
//   - `+` matches exactly one topic level
//   - `#` matches any number of remaining topic levels (must be last)
type Trie[T any] struct {
	mu   sync.RWMutex
	root *trieNode[T]
}

type trieNode[T any] struct {
	children map[string]*trieNode[T]
	matchAny *trieNode[T] // + wildcard
	matchAll *trieNode[T] // # wildcard
	values   []T
}

// NewTrie creates a new Trie.
func NewTrie[T any]() *Trie[T] {
	return &Trie[T]{
		root: &trieNode[T]{},
	}
}

// Insert adds a value at the given pattern.
func (t *Trie[T]) Insert(pattern string, value T) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.root.insert(pattern, value)
}

// Get returns all values matching the given topic.
func (t *Trie[T]) Get(topic string) []T {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.root.get(topic)
}

// Match returns the matched pattern and values for the given topic.
func (t *Trie[T]) Match(topic string) (pattern string, values []T, ok bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.root.match("", topic)
}

// Remove removes values matching the predicate from the given pattern.
// Returns true if any value was removed.
func (t *Trie[T]) Remove(pattern string, predicate func(T) bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.root.remove(pattern, predicate)
}

func (n *trieNode[T]) insert(pattern string, value T) error {
	if pattern == "" {
		n.values = append(n.values, value)
		return nil
	}

	var first, rest string
	idx := strings.Index(pattern, "/")
	if idx == -1 {
		first = pattern
	} else {
		first = pattern[:idx]
		rest = pattern[idx+1:]

		// Handle special prefixes
		switch first {
		case "$share":
			// $share/<sharename>/<topic>
			parts := strings.SplitN(rest, "/", 3)
			if len(parts) < 2 {
				return ErrInvalidTopic
			}
			first = parts[1]
			if len(parts) == 3 {
				rest = parts[2]
			} else {
				rest = ""
			}
		case "$queue":
			// $queue/<topic>
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) < 1 {
				return ErrInvalidTopic
			}
			first = parts[0]
			if len(parts) == 2 {
				rest = parts[1]
			} else {
				rest = ""
			}
		}
	}

	// Check existing children first
	if n.children != nil {
		if child, ok := n.children[first]; ok {
			return child.insert(rest, value)
		}
	}

	switch first {
	case "+":
		if n.matchAny == nil {
			n.matchAny = &trieNode[T]{}
		}
		return n.matchAny.insert(rest, value)
	case "#":
		if rest != "" {
			return ErrInvalidTopic
		}
		if n.matchAll == nil {
			n.matchAll = &trieNode[T]{}
		}
		n.matchAll.values = append(n.matchAll.values, value)
		return nil
	default:
		if n.children == nil {
			n.children = make(map[string]*trieNode[T])
		}
		child := &trieNode[T]{}
		n.children[first] = child
		return child.insert(rest, value)
	}
}

func (n *trieNode[T]) get(topic string) []T {
	_, values, _ := n.match("", topic)
	return values
}

func (n *trieNode[T]) match(matched, topic string) (string, []T, bool) {
	if topic == "" {
		if len(n.values) > 0 {
			return matched, n.values, true
		}
		return "", nil, false
	}

	var first, rest string
	idx := strings.Index(topic, "/")
	if idx == -1 {
		first = topic
	} else {
		first = topic[:idx]
		rest = topic[idx+1:]
	}

	// MQTT spec: $ topics should only match explicit $ patterns, not wildcards at root level
	isDollarTopic := len(first) > 0 && first[0] == '$'
	atRootLevel := matched == ""

	// Try exact match first
	if n.children != nil {
		if child, ok := n.children[first]; ok {
			newMatched := joinPath(matched, first)
			if route, values, ok := child.match(newMatched, rest); ok {
				return route, values, true
			}
		}
	}

	// Try single-level wildcard (+)
	// Skip if this is a $ topic at root level (MQTT spec compliance)
	if n.matchAny != nil && !(isDollarTopic && atRootLevel) {
		newMatched := joinPath(matched, "+")
		if route, values, ok := n.matchAny.match(newMatched, rest); ok {
			return route, values, true
		}
	}

	// Try multi-level wildcard (#)
	// Skip if this is a $ topic at root level (MQTT spec compliance)
	if n.matchAll != nil && !(isDollarTopic && atRootLevel) {
		newMatched := joinPath(matched, "#")
		if len(n.matchAll.values) > 0 {
			return newMatched, n.matchAll.values, true
		}
	}

	return "", nil, false
}

func (n *trieNode[T]) remove(pattern string, predicate func(T) bool) bool {
	if pattern == "" {
		before := len(n.values)
		newValues := make([]T, 0, len(n.values))
		for _, v := range n.values {
			if !predicate(v) {
				newValues = append(newValues, v)
			}
		}
		n.values = newValues
		return len(n.values) < before
	}

	var first, rest string
	idx := strings.Index(pattern, "/")
	if idx == -1 {
		first = pattern
	} else {
		first = pattern[:idx]
		rest = pattern[idx+1:]
	}

	switch first {
	case "+":
		if n.matchAny != nil {
			return n.matchAny.remove(rest, predicate)
		}
	case "#":
		if n.matchAll != nil {
			before := len(n.matchAll.values)
			newValues := make([]T, 0, len(n.matchAll.values))
			for _, v := range n.matchAll.values {
				if !predicate(v) {
					newValues = append(newValues, v)
				}
			}
			n.matchAll.values = newValues
			return len(n.matchAll.values) < before
		}
	default:
		if n.children != nil {
			if child, ok := n.children[first]; ok {
				return child.remove(rest, predicate)
			}
		}
	}

	return false
}

func joinPath(base, segment string) string {
	if base == "" {
		return segment
	}
	return base + "/" + segment
}
