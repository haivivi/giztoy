package kv_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// newBadgerStore creates an in-memory badger Store for testing.
func newBadgerStore(t *testing.T, opts *kv.Options) kv.Store {
	t.Helper()
	s, err := kv.NewBadger(kv.BadgerOptions{
		Options:  opts,
		InMemory: true,
	})
	if err != nil {
		t.Fatalf("NewBadger: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBadgerGetSetDelete(t *testing.T) {
	ctx := context.Background()
	s := newBadgerStore(t, nil)

	key := kv.Key{"user", "profile", "123"}
	val := []byte("hello")

	// Get non-existent key.
	_, err := s.Get(ctx, key)
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Set and Get.
	if err := s.Set(ctx, key, val); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(val) {
		t.Fatalf("Get = %q, want %q", got, val)
	}

	// Overwrite.
	val2 := []byte("world")
	if err := s.Set(ctx, key, val2); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	got, err = s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if string(got) != string(val2) {
		t.Fatalf("Get = %q, want %q", got, val2)
	}

	// Delete.
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.Get(ctx, key)
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete non-existent key should not error.
	if err := s.Delete(ctx, kv.Key{"no", "such", "key"}); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}

func TestBadgerList(t *testing.T) {
	ctx := context.Background()
	s := newBadgerStore(t, nil)

	entries := []kv.Entry{
		{Key: kv.Key{"m1", "g", "e", "Alice"}, Value: []byte("a")},
		{Key: kv.Key{"m1", "g", "e", "Bob"}, Value: []byte("b")},
		{Key: kv.Key{"m1", "g", "r", "Alice", "knows", "Bob"}, Value: []byte("r1")},
		{Key: kv.Key{"m1", "seg", "20260101", "1"}, Value: []byte("s1")},
		{Key: kv.Key{"m2", "g", "e", "Charlie"}, Value: []byte("c")},
	}
	if err := s.BatchSet(ctx, entries); err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	// List m1:g:e — should get Alice and Bob.
	var got []string
	for entry, err := range s.List(ctx, kv.Key{"m1", "g", "e"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String()+"="+string(entry.Value))
	}
	want := []string{
		"m1:g:e:Alice=a",
		"m1:g:e:Bob=b",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("List m1:g:e = %v, want %v", got, want)
	}

	// List m1 — should get all m1 entries.
	got = nil
	for entry, err := range s.List(ctx, kv.Key{"m1"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String())
	}
	if len(got) != 4 {
		t.Fatalf("List m1: got %d entries, want 4: %v", len(got), got)
	}

	// List with empty prefix — should get everything.
	got = nil
	for entry, err := range s.List(ctx, nil) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String())
	}
	if len(got) != 5 {
		t.Fatalf("List all: got %d entries, want 5: %v", len(got), got)
	}
}

func TestBadgerListPrefixBoundary(t *testing.T) {
	ctx := context.Background()
	s := newBadgerStore(t, nil)

	entries := []kv.Entry{
		{Key: kv.Key{"ab", "1"}, Value: []byte("yes")},
		{Key: kv.Key{"abc", "2"}, Value: []byte("no")},
		{Key: kv.Key{"ab", "3"}, Value: []byte("yes")},
	}
	if err := s.BatchSet(ctx, entries); err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	var got []string
	for entry, err := range s.List(ctx, kv.Key{"ab"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String())
	}
	want := []string{"ab:1", "ab:3"}
	if !slices.Equal(got, want) {
		t.Fatalf("List ab = %v, want %v", got, want)
	}
}

func TestBadgerBatchSetBatchDelete(t *testing.T) {
	ctx := context.Background()
	s := newBadgerStore(t, nil)

	entries := []kv.Entry{
		{Key: kv.Key{"a", "1"}, Value: []byte("v1")},
		{Key: kv.Key{"a", "2"}, Value: []byte("v2")},
		{Key: kv.Key{"a", "3"}, Value: []byte("v3")},
	}
	if err := s.BatchSet(ctx, entries); err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	// Verify all set.
	for _, e := range entries {
		got, err := s.Get(ctx, e.Key)
		if err != nil {
			t.Fatalf("Get %v: %v", e.Key, err)
		}
		if string(got) != string(e.Value) {
			t.Fatalf("Get %v = %q, want %q", e.Key, got, e.Value)
		}
	}

	// BatchDelete first two.
	if err := s.BatchDelete(ctx, []kv.Key{{"a", "1"}, {"a", "2"}}); err != nil {
		t.Fatalf("BatchDelete: %v", err)
	}

	_, err := s.Get(ctx, kv.Key{"a", "1"})
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a:1, got %v", err)
	}
	_, err = s.Get(ctx, kv.Key{"a", "2"})
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a:2, got %v", err)
	}
	got, err := s.Get(ctx, kv.Key{"a", "3"})
	if err != nil {
		t.Fatalf("Get a:3: %v", err)
	}
	if string(got) != "v3" {
		t.Fatalf("Get a:3 = %q, want %q", got, "v3")
	}
}

func TestBadgerCustomSeparator(t *testing.T) {
	ctx := context.Background()
	s := newBadgerStore(t, &kv.Options{Separator: '/'})

	key := kv.Key{"path", "to", "value"}
	val := []byte("data")

	if err := s.Set(ctx, key, val); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(val) {
		t.Fatalf("Get = %q, want %q", got, val)
	}

	var keys []string
	for entry, err := range s.List(ctx, kv.Key{"path", "to"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		keys = append(keys, entry.Key.String())
	}
	if len(keys) != 1 || keys[0] != "path:to:value" {
		t.Fatalf("List = %v, want [path:to:value]", keys)
	}
}

func TestBadgerDirRequired(t *testing.T) {
	_, err := kv.NewBadger(kv.BadgerOptions{
		Dir:      "",
		InMemory: false,
	})
	if err == nil {
		t.Fatal("expected error for empty Dir in on-disk mode")
	}
	if !strings.Contains(err.Error(), "Dir is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
