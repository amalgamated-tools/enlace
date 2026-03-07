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
		t.Fatalf("failed to create db: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	creatorID := "user-1"
	if err := userRepo.Create(context.Background(), &model.User{
		ID:           creatorID,
		Email:        "user@example.com",
		PasswordHash: "hash",
		DisplayName:  "User",
	}); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if err := shareRepo.Create(context.Background(), &model.Share{
		ID:        "share-1",
		CreatorID: &creatorID,
		Slug:      "share-1",
		Name:      "Share 1",
	}); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	return repository.NewPendingUploadRepository(db.DB()), func() { _ = db.Close() }
}

func TestPendingUploadRepository_CreateGetFinalize(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	uploaderID := "user-1"
	upload := &model.PendingUpload{
		ID:         "upload-1",
		FileID:     "file-1",
		ShareID:    "share-1",
		UploaderID: &uploaderID,
		Filename:   "test.txt",
		Size:       11,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-1/test.txt",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	if err := repo.Create(context.Background(), upload); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err := repo.GetByID(context.Background(), upload.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Status != "pending" {
		t.Fatalf("expected pending status, got %q", got.Status)
	}

	if err := repo.Finalize(context.Background(), upload.ID); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	got, err = repo.GetByID(context.Background(), upload.ID)
	if err != nil {
		t.Fatalf("get after finalize failed: %v", err)
	}
	if got.Status != "finalized" {
		t.Fatalf("expected finalized status, got %q", got.Status)
	}
	if got.FinalizedAt == nil {
		t.Fatal("expected finalized_at to be set")
	}
}

func TestPendingUploadRepository_FinalizeIdempotency(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	upload := &model.PendingUpload{
		ID:         "upload-2",
		FileID:     "file-2",
		ShareID:    "share-1",
		Filename:   "test.txt",
		Size:       11,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-2/test.txt",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	if err := repo.Create(context.Background(), upload); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := repo.Finalize(context.Background(), upload.ID); err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}
	if err := repo.Finalize(context.Background(), upload.ID); err == nil {
		t.Fatal("expected second finalize to fail")
	}
}

func TestPendingUploadRepository_ExpireStale(t *testing.T) {
	repo, cleanup := setupPendingUploadRepo(t)
	defer cleanup()

	if err := repo.Create(context.Background(), &model.PendingUpload{
		ID:         "upload-expired",
		FileID:     "file-expired",
		ShareID:    "share-1",
		Filename:   "test.txt",
		Size:       1,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-expired/test.txt",
		ExpiresAt:  time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	updated, err := repo.ExpireStale(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("expire stale failed: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 expired row, got %d", updated)
	}
}
