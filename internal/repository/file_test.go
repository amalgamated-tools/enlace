package repository_test

import (
	"context"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

// createTestShare is a helper to create a share for file FK constraints.
func createTestShare(t *testing.T, shareRepo *repository.ShareRepository, id string) *model.Share {
	t.Helper()
	share := &model.Share{
		ID:   id,
		Slug: "share-" + id,
		Name: "Test Share " + id,
	}
	if err := shareRepo.Create(context.Background(), share); err != nil {
		t.Fatalf("failed to create test share: %v", err)
	}
	return share
}

func TestFileRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	// Create share first (FK constraint)
	share := createTestShare(t, shareRepo, "share-1")

	file := &model.File{
		ID:         "file-123",
		ShareID:    share.ID,
		Name:       "document.pdf",
		Size:       1024,
		MimeType:   "application/pdf",
		StorageKey: "files/share-1/document.pdf",
	}

	err := fileRepo.Create(ctx, file)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	found, err := fileRepo.GetByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("failed to get file: %v", err)
	}
	if found.Name != file.Name {
		t.Errorf("expected name %s, got %s", file.Name, found.Name)
	}
	if found.Size != file.Size {
		t.Errorf("expected size %d, got %d", file.Size, found.Size)
	}
	if found.MimeType != file.MimeType {
		t.Errorf("expected mime_type %s, got %s", file.MimeType, found.MimeType)
	}
	if found.StorageKey != file.StorageKey {
		t.Errorf("expected storage_key %s, got %s", file.StorageKey, found.StorageKey)
	}
	if found.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestFileRepository_Create_WithUploader(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	// Create user for uploader FK
	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
	}
	_ = userRepo.Create(ctx, user)

	// Create share
	share := createTestShare(t, shareRepo, "share-1")

	uploaderID := "user-123"
	file := &model.File{
		ID:         "file-123",
		ShareID:    share.ID,
		UploaderID: &uploaderID,
		Name:       "document.pdf",
		Size:       2048,
		MimeType:   "application/pdf",
		StorageKey: "files/share-1/document.pdf",
	}

	err := fileRepo.Create(ctx, file)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	found, err := fileRepo.GetByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("failed to get file: %v", err)
	}
	if found.UploaderID == nil || *found.UploaderID != uploaderID {
		t.Errorf("expected uploader_id %s, got %v", uploaderID, found.UploaderID)
	}
}

func TestFileRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFileRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share := createTestShare(t, shareRepo, "share-1")

	file := &model.File{
		ID:         "file-123",
		ShareID:    share.ID,
		Name:       "document.pdf",
		Size:       1024,
		MimeType:   "application/pdf",
		StorageKey: "files/share-1/document.pdf",
	}
	_ = fileRepo.Create(ctx, file)

	err := fileRepo.Delete(ctx, file.ID)
	if err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	_, err = fileRepo.GetByID(ctx, file.ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestFileRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent-id")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFileRepository_ListByShare(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share1 := createTestShare(t, shareRepo, "share-1")
	share2 := createTestShare(t, shareRepo, "share-2")

	// Create files for share1
	for i := 1; i <= 3; i++ {
		file := &model.File{
			ID:         "file-share1-" + string(rune('0'+i)),
			ShareID:    share1.ID,
			Name:       "file" + string(rune('0'+i)) + ".txt",
			Size:       int64(i * 100),
			MimeType:   "text/plain",
			StorageKey: "files/share-1/file" + string(rune('0'+i)) + ".txt",
		}
		_ = fileRepo.Create(ctx, file)
	}

	// Create file for share2
	file := &model.File{
		ID:         "file-share2-1",
		ShareID:    share2.ID,
		Name:       "other.txt",
		Size:       500,
		MimeType:   "text/plain",
		StorageKey: "files/share-2/other.txt",
	}
	_ = fileRepo.Create(ctx, file)

	// List files by share1
	files, err := fileRepo.ListByShare(ctx, share1.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 files for share-1, got %d", len(files))
	}

	// List files by share2
	files, err = fileRepo.ListByShare(ctx, share2.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file for share-2, got %d", len(files))
	}
}

func TestFileRepository_ListByShare_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share := createTestShare(t, shareRepo, "share-1")

	files, err := fileRepo.ListByShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestFileRepository_DeleteByShare(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share1 := createTestShare(t, shareRepo, "share-1")
	share2 := createTestShare(t, shareRepo, "share-2")

	// Create files for share1
	for i := 1; i <= 3; i++ {
		file := &model.File{
			ID:         "file-share1-" + string(rune('0'+i)),
			ShareID:    share1.ID,
			Name:       "file" + string(rune('0'+i)) + ".txt",
			Size:       int64(i * 100),
			MimeType:   "text/plain",
			StorageKey: "files/share-1/file" + string(rune('0'+i)) + ".txt",
		}
		_ = fileRepo.Create(ctx, file)
	}

	// Create file for share2
	file := &model.File{
		ID:         "file-share2-1",
		ShareID:    share2.ID,
		Name:       "other.txt",
		Size:       500,
		MimeType:   "text/plain",
		StorageKey: "files/share-2/other.txt",
	}
	_ = fileRepo.Create(ctx, file)

	// Delete all files for share1
	err := fileRepo.DeleteByShare(ctx, share1.ID)
	if err != nil {
		t.Fatalf("failed to delete files by share: %v", err)
	}

	// Verify share1 files are deleted
	files, err := fileRepo.ListByShare(ctx, share1.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for share-1 after deletion, got %d", len(files))
	}

	// Verify share2 files are untouched
	files, err = fileRepo.ListByShare(ctx, share2.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file for share-2, got %d", len(files))
	}
}

func TestFileRepository_DeleteByShare_NoFiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share := createTestShare(t, shareRepo, "share-1")

	// Should not error when there are no files to delete
	err := fileRepo.DeleteByShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("expected no error when deleting files for share with no files, got %v", err)
	}
}

func TestFileRepository_GetStorageKeysByShare(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share1 := createTestShare(t, shareRepo, "share-1")
	share2 := createTestShare(t, shareRepo, "share-2")

	// Create files for share1
	expectedKeys := []string{
		"files/share-1/file1.txt",
		"files/share-1/file2.txt",
		"files/share-1/file3.txt",
	}
	for i, key := range expectedKeys {
		file := &model.File{
			ID:         "file-share1-" + string(rune('0'+i+1)),
			ShareID:    share1.ID,
			Name:       "file" + string(rune('0'+i+1)) + ".txt",
			Size:       int64((i + 1) * 100),
			MimeType:   "text/plain",
			StorageKey: key,
		}
		_ = fileRepo.Create(ctx, file)
	}

	// Create file for share2
	file := &model.File{
		ID:         "file-share2-1",
		ShareID:    share2.ID,
		Name:       "other.txt",
		Size:       500,
		MimeType:   "text/plain",
		StorageKey: "files/share-2/other.txt",
	}
	_ = fileRepo.Create(ctx, file)

	// Get storage keys for share1
	keys, err := fileRepo.GetStorageKeysByShare(ctx, share1.ID)
	if err != nil {
		t.Fatalf("failed to get storage keys: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("expected 3 storage keys, got %d", len(keys))
	}

	// Verify each expected key is present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}
	for _, expected := range expectedKeys {
		if !keyMap[expected] {
			t.Errorf("expected storage key %s not found", expected)
		}
	}

	// Get storage keys for share2
	keys, err = fileRepo.GetStorageKeysByShare(ctx, share2.ID)
	if err != nil {
		t.Fatalf("failed to get storage keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 storage key for share-2, got %d", len(keys))
	}
	if len(keys) > 0 && keys[0] != "files/share-2/other.txt" {
		t.Errorf("expected storage key %s, got %s", "files/share-2/other.txt", keys[0])
	}
}

func TestFileRepository_GetStorageKeysByShare_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share := createTestShare(t, shareRepo, "share-1")

	keys, err := fileRepo.GetStorageKeysByShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get storage keys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 storage keys, got %d", len(keys))
	}
}

func TestFileRepository_NullableFieldsHandledCorrectly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	ctx := context.Background()

	share := createTestShare(t, shareRepo, "share-1")

	// Create file with nil UploaderID
	file := &model.File{
		ID:         "file-nulls",
		ShareID:    share.ID,
		Name:       "document.pdf",
		Size:       1024,
		MimeType:   "application/pdf",
		StorageKey: "files/share-1/document.pdf",
	}
	err := fileRepo.Create(ctx, file)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	found, err := fileRepo.GetByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("failed to get file: %v", err)
	}

	if found.UploaderID != nil {
		t.Errorf("expected UploaderID to be nil, got %v", found.UploaderID)
	}
}
