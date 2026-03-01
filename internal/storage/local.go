package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements the Storage interface using the local filesystem.
// Files are stored under basePath with the key serving as the relative path.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new LocalStorage instance.
// The basePath is the root directory where all files will be stored.
func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{basePath: basePath}
}

// Put stores data from reader at {basePath}/{key}.
// It creates any necessary parent directories.
// The contentType parameter is not used for local storage but is part of the interface.
func (s *LocalStorage) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath := filepath.Join(s.basePath, key)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		// Clean up the partial file on error
		os.Remove(fullPath)
		return err
	}

	return nil
}

// Get retrieves the file at {basePath}/{key}.
// Returns ErrNotFound if the file does not exist.
// The caller is responsible for closing the returned ReadCloser.
func (s *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(s.basePath, key)

	file, err := os.Open(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return file, nil
}

// Delete removes the file at {basePath}/{key}.
// Returns ErrNotFound if the file does not exist.
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath := filepath.Join(s.basePath, key)

	err := os.Remove(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}

	return nil
}

// Exists checks if the file at {basePath}/{key} exists.
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	fullPath := filepath.Join(s.basePath, key)

	_, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
