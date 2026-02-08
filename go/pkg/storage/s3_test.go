package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// ---------------------------------------------------------------------------
// mock S3 client
// ---------------------------------------------------------------------------

// apiError implements smithy.APIError for test assertions.
type apiError struct {
	code string
	msg  string
}

func (e *apiError) Error() string            { return e.msg }
func (e *apiError) ErrorCode() string        { return e.code }
func (e *apiError) ErrorMessage() string     { return e.msg }
func (e *apiError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

var errNoSuchKey = &apiError{code: "NoSuchKey", msg: "no such key"}
var errNotFound  = &apiError{code: "NotFound", msg: "not found"}

// mockS3 is a thread-safe in-memory S3 backend for testing.
type mockS3 struct {
	mu      sync.Mutex
	objects map[string][]byte

	// Optional hooks to inject errors.
	getErr    error
	putErr    error
	deleteErr error
	headErr   error
}

func newMockS3() *mockS3 {
	return &mockS3{objects: make(map[string][]byte)}
}

func (m *mockS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[*in.Key]
	if !ok {
		return nil, errNoSuchKey
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (m *mockS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	data, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[*in.Key] = data
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3) HeadObject(_ context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if m.headErr != nil {
		return nil, m.headErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[*in.Key]; !ok {
		return nil, errNotFound
	}
	return &s3.HeadObjectOutput{}, nil
}

// ---------------------------------------------------------------------------
// S3Store tests
// ---------------------------------------------------------------------------

func newTestS3(t *testing.T) (*S3Store, *mockS3) {
	t.Helper()
	mock := newMockS3()
	store := NewS3(mock, "test-bucket", "")
	return store, mock
}

func TestS3WriteAndRead(t *testing.T) {
	store, _ := newTestS3(t)
	ctx := context.Background()

	const data = "hello s3"
	w, err := store.Write(ctx, "obj.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := store.Read(ctx, "obj.txt")
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

func TestS3ReadNotExist(t *testing.T) {
	store, _ := newTestS3(t)
	ctx := context.Background()

	_, err := store.Read(ctx, "missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestS3ReadOtherError(t *testing.T) {
	mock := newMockS3()
	mock.getErr = errors.New("network timeout")
	store := NewS3(mock, "bucket", "pfx")
	ctx := context.Background()

	_, err := store.Read(ctx, "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatal("should not be ErrNotExist for generic errors")
	}
	if err.Error() != "network timeout" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3Exists(t *testing.T) {
	store, mock := newTestS3(t)
	ctx := context.Background()

	ok, err := store.Exists(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false for missing key")
	}

	// Seed an object directly.
	mock.mu.Lock()
	mock.objects["present"] = []byte("data")
	mock.mu.Unlock()

	ok, err = store.Exists(ctx, "present")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true for existing key")
	}
}

func TestS3ExistsOtherError(t *testing.T) {
	mock := newMockS3()
	mock.headErr = errors.New("network failure")
	store := NewS3(mock, "bucket", "")
	ctx := context.Background()

	_, err := store.Exists(ctx, "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "network failure" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3DeleteIdempotent(t *testing.T) {
	store, mock := newTestS3(t)
	ctx := context.Background()

	// Delete non-existent — should succeed (S3 semantics).
	if err := store.Delete(ctx, "ghost"); err != nil {
		t.Fatal(err)
	}

	// Seed then delete.
	mock.mu.Lock()
	mock.objects["tmp"] = []byte("x")
	mock.mu.Unlock()

	if err := store.Delete(ctx, "tmp"); err != nil {
		t.Fatal(err)
	}

	ok, err := store.Exists(ctx, "tmp")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("key should be gone after delete")
	}
}

func TestS3DeleteError(t *testing.T) {
	mock := newMockS3()
	mock.deleteErr = errors.New("access denied")
	store := NewS3(mock, "bucket", "")
	ctx := context.Background()

	err := store.Delete(ctx, "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestS3WriteUploadError(t *testing.T) {
	mock := newMockS3()
	mock.putErr = errors.New("upload failed")
	store := NewS3(mock, "bucket", "")
	ctx := context.Background()

	w, err := store.Write(ctx, "obj")
	if err != nil {
		t.Fatal(err)
	}
	// Write some data — the pipe may or may not accept it depending on
	// how fast the goroutine fails.
	io.WriteString(w, "data")
	// Close must return the upload error.
	err = w.Close()
	if err == nil {
		t.Fatal("expected upload error from Close")
	}
	if err.Error() != "upload failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3KeyPrefix(t *testing.T) {
	mock := newMockS3()
	store := NewS3(mock, "bucket", "my/prefix")
	ctx := context.Background()

	w, err := store.Write(ctx, "file.bin")
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(w, "content")
	w.Close()

	// The object should be stored under the prefixed key.
	mock.mu.Lock()
	_, ok := mock.objects["my/prefix/file.bin"]
	mock.mu.Unlock()
	if !ok {
		t.Fatal("expected key with prefix my/prefix/file.bin")
	}
}

func TestS3KeyNoPrefix(t *testing.T) {
	store := NewS3(newMockS3(), "bucket", "")
	if got := store.key("a/b"); got != "a/b" {
		t.Fatalf("key = %q, want %q", got, "a/b")
	}
}

func TestS3WriteTruncates(t *testing.T) {
	store, _ := newTestS3(t)
	ctx := context.Background()

	// First write.
	w, _ := store.Write(ctx, "f")
	io.WriteString(w, "long content here")
	w.Close()

	// Overwrite.
	w, _ = store.Write(ctx, "f")
	io.WriteString(w, "short")
	w.Close()

	r, _ := store.Read(ctx, "f")
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != "short" {
		t.Fatalf("got %q, want %q", got, "short")
	}
}

func TestIsS3NotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"NoSuchKey", errNoSuchKey, true},
		{"NotFound", errNotFound, true},
		{"other api error", &apiError{code: "AccessDenied", msg: "denied"}, false},
		{"plain error", errors.New("timeout"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isS3NotFound(tt.err); got != tt.want {
				t.Fatalf("isS3NotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
