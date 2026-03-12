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

	return cleanOSKey, nil
}

func isWithinBasePath(basePath, candidate string) bool {
	relPath, err := filepath.Rel(basePath, candidate)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) && relPath != ".."
}

func (s *LocalStorage) openRoot() (*os.Root, error) {
	return os.OpenRoot(s.basePath)
}

func (s *LocalStorage) mapRootPathError(err error) error {
	if err == nil {
		return nil
	}

	// Root operations enforce the actual confinement boundary. Map their
	// permission-denied escape failures directly instead of re-running resolveKey,
	// which would introduce a TOCTOU window under concurrent filesystem changes.
	if errors.Is(err, os.ErrPermission) {
		return ErrInvalidKey
	}

	return err
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

	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	dir := filepath.Dir(fullPath)

	if dir != "." {
		if err := s.mapRootPathError(root.MkdirAll(dir, 0755)); err != nil {
			return err
		}
	}

	file, err := root.Create(fullPath)
	if err != nil {
		return s.mapRootPathError(err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		// Clean up the partial file on error
		return errors.Join(err, root.Remove(fullPath))
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

	root, err := s.openRoot()
	if err != nil {
		return nil, err
	}

	file, err := root.Open(fullPath)
	if err != nil {
		closeErr := root.Close()
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, errors.Join(s.mapRootPathError(err), closeErr)
	}

	return &rootedReadCloser{Root: root, File: file}, nil
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

	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	err = root.Remove(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return s.mapRootPathError(err)
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

	root, err := s.openRoot()
	if err != nil {
		return false, err
	}
	defer root.Close()

	_, err = root.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, s.mapRootPathError(err)
	}

	return true, nil
}

// rootedReadCloser keeps the root handle alive until the opened file is closed,
// then closes both resources together.
type rootedReadCloser struct {
	*os.Root
	*os.File
}

func (r *rootedReadCloser) Close() error {
	fileErr := r.File.Close()
	rootErr := r.Root.Close()
	return errors.Join(fileErr, rootErr)
}
