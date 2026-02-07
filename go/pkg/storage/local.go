package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Local implements FileStore on top of the local filesystem.
// All paths are resolved relative to the configured root directory.
type Local struct {
	root string
}

// NewLocal creates a Local store rooted at dir.
// The directory is created (with parents) if it does not already exist.
func NewLocal(dir string) (*Local, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Local{root: abs}, nil
}

// resolve turns a storage path into an absolute filesystem path.
func (l *Local) resolve(path string) string {
	return filepath.Join(l.root, filepath.FromSlash(path))
}

// Read opens the named file for reading.
func (l *Local) Read(_ context.Context, path string) (io.ReadCloser, error) {
	f, err := os.Open(l.resolve(path))
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Write opens the named file for writing, creating parent directories as
// needed. If the file already exists it is truncated.
func (l *Local) Write(_ context.Context, path string) (io.WriteCloser, error) {
	full := l.resolve(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(full)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Delete removes the named file. If the file does not exist, Delete
// returns nil (idempotent).
func (l *Local) Delete(_ context.Context, path string) error {
	err := os.Remove(l.resolve(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Exists reports whether the named file exists.
func (l *Local) Exists(_ context.Context, path string) (bool, error) {
	_, err := os.Stat(l.resolve(path))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
