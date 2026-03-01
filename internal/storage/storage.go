// Package storage provides an abstraction layer for file storage operations.
// It defines a common interface that can be implemented by different backends
// such as local filesystem, S3, or other cloud storage services.
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound is returned when a requested file does not exist in storage.
var ErrNotFound = errors.New("file not found")

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
