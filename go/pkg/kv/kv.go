// Package kv provides a key-value store interface with hierarchical path-based
// keys. Keys are represented as string slices (e.g., ["user", "profile", "123"])
// and encoded internally using a configurable separator (default ':').
//
// The package includes a BadgerDB-backed implementation for production use and
// an in-memory implementation for testing.
package kv

import (
	"context"
	"errors"
	"iter"
	"strings"
)

// Sentinel errors.
var (
	// ErrNotFound is returned when a key does not exist in the store.
	ErrNotFound = errors.New("kv: not found")
)

// Key is a hierarchical path represented as a slice of string segments.
// For example, Key{"user", "g", "e", "Alice"} encodes to "user:g:e:Alice"
// using the default separator ':'.
//
// Segments must not contain the configured separator character.
type Key []string

// String returns the key as a human-readable string using ':' as separator.
// This is for display/debug only; use Options.encode for storage encoding.
func (k Key) String() string {
	return strings.Join(k, ":")
}

// Entry is a key-value pair returned by List and used by BatchSet.
type Entry struct {
	Key   Key
	Value []byte
}

// Store is the interface for a key-value store with path-based keys.
type Store interface {
	// Get retrieves the value for a key. Returns ErrNotFound if not present.
	Get(ctx context.Context, key Key) ([]byte, error)

	// Set stores a key-value pair. Overwrites any existing value.
	Set(ctx context.Context, key Key, value []byte) error

	// Delete removes a key. No error if the key does not exist.
	Delete(ctx context.Context, key Key) error

	// List iterates over all entries whose key starts with the given prefix.
	// The iteration order is lexicographic by encoded key.
	List(ctx context.Context, prefix Key) iter.Seq2[Entry, error]

	// BatchSet atomically stores multiple key-value pairs.
	BatchSet(ctx context.Context, entries []Entry) error

	// BatchDelete atomically removes multiple keys.
	BatchDelete(ctx context.Context, keys []Key) error

	// Close releases any resources held by the store.
	Close() error
}

// DefaultSeparator is the default separator byte used to encode key segments.
const DefaultSeparator byte = ':'

// Options configures store behavior.
type Options struct {
	// Separator is the byte used to join key segments when encoding to storage.
	// Default is ':' if zero.
	Separator byte
}

// sep returns the effective separator.
func (o *Options) sep() byte {
	if o != nil && o.Separator != 0 {
		return o.Separator
	}
	return DefaultSeparator
}

// encode converts a Key to its byte representation using the separator.
func (o *Options) encode(k Key) []byte {
	s := o.sep()
	// Calculate total length to avoid allocations.
	n := 0
	for i, seg := range k {
		if i > 0 {
			n++ // separator
		}
		n += len(seg)
	}
	buf := make([]byte, n)
	pos := 0
	for i, seg := range k {
		if i > 0 {
			buf[pos] = s
			pos++
		}
		pos += copy(buf[pos:], seg)
	}
	return buf
}

// decode converts a byte representation back to a Key using the separator.
func (o *Options) decode(b []byte) Key {
	s := o.sep()
	parts := splitBytes(b, s)
	k := make(Key, len(parts))
	for i, p := range parts {
		k[i] = string(p)
	}
	return k
}

// splitBytes splits b by separator byte, similar to bytes.Split but returns
// [][]byte without importing bytes package for this single use.
func splitBytes(b []byte, sep byte) [][]byte {
	n := 1
	for _, c := range b {
		if c == sep {
			n++
		}
	}
	parts := make([][]byte, 0, n)
	start := 0
	for i, c := range b {
		if c == sep {
			parts = append(parts, b[start:i])
			start = i + 1
		}
	}
	parts = append(parts, b[start:])
	return parts
}
