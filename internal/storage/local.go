package storage

import (
	"context"
	"errors"
	"fmt"
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
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("invalid local storage base path %q: %w", basePath, err)
	}

	if stat, statErr := os.Stat(absBase); statErr == nil {
		if !stat.IsDir() {
			return nil, fmt.Errorf("local storage base path %q is not a directory", absBase)
		}
		if realBase, evalErr := filepath.EvalSymlinks(absBase); evalErr == nil {
			absBase = realBase
		} else {
			return nil, fmt.Errorf("failed to resolve storage base path %q: %w", absBase, evalErr)
		}
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("cannot access local storage base path %q: %w", absBase, statErr)
	}
	return &LocalStorage{basePath: absBase}, nil
}

func (s *LocalStorage) resolveKey(key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}

	normalized := strings.ReplaceAll(key, "\\", "/")
	cleanKey := path.Clean(normalized)

	if cleanKey == "." || cleanKey == "/" || cleanKey == ".." || strings.HasPrefix(cleanKey, "../") {
		return "", ErrInvalidKey
	}
	if strings.HasPrefix(cleanKey, "/") {
		return "", ErrInvalidKey
	}
	if len(cleanKey) >= 2 && cleanKey[1] == ':' {
		return "", ErrInvalidKey
	}

	// Convert to OS-specific separators for final path construction.
	osKey := filepath.FromSlash(cleanKey)
	if filepath.IsAbs(osKey) || filepath.VolumeName(osKey) != "" {
		return "", ErrInvalidKey
	}

	cleanOSKey := filepath.Clean(osKey)

	fullPath := filepath.Join(s.basePath, cleanOSKey)
	fullPath = filepath.Clean(fullPath)

	if !isWithinBasePath(s.basePath, fullPath) {
		return "", ErrInvalidKey
	}

	currentPath := s.basePath
	for _, part := range strings.Split(cleanOSKey, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}

		currentPath = filepath.Join(currentPath, part)
		info, err := os.Lstat(currentPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if !isWithinBasePath(s.basePath, currentPath) {
					return "", ErrInvalidKey
				}
				continue
			}
			return "", err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolvedPath, err := filepath.EvalSymlinks(currentPath)
			if err != nil {
				return "", ErrInvalidKey
			}
			currentPath = resolvedPath
		}

		if !isWithinBasePath(s.basePath, currentPath) {
			return "", ErrInvalidKey
		}
	}

	return currentPath, nil
}

func isWithinBasePath(basePath, candidate string) bool {
	relPath, err := filepath.Rel(basePath, candidate)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) && relPath != ".."
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
