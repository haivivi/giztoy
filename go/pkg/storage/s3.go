package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// S3Client abstracts the S3 API operations used by [S3Store].
// The [s3.Client] type satisfies this interface.
type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

// S3Store implements FileStore backed by Amazon S3 or any S3-compatible
// object store (MinIO, R2, etc.).
//
// All storage paths are mapped to S3 keys under an optional prefix.
// The caller is responsible for configuring the [s3.Client] with appropriate
// credentials, region, and endpoint.
type S3Store struct {
	client S3Client
	bucket string
	prefix string
}

// NewS3 creates an S3-backed FileStore.
//
// The client should be pre-configured (credentials, region, endpoint).
// Any type satisfying [S3Client] is accepted; typically an [s3.Client].
// Prefix is prepended to all object keys; pass "" for no prefix.
func NewS3(client S3Client, bucket, prefix string) *S3Store {
	return &S3Store{client: client, bucket: bucket, prefix: prefix}
}

// key builds the full S3 object key for the given storage path.
func (s *S3Store) key(path string) string {
	if s.prefix == "" {
		return path
	}
	return s.prefix + "/" + path
}

// Read opens the named object for reading via GetObject.
// Returns an error wrapping os.ErrNotExist if the key does not exist.
func (s *S3Store) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(path)),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, fmt.Errorf("storage: read %s: %w", path, os.ErrNotExist)
		}
		return nil, err
	}
	return out.Body, nil
}

// Write returns a writer that streams data to S3 via PutObject.
//
// A background goroutine performs the upload, reading from an [io.Pipe].
// The caller must close the writer to complete the upload; Close blocks
// until the upload finishes and returns any S3 error.
func (s *S3Store) Write(ctx context.Context, path string) (io.WriteCloser, error) {
	pr, pw := io.Pipe()
	w := &s3Writer{pw: pw, done: make(chan struct{})}
	go func() {
		defer close(w.done)
		_, w.uploadErr = s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(s.key(path)),
			Body:   pr,
		})
		// If the upload failed early, unblock any pending writes so the
		// caller's Write calls don't hang forever.
		pr.CloseWithError(w.uploadErr)
	}()
	return w, nil
}

// Delete removes the named object via DeleteObject.
// S3 DeleteObject is already idempotent (returns success for missing keys).
func (s *S3Store) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(path)),
	})
	return err
}

// Exists checks whether the named object exists via HeadObject.
func (s *S3Store) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(path)),
	})
	if err != nil {
		if isS3NotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// s3Writer streams data to a background PutObject call through an io.Pipe.
type s3Writer struct {
	pw        *io.PipeWriter
	done      chan struct{}
	uploadErr error
}

func (w *s3Writer) Write(p []byte) (int, error) {
	return w.pw.Write(p)
}

// Close signals EOF to the PutObject reader, waits for the upload to
// complete, and returns the upload error (if any).
func (w *s3Writer) Close() error {
	w.pw.Close() // signal EOF → PutObject finishes reading
	<-w.done     // wait for upload goroutine
	return w.uploadErr
}

// isS3NotFound reports whether err indicates the S3 object does not exist.
func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}
	return false
}

// Compile-time interface check.
var _ FileStore = (*S3Store)(nil)
