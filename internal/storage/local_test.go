package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/storage"
)

func newLocalStore(t *testing.T, basePath string) *storage.LocalStorage {
	t.Helper()

	store, err := storage.NewLocalStorage(basePath)
	if err != nil {
		t.Fatalf("failed to create local storage: %v", err)
	}
	return store
}

func TestLocalStorage_PutAndGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	content := []byte("hello world")
	err = store.Put(ctx, "test/file.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	reader, err := store.Get(ctx, "test/file.txt")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestLocalStorage_Get_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	_, err = store.Get(ctx, "nonexistent/file.txt")
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	content := []byte("to be deleted")
	err = store.Put(ctx, "delete/file.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	err = store.Delete(ctx, "delete/file.txt")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, "delete/file.txt")
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestLocalStorage_Delete_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	err = store.Delete(ctx, "nonexistent/file.txt")
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLocalStorage_Exists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	content := []byte("exists test")
	err = store.Put(ctx, "exists/file.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	exists, err := store.Exists(ctx, "exists/file.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected file to exist")
	}
}

func TestLocalStorage_Exists_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	exists, err := store.Exists(ctx, "nonexistent/file.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected file to not exist")
	}
}

func TestLocalStorage_Put_NestedDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	content := []byte("deeply nested content")
	key := "a/b/c/d/e/file.txt"
	err = store.Put(ctx, key, bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed for nested directories: %v", err)
	}

	// Verify file exists at the correct location
	fullPath := filepath.Join(tmpDir, key)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %s", fullPath)
	}

	// Verify content is correct
	reader, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestLocalStorage_Put_Overwrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	// Write initial content
	content1 := []byte("initial content")
	err = store.Put(ctx, "overwrite/file.txt", bytes.NewReader(content1), int64(len(content1)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Overwrite with new content
	content2 := []byte("updated content")
	err = store.Put(ctx, "overwrite/file.txt", bytes.NewReader(content2), int64(len(content2)), "text/plain")
	if err != nil {
		t.Fatalf("Put (overwrite) failed: %v", err)
	}

	// Verify new content
	reader, err := store.Get(ctx, "overwrite/file.txt")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}
	if !bytes.Equal(got, content2) {
		t.Errorf("expected %q, got %q", content2, got)
	}
}

func TestLocalStorage_Put_EmptyContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	content := []byte{}
	err = store.Put(ctx, "empty/file.txt", bytes.NewReader(content), 0, "text/plain")
	if err != nil {
		t.Fatalf("Put failed for empty content: %v", err)
	}

	exists, err := store.Exists(ctx, "empty/file.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected empty file to exist")
	}

	reader, err := store.Get(ctx, "empty/file.txt")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty content, got %d bytes", len(got))
	}
}

func TestLocalStorage_Put_LargeFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	// Create a 1MB file
	size := 1024 * 1024
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	err = store.Put(ctx, "large/file.bin", bytes.NewReader(content), int64(len(content)), "application/octet-stream")
	if err != nil {
		t.Fatalf("Put failed for large file: %v", err)
	}

	reader, err := store.Get(ctx, "large/file.bin")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("large file content mismatch")
	}
}

func TestLocalStorage_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := []byte("should not be written")
	err = store.Put(ctx, "cancelled/file.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	_, err = store.Get(ctx, "any/file.txt")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled for Get, got %v", err)
	}

	err = store.Delete(ctx, "any/file.txt")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled for Delete, got %v", err)
	}

	_, err = store.Exists(ctx, "any/file.txt")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled for Exists, got %v", err)
	}
}

func TestLocalStorage_RootLevelFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	content := []byte("root level content")
	err = store.Put(ctx, "rootfile.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Put failed for root level file: %v", err)
	}

	reader, err := store.Get(ctx, "rootfile.txt")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestLocalStorage_InterfaceCompliance(t *testing.T) {
	// Verify that LocalStorage implements the Storage interface
	var _ storage.Storage = (*storage.LocalStorage)(nil)
}

func TestLocalStorage_RejectsTraversalAndAbsoluteKeys(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	keys := []string{
		"../evil.txt",
		"..\\evil.txt",
		"/absolute.txt",
		"../../nested/evil.txt",
		"\\windows-style.txt",
		"C:\\evil.txt",
		"C:/evil.txt",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			data := []byte("should be rejected")
			if err := store.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "text/plain"); !errors.Is(err, storage.ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey from Put, got %v", err)
			}

			if _, err := store.Get(ctx, key); !errors.Is(err, storage.ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey from Get, got %v", err)
			}

			if err := store.Delete(ctx, key); !errors.Is(err, storage.ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey from Delete, got %v", err)
			}

			if _, err := store.Exists(ctx, key); !errors.Is(err, storage.ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey from Exists, got %v", err)
			}

			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatalf("failed to read base dir: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("expected no files to be created, found %d", len(entries))
			}
		})
	}
}

func TestLocalStorage_RejectsSymlinkEscape(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outsideDir, err := os.MkdirTemp("", "outside-dir")
	if err != nil {
		t.Fatalf("failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	linkPath := filepath.Join(tmpDir, "link")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	store := newLocalStore(t, tmpDir)
	ctx := context.Background()

	key := "link/escape.txt"
	if err := store.Put(ctx, key, bytes.NewReader([]byte("data")), 4, "text/plain"); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey when writing via symlink, got %v", err)
	}

	if _, err := store.Get(ctx, key); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey when reading via symlink, got %v", err)
	}

	if err := store.Delete(ctx, key); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey when deleting via symlink, got %v", err)
	}

	if exists, err := store.Exists(ctx, key); err == nil && exists {
		t.Fatalf("expected file to not exist via symlink escape")
	}

	if _, err := os.Stat(filepath.Join(outsideDir, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no file to be written outside base path, got err %v", err)
	}
}
