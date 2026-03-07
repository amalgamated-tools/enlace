// Package storage provides an abstraction layer for file storage operations.
// It defines a common interface that can be implemented by different backends
// such as local filesystem, S3, or other cloud storage services.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned when a requested file does not exist in storage.
var ErrNotFound = errors.New("file not found")

// ErrInvalidKey is returned when a storage key is malformed or would escape the storage root.
var ErrInvalidKey = errors.New("invalid storage key")

// Storage defines the interface for file storage operations.
// Implementations must be safe for concurrent use.
type Storage interface {
	// Put stores data from reader with the given key.
	// It creates any necessary directories/prefixes.
	// The size parameter is the expected content length.
	// The contentType parameter specifies the MIME type of the content.
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Get retrieves the data for the given key.
	// Returns ErrNotFound if the key does not exist.
	// The caller is responsible for closing the returned ReadCloser.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the data for the given key.
	// Returns ErrNotFound if the key does not exist.
	Delete(ctx context.Context, key string) error

	// Exists checks if data exists for the given key.
	// Returns true if the key exists, false otherwise.
	Exists(ctx context.Context, key string) (bool, error)
}

// PresignedURLResult holds a presigned URL and its metadata.
type PresignedURLResult struct {
	URL       string
	Method    string
	ExpiresAt time.Time
}

// ObjectInfo holds metadata about a stored object.
type ObjectInfo struct {
	Size        int64
	ContentType string
}

// DirectTransfer extends Storage with presigned URL capabilities.
// Backends that support direct client-to-storage transfers (e.g. S3)
// implement this interface. Backends that do not (e.g. local filesystem)
// only implement Storage, and callers use a type assertion to detect support.
type DirectTransfer interface {
	Storage

	// PresignUpload returns a short-lived presigned PUT URL for direct upload.
	// The URL enforces the declared size and content type.
	PresignUpload(ctx context.Context, key string, size int64, contentType string, expiry time.Duration) (*PresignedURLResult, error)

	// PresignDownload returns a short-lived presigned GET URL for direct download.
	// The disposition parameter sets the Content-Disposition header on the response.
	PresignDownload(ctx context.Context, key string, disposition string, expiry time.Duration) (*PresignedURLResult, error)

	// StatObject retrieves metadata about an object without downloading it.
	StatObject(ctx context.Context, key string) (*ObjectInfo, error)
}
