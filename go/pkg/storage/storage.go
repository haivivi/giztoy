// Package storage defines the FileStore interface for reading and writing
// files. It abstracts the underlying storage backend so that callers can
// swap between local disk, cloud object stores, or in-memory implementations
// without changing application code.
//
// The primary use case within giztoy is persisting HNSW index files for the
// memory system's vector search layer.
package storage

import (
	"context"
	"io"
)

// FileStore is a minimal interface for file-oriented storage.
//
// Paths are forward-slash separated and relative to the store root.
// Implementations must be safe for concurrent use.
type FileStore interface {
	// Read opens the named file for reading.
	// The caller must close the returned ReadCloser when done.
	// If the file does not exist, an error wrapping os.ErrNotExist is returned.
	Read(ctx context.Context, path string) (io.ReadCloser, error)

	// Write opens the named file for writing.
	// If the file already exists it is truncated.
	// Parent directories are created automatically.
	// The caller must close the returned WriteCloser to flush data.
	Write(ctx context.Context, path string) (io.WriteCloser, error)

	// Delete removes the named file.
	// If the file does not exist, Delete returns nil (idempotent).
	Delete(ctx context.Context, path string) error

	// Exists reports whether the named file exists.
	Exists(ctx context.Context, path string) (bool, error)
}
