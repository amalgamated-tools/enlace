package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

func setupPendingUploadRepo(t *testing.T) (*repository.PendingUploadRepository, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	repo := repository.NewPendingUploadRepository(db.DB())
	return repo, func() { db.Close() }
}

func TestPendingUploadRepository_CreateAndGetByID(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	ctx := context.Background()
	pu := &model.PendingUpload{
		ID:         "upload-1",
		FileID:     "file-1",
		ShareID:    "share-1",
		Filename:   "test.txt",
		Size:       1024,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-1/test.txt",
		Status:     "pending",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	if err := repo.Create(ctx, pu); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByID(ctx, "upload-1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.ID != "upload-1" {
		t.Errorf("expected ID upload-1, got %s", got.ID)
	}
	if got.FileID != "file-1" {
		t.Errorf("expected FileID file-1, got %s", got.FileID)
	}
	if got.Status != "pending" {
		t.Errorf("expected status pending, got %s", got.Status)
	}
}

func TestPendingUploadRepository_GetByID_NotFound(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	ctx := context.Background()
	_, err := repo.GetByID(ctx, "nonexistent")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPendingUploadRepository_Finalize(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	ctx := context.Background()
	pu := &model.PendingUpload{
		ID:         "upload-2",
		FileID:     "file-2",
		ShareID:    "share-1",
		Filename:   "test.txt",
		Size:       1024,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-2/test.txt",
		Status:     "pending",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	if err := repo.Create(ctx, pu); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Finalize(ctx, "upload-2"); err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	got, err := repo.GetByID(ctx, "upload-2")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.Status != "finalized" {
		t.Errorf("expected status finalized, got %s", got.Status)
	}
	if got.FinalizedAt == nil {
		t.Error("expected FinalizedAt to be set")
	}
}

func TestPendingUploadRepository_Finalize_AlreadyFinalized(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	ctx := context.Background()
	pu := &model.PendingUpload{
		ID:         "upload-3",
		FileID:     "file-3",
		ShareID:    "share-1",
		Filename:   "test.txt",
		Size:       1024,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-3/test.txt",
		Status:     "pending",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	if err := repo.Create(ctx, pu); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Finalize(ctx, "upload-3"); err != nil {
		t.Fatalf("first Finalize failed: %v", err)
	}

	// Second finalize should fail
	err := repo.Finalize(ctx, "upload-3")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound on double finalize, got %v", err)
	}
}

func TestPendingUploadRepository_ExpireStale(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create an already-expired upload
	pu := &model.PendingUpload{
		ID:         "upload-expired",
		FileID:     "file-expired",
		ShareID:    "share-1",
		Filename:   "old.txt",
		Size:       512,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-expired/old.txt",
		Status:     "pending",
		ExpiresAt:  time.Now().Add(-1 * time.Minute),
	}
	if err := repo.Create(ctx, pu); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create a still-valid upload
	pu2 := &model.PendingUpload{
		ID:         "upload-valid",
		FileID:     "file-valid",
		ShareID:    "share-1",
		Filename:   "new.txt",
		Size:       512,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-valid/new.txt",
		Status:     "pending",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	if err := repo.Create(ctx, pu2); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	affected, err := repo.ExpireStale(ctx)
	if err != nil {
		t.Fatalf("ExpireStale failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 expired, got %d", affected)
	}

	// Verify expired
	got, _ := repo.GetByID(ctx, "upload-expired")
	if got.Status != "expired" {
		t.Errorf("expected status expired, got %s", got.Status)
	}

	// Verify still-valid is untouched
	got2, _ := repo.GetByID(ctx, "upload-valid")
	if got2.Status != "pending" {
		t.Errorf("expected status pending, got %s", got2.Status)
	}
}
