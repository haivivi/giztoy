package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func newTestLocal(t *testing.T) *Local {
	t.Helper()
	s, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWriteAndRead(t *testing.T) {
	s := newTestLocal(t)
	ctx := context.Background()

	const data = "hello, storage"
	w, err := s.Write(ctx, "a/b/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := s.Read(ctx, "a/b/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != data {
		t.Fatalf("got %q, want %q", got, data)
	}
}

func TestReadNotExist(t *testing.T) {
	s := newTestLocal(t)
	ctx := context.Background()

	_, err := s.Read(ctx, "no-such-file")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestExists(t *testing.T) {
	s := newTestLocal(t)
	ctx := context.Background()

	ok, err := s.Exists(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false for missing file")
	}

	w, err := s.Write(ctx, "present")
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	ok, err = s.Exists(ctx, "present")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true for existing file")
	}
}

func TestDeleteIdempotent(t *testing.T) {
	s := newTestLocal(t)
	ctx := context.Background()

	// Delete a file that doesn't exist — should succeed.
	if err := s.Delete(ctx, "ghost"); err != nil {
		t.Fatal(err)
	}

	// Write then delete.
	w, err := s.Write(ctx, "tmp")
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	if err := s.Delete(ctx, "tmp"); err != nil {
		t.Fatal(err)
	}

	ok, err := s.Exists(ctx, "tmp")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("file should be gone after delete")
	}

	// Delete again — idempotent.
	if err := s.Delete(ctx, "tmp"); err != nil {
		t.Fatal(err)
	}
}

func TestWriteTruncates(t *testing.T) {
	s := newTestLocal(t)
	ctx := context.Background()

	// First write.
	w, err := s.Write(ctx, "f")
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(w, "long content here")
	w.Close()

	// Overwrite with shorter data.
	w, err = s.Write(ctx, "f")
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(w, "short")
	w.Close()

	r, err := s.Read(ctx, "f")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "short" {
		t.Fatalf("got %q, want %q", got, "short")
	}
}

func TestNewLocalCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	s, err := NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.root)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

// Verify Local satisfies FileStore at compile time.
var _ FileStore = (*Local)(nil)
