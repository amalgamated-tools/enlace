package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// LocalStorage implements the Storage interface using the local filesystem.
// Files are stored under basePath with the key serving as the relative path.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new LocalStorage instance.
// The basePath is the root directory where all files will be stored.
func NewLocalStorage(basePath string) *LocalStorage {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		absBase = filepath.Clean(basePath)
	}
	return &LocalStorage{basePath: absBase}
}

func (s *LocalStorage) resolveKey(key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}

	normalized := strings.ReplaceAll(key, "\\", "/")
	cleanKey := path.Clean(normalized)

	if cleanKey == "." || cleanKey == "/" || cleanKey == "" || cleanKey == ".." || strings.HasPrefix(cleanKey, "../") {
		return "", ErrInvalidKey
	}
	if strings.HasPrefix(cleanKey, "/") {
		return "", ErrInvalidKey
	}
	if len(cleanKey) > 1 && cleanKey[1] == ':' {
		return "", ErrInvalidKey
	}

	// Convert to OS-specific separators for final path construction.
	osKey := filepath.FromSlash(cleanKey)
	if filepath.IsAbs(osKey) || filepath.VolumeName(osKey) != "" {
		return "", ErrInvalidKey
	}

	cleanKey = filepath.Clean(osKey)

	fullPath := filepath.Join(s.basePath, cleanKey)
	fullPath = filepath.Clean(fullPath)

	basePrefix := s.basePath
	if !strings.HasSuffix(basePrefix, string(os.PathSeparator)) {
		basePrefix += string(os.PathSeparator)
	}

	if !strings.HasPrefix(fullPath, basePrefix) {
		return "", ErrInvalidKey
	}

	return fullPath, nil
}

// Put stores data from reader at {basePath}/{key}.
// It creates any necessary parent directories.
// The contentType parameter is not used for local storage but is part of the interface.
func (s *LocalStorage) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath, err := s.resolveKey(key)
	if err != nil {
		return err
	}
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

	fullPath, err := s.resolveKey(key)
	if err != nil {
		return nil, err
	}

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

	fullPath, err := s.resolveKey(key)
	if err != nil {
		return err
	}

	err = os.Remove(fullPath)
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

	fullPath, err := s.resolveKey(key)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
