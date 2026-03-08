package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

func setupPendingUploadRepository(t *testing.T) (*repository.PendingUploadRepository, *repository.ShareRepository, *repository.FileRepository, *model.Share, func()) {
	t.Helper()

	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	pendingRepo := repository.NewPendingUploadRepository(db.DB())

	user := &model.User{
		ID:           "user-1",
		Email:        "user@example.com",
		PasswordHash: "hash",
		DisplayName:  "User",
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	share := &model.Share{
		ID:        "share-1",
		CreatorID: &user.ID,
		Slug:      "share-1",
		Name:      "Share",
	}
	if err := shareRepo.Create(context.Background(), share); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	return pendingRepo, shareRepo, fileRepo, share, func() { _ = db.Close() }
}

func TestPendingUploadRepository_CreateGetAndFinalize(t *testing.T) {
	repo, _, fileRepo, share, cleanup := setupPendingUploadRepository(t)
	defer cleanup()

	uploaderID := "user-1"
	upload := &model.PendingUpload{
		ID:         "upload-1",
		FileID:     "file-1",
		ShareID:    share.ID,
		UploaderID: &uploaderID,
		Filename:   "test.txt",
		Size:       12,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-1/test.txt",
		ExpiresAt:  time.Now().Add(time.Hour),
	}

	if err := repo.Create(context.Background(), upload); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(context.Background(), upload.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != repository.PendingUploadStatusPending {
		t.Fatalf("expected pending status, got %q", got.Status)
	}

	file := &model.File{
		ID:         upload.FileID,
		ShareID:    upload.ShareID,
		UploaderID: upload.UploaderID,
		Name:       upload.Filename,
		Size:       upload.Size,
		MimeType:   upload.MimeType,
		StorageKey: upload.StorageKey,
	}
	if err := repo.Finalize(context.Background(), upload.ID, file); err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if _, err := fileRepo.GetByID(context.Background(), file.ID); err != nil {
		t.Fatalf("expected finalized file to be inserted, got %v", err)
	}

	finalized, err := repo.GetByID(context.Background(), upload.ID)
	if err != nil {
		t.Fatalf("GetByID() after finalize error = %v", err)
	}
	if finalized.Status != repository.PendingUploadStatusFinalized {
		t.Fatalf("expected finalized status, got %q", finalized.Status)
	}
	if finalized.FinalizedAt == nil {
		t.Fatal("expected finalized_at to be set")
	}

	if err := repo.Finalize(context.Background(), upload.ID, file); err == nil {
		t.Fatal("expected second finalize to fail")
	}
}

func TestPendingUploadRepository_ExpireStale(t *testing.T) {
	repo, _, _, share, cleanup := setupPendingUploadRepository(t)
	defer cleanup()

	upload := &model.PendingUpload{
		ID:         "upload-expired",
		FileID:     "file-expired",
		ShareID:    share.ID,
		Filename:   "old.txt",
		Size:       1,
		MimeType:   "text/plain",
		StorageKey: "share-1/file-expired/old.txt",
		ExpiresAt:  time.Now().Add(-time.Minute),
	}
	if err := repo.Create(context.Background(), upload); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	rows, err := repo.ExpireStale(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("ExpireStale() error = %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 expired row, got %d", rows)
	}

	got, err := repo.GetByID(context.Background(), upload.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != repository.PendingUploadStatusExpired {
		t.Fatalf("expected expired status, got %q", got.Status)
	}
}
