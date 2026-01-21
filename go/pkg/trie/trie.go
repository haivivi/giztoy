// Package trie provides a generic trie data structure for efficient path-based
// storage and retrieval. It supports MQTT-style topic patterns with wildcards:
//   - "/a/b/c" - exact path match
//   - "/a/+/c" - single-level wildcard (matches any single segment)
//   - "/a/#"   - multi-level wildcard (matches any remaining path segments)
//
// The trie is useful for routing, topic matching, and hierarchical data
// storage.
package trie

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidPattern is returned when the path pattern is invalid.
var ErrInvalidPattern = errors.New("invalid path pattern, path should be /a/b/c or /a/+/c or /a/#")

// Trie is a generic trie data structure that supports path-based storage with
// wildcard matching. It uses MQTT-style topic patterns where:
//   - "/" separates path segments
//   - "+" matches any single segment (single-level wildcard)
//   - "#" matches any remaining path segments (multi-level wildcard)
//
// The trie stores values of type T at specific paths and allows efficient
// lookup and pattern matching operations.
type Trie[T any] struct {
	children map[string]*Trie[T] // exact path segment matches
	matchAny *Trie[T]            // single-level wildcard ("+") matches
	matchAll *Trie[T]            // multi-level wildcard ("#") matches
	set      bool                // whether this node has a value set
	value    T                   // the value stored at this node
}

// New creates a new empty Trie.
func New[T any]() *Trie[T] {
	return &Trie[T]{}
}

func (t *Trie[T]) setFunc(fn func(ptr *T, existed bool) error) error {
	if err := fn(&t.value, t.set); err != nil {
		return err
	}
	t.set = true
	return nil
}

// Set stores a value at the specified path using the provided setFunc. The
// setFunc is called with a pointer to the value and a boolean indicating
// whether a value already existed at this path.
//
// Path patterns supported:
//   - "/a/b/c"   - exact path segments
//   - "/a/b/c/"  - has the same effect as "/a/b/c"
//   - "/a/b//c/" - will add a child with the name "" to the node of "/a/b"
//   - "/a/+/c"   - single-level wildcard (matches any single segment)
//   - "/a/#"     - multi-level wildcard (must be at the end of path)
//
// Returns an error if the path pattern is invalid or if setFunc returns an
// error.
func (t *Trie[T]) Set(path string, setFunc func(ptr *T, existed bool) error) error {
	if len(path) == 0 {
		return t.setFunc(setFunc)
	}

	var first, subseq string
	if idx := strings.IndexByte(path, '/'); idx == -1 {
		first = path
	} else {
		first = path[:idx]
		subseq = path[idx+1:]
	}
	if t.children != nil {
		if ch, ok := t.children[first]; ok {
			return ch.Set(subseq, setFunc)
		}
	}

	switch first {
	case "+": // .../<first:+>/<subseq>
		if t.matchAny == nil {
			t.matchAny = &Trie[T]{}
		}
		return t.matchAny.Set(subseq, setFunc)
	case "#": // .../<first:#>
		if len(subseq) != 0 {
			return ErrInvalidPattern
		}
		if t.matchAll == nil {
			t.matchAll = &Trie[T]{}
		}
		return t.matchAll.setFunc(setFunc)
	default: // .../<first>/<subseq>
		if t.children == nil {
			t.children = make(map[string]*Trie[T])
		}
		ch := &Trie[T]{}
		t.children[first] = ch
		return ch.Set(subseq, setFunc)
	}
}

// SetValue is a convenience method that stores a value at the specified path.
// It is equivalent to Set(path, func(ptr *T, _ bool) error { *ptr = value; return nil }).
func (t *Trie[T]) SetValue(path string, value T) error {
	return t.Set(path, func(ptr *T, _ bool) error {
		*ptr = value
		return nil
	})
}

// Get retrieves the value stored at the specified path. It performs pattern
// matching to find the best match for the given path. The matching follows MQTT
// topic subscription rules where exact matches take precedence over wildcard
// matches.
//
// Returns the value and true if found, nil and false otherwise.
func (t *Trie[T]) Get(path string) (*T, bool) {
	_, val, ok := t.match("", path)
	return val, ok
}

// GetValue retrieves the value stored at the specified path.
// Returns the value and true if found, zero value and false otherwise.
func (t *Trie[T]) GetValue(path string) (T, bool) {
	ptr, ok := t.Get(path)
	if !ok {
		var zero T
		return zero, false
	}
	return *ptr, true
}

// Match returns the matched route pattern and value for the given path.
// Returns empty string and nil if no match is found.
func (t *Trie[T]) Match(path string) (route string, value *T, ok bool) {
	return t.match("", path)
}

func (t *Trie[T]) match(matched, path string) (string, *T, bool) {
	if len(path) == 0 {
		return matched, &t.value, t.set
	}
	var first, subseq string
	p := strings.IndexByte(path, '/')
	if p == -1 {
		first = path
	} else {
		first = path[:p]
		subseq = path[p+1:]
	}
	if t.children != nil {
		if ch, ok := t.children[first]; ok {
			if route, handlers, ok := ch.match(matched+"/"+first, subseq); ok {
				return route, handlers, true
			}
		}
	}
	if t.matchAny != nil {
		if route, handlers, ok := t.matchAny.match(matched+"/+", subseq); ok {
			return route, handlers, true
		}
	}
	if t.matchAll != nil {
		if route, handlers, ok := t.matchAll.match(matched+"/#", ""); ok {
			return route, handlers, true
		}
	}
	return "", nil, false
}

// Walk calls the given function for each node in the trie.
func (t *Trie[T]) Walk(f func(path string, value T, set bool)) {
	t.walkWithPath(nil, func(path []string, node *Trie[T]) {
		f(strings.Join(path, "/"), node.value, node.set)
	})
}

func (t *Trie[T]) walkWithPath(path []string, f func([]string, *Trie[T])) {
	if t.children != nil {
		for seg, ch := range t.children {
			ch.walkWithPath(append(path, seg), f)
		}
	}
	if t.matchAny != nil {
		t.matchAny.walkWithPath(append(path, "+"), f)
	}
	if t.matchAll != nil {
		t.matchAll.walkWithPath(append(path, "#"), f)
	}
	f(path, t)
}

// String returns a string representation of the trie structure. It shows all
// nodes with their paths and values, sorted alphabetically. Useful for
// debugging and understanding the trie's internal structure.
func (t *Trie[T]) String() string {
	var lines []string
	t.walkWithPath(nil, func(path []string, node *Trie[T]) {
		if node.set {
			lines = append(lines, fmt.Sprintf("%s: %v", strings.Join(path, "/"), node.value))
		}
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// Len returns the number of values stored in the trie.
func (t *Trie[T]) Len() int {
	count := 0
	t.Walk(func(_ string, _ T, set bool) {
		if set {
			count++
		}
	})
	return count
}
