package kv

import (
	"bytes"
	"context"
	"iter"
	"sort"
	"sync"
)

// Memory is an in-memory Store implementation backed by a sorted map.
// It is safe for concurrent use and intended primarily for testing.
type Memory struct {
	mu   sync.RWMutex
	data map[string][]byte
	opts *Options
}

// NewMemory creates a new in-memory Store.
// Pass nil for default options.
func NewMemory(opts *Options) *Memory {
	return &Memory{
		data: make(map[string][]byte),
		opts: opts,
	}
}

func (m *Memory) Get(_ context.Context, key Key) ([]byte, error) {
	k := string(m.opts.encode(key))
	m.mu.RLock()
	v, ok := m.data[k]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	// Return a copy to prevent mutation.
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, nil
}

func (m *Memory) Set(_ context.Context, key Key, value []byte) error {
	k := string(m.opts.encode(key))
	cp := make([]byte, len(value))
	copy(cp, value)
	m.mu.Lock()
	m.data[k] = cp
	m.mu.Unlock()
	return nil
}

func (m *Memory) Delete(_ context.Context, key Key) error {
	k := string(m.opts.encode(key))
	m.mu.Lock()
	delete(m.data, k)
	m.mu.Unlock()
	return nil
}

func (m *Memory) List(_ context.Context, prefix Key) iter.Seq2[Entry, error] {
	p := m.opts.encode(prefix)
	// Append separator so "a:b" prefix doesn't match "a:bc".
	// But if prefix is empty, scan everything.
	var prefixBytes []byte
	if len(p) > 0 {
		prefixBytes = append(p, m.opts.sep())
	}

	// Snapshot matching keys under read lock.
	m.mu.RLock()
	type kv struct {
		key string
		val []byte
	}
	var matches []kv
	for k, v := range m.data {
		if len(prefixBytes) == 0 || bytes.HasPrefix([]byte(k), prefixBytes) {
			cp := make([]byte, len(v))
			copy(cp, v)
			matches = append(matches, kv{k, cp})
		}
	}
	m.mu.RUnlock()

	// Sort for deterministic lexicographic order.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].key < matches[j].key
	})

	return func(yield func(Entry, error) bool) {
		for _, kv := range matches {
			entry := Entry{
				Key:   m.opts.decode([]byte(kv.key)),
				Value: kv.val,
			}
			if !yield(entry, nil) {
				return
			}
		}
	}
}

func (m *Memory) BatchSet(_ context.Context, entries []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		k := string(m.opts.encode(e.Key))
		cp := make([]byte, len(e.Value))
		copy(cp, e.Value)
		m.data[k] = cp
	}
	return nil
}

func (m *Memory) BatchDelete(_ context.Context, keys []Key) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		k := string(m.opts.encode(key))
		delete(m.data, k)
	}
	return nil
}

func (m *Memory) Close() error {
	return nil
}
