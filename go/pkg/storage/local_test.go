package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestWriteErrorReadOnlyRoot(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Make root read-only so MkdirAll fails for nested paths.
	os.Chmod(dir, 0o444)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	ctx := context.Background()
	_, err = s.Write(ctx, "sub/file.txt")
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
}

func TestExistsPermissionError(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Create a subdirectory, put a file in it, then make the subdir unreadable.
	subdir := filepath.Join(dir, "locked")
	os.MkdirAll(subdir, 0o755)
	os.WriteFile(filepath.Join(subdir, "secret"), []byte("x"), 0o644)
	os.Chmod(subdir, 0o000)
	t.Cleanup(func() { os.Chmod(subdir, 0o755) })

	_, err = s.Exists(ctx, "locked/secret")
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestResolvePathTraversal(t *testing.T) {
	s := newTestLocal(t)

	// All traversal attempts must stay under root.
	cases := []string{
		"../etc/passwd",
		"a/../../etc/passwd",
		"../../../../../../../etc/passwd",
		"..\\etc\\passwd", // backslash variant
	}
	for _, tc := range cases {
		resolved := s.resolve(tc)
		if !strings.HasPrefix(resolved, s.root) {
			t.Errorf("resolve(%q) = %q, escapes root %q", tc, resolved, s.root)
		}
	}
}

// Verify Local satisfies FileStore at compile time.
var _ FileStore = (*Local)(nil)
