package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

func TestShareRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}

	err := repo.Create(ctx, share)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	found, err := repo.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}
	if found.Slug != share.Slug {
		t.Errorf("expected slug %s, got %s", share.Slug, found.Slug)
	}
	if found.Name != share.Name {
		t.Errorf("expected name %s, got %s", share.Name, found.Name)
	}
	if found.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestShareRepository_Create_WithAllFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	// Create a user first for the foreign key
	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
	}
	_ = userRepo.Create(ctx, user)

	creatorID := "user-123"
	passwordHash := "sharepasshash"
	expiresAt := time.Now().Add(24 * time.Hour)
	maxDownloads := 10

	share := &model.Share{
		ID:             "share-full",
		CreatorID:      &creatorID,
		Slug:           "full-share",
		Name:           "Full Share",
		Description:    "A share with all fields",
		PasswordHash:   &passwordHash,
		ExpiresAt:      &expiresAt,
		MaxDownloads:   &maxDownloads,
		DownloadCount:  5,
		IsReverseShare: true,
	}

	err := shareRepo.Create(ctx, share)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	found, err := shareRepo.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	if found.CreatorID == nil || *found.CreatorID != creatorID {
		t.Errorf("expected creator_id %s, got %v", creatorID, found.CreatorID)
	}
	if found.Description != share.Description {
		t.Errorf("expected description %s, got %s", share.Description, found.Description)
	}
	if found.PasswordHash == nil || *found.PasswordHash != passwordHash {
		t.Errorf("expected password_hash %s, got %v", passwordHash, found.PasswordHash)
	}
	if found.MaxDownloads == nil || *found.MaxDownloads != maxDownloads {
		t.Errorf("expected max_downloads %d, got %v", maxDownloads, found.MaxDownloads)
	}
	if found.DownloadCount != 5 {
		t.Errorf("expected download_count 5, got %d", found.DownloadCount)
	}
	if !found.IsReverseShare {
		t.Error("expected is_reverse_share to be true")
	}
}

func TestShareRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestShareRepository_GetBySlug(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:   "share-123",
		Slug: "unique-slug",
		Name: "Test Share",
	}
	_ = repo.Create(ctx, share)

	found, err := repo.GetBySlug(ctx, "unique-slug")
	if err != nil {
		t.Fatalf("failed to get share by slug: %v", err)
	}
	if found.ID != share.ID {
		t.Errorf("expected ID %s, got %s", share.ID, found.ID)
	}
}

func TestShareRepository_GetBySlug_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	_, err := repo.GetBySlug(ctx, "nonexistent-slug")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestShareRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:          "share-123",
		Slug:        "my-share",
		Name:        "Test Share",
		Description: "Original description",
	}
	_ = repo.Create(ctx, share)

	// Update the share
	newMaxDownloads := 50
	updatedShare := &model.Share{
		ID:           share.ID,
		Slug:         "updated-slug",
		Name:         "Updated Share",
		Description:  "Updated description",
		MaxDownloads: &newMaxDownloads,
	}

	err := repo.Update(ctx, updatedShare)
	if err != nil {
		t.Fatalf("failed to update share: %v", err)
	}

	found, err := repo.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}
	if found.Slug != "updated-slug" {
		t.Errorf("expected slug %s, got %s", "updated-slug", found.Slug)
	}
	if found.Name != "Updated Share" {
		t.Errorf("expected name %s, got %s", "Updated Share", found.Name)
	}
	if found.Description != "Updated description" {
		t.Errorf("expected description %s, got %s", "Updated description", found.Description)
	}
	if found.MaxDownloads == nil || *found.MaxDownloads != 50 {
		t.Errorf("expected max_downloads 50, got %v", found.MaxDownloads)
	}
}

func TestShareRepository_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:   "nonexistent",
		Slug: "test",
		Name: "Test",
	}

	err := repo.Update(ctx, share)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestShareRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}
	_ = repo.Create(ctx, share)

	err := repo.Delete(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to delete share: %v", err)
	}

	_, err = repo.GetByID(ctx, share.ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestShareRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent-id")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestShareRepository_ListByCreator(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	// Create users
	user1 := &model.User{ID: "user-1", Email: "user1@example.com", PasswordHash: "hash", DisplayName: "User 1"}
	user2 := &model.User{ID: "user-2", Email: "user2@example.com", PasswordHash: "hash", DisplayName: "User 2"}
	_ = userRepo.Create(ctx, user1)
	_ = userRepo.Create(ctx, user2)

	// Create shares for user1
	user1ID := "user-1"
	for i := 1; i <= 3; i++ {
		share := &model.Share{
			ID:        "share-user1-" + string(rune('0'+i)),
			CreatorID: &user1ID,
			Slug:      "user1-share-" + string(rune('0'+i)),
			Name:      "User1 Share",
		}
		_ = shareRepo.Create(ctx, share)
	}

	// Create share for user2
	user2ID := "user-2"
	share := &model.Share{
		ID:        "share-user2-1",
		CreatorID: &user2ID,
		Slug:      "user2-share-1",
		Name:      "User2 Share",
	}
	_ = shareRepo.Create(ctx, share)

	// List shares by user1
	shares, err := shareRepo.ListByCreator(ctx, "user-1")
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 3 {
		t.Errorf("expected 3 shares for user-1, got %d", len(shares))
	}

	// List shares by user2
	shares, err = shareRepo.ListByCreator(ctx, "user-2")
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 1 {
		t.Errorf("expected 1 share for user-2, got %d", len(shares))
	}
}

func TestShareRepository_ListByCreator_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	shares, err := repo.ListByCreator(ctx, "nonexistent-user")
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("expected 0 shares, got %d", len(shares))
	}
}

func TestShareRepository_SlugExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:   "share-123",
		Slug: "existing-slug",
		Name: "Test Share",
	}
	_ = repo.Create(ctx, share)

	exists, err := repo.SlugExists(ctx, "existing-slug")
	if err != nil {
		t.Fatalf("failed to check slug exists: %v", err)
	}
	if !exists {
		t.Error("expected slug to exist")
	}

	exists, err = repo.SlugExists(ctx, "nonexistent-slug")
	if err != nil {
		t.Fatalf("failed to check slug exists: %v", err)
	}
	if exists {
		t.Error("expected slug to not exist")
	}
}

func TestShareRepository_IncrementDownloadCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:            "share-123",
		Slug:          "my-share",
		Name:          "Test Share",
		DownloadCount: 5,
	}
	_ = repo.Create(ctx, share)

	err := repo.IncrementDownloadCount(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to increment download count: %v", err)
	}

	found, err := repo.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}
	if found.DownloadCount != 6 {
		t.Errorf("expected download_count 6, got %d", found.DownloadCount)
	}
}

func TestShareRepository_IncrementDownloadCount_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	err := repo.IncrementDownloadCount(ctx, "nonexistent-id")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestShareRepository_NullableFieldsHandledCorrectly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	// Create share with all nullable fields as nil
	share := &model.Share{
		ID:   "share-nulls",
		Slug: "null-share",
		Name: "Null Share",
	}
	err := repo.Create(ctx, share)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	found, err := repo.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	if found.CreatorID != nil {
		t.Errorf("expected CreatorID to be nil, got %v", found.CreatorID)
	}
	if found.PasswordHash != nil {
		t.Errorf("expected PasswordHash to be nil, got %v", found.PasswordHash)
	}
	if found.ExpiresAt != nil {
		t.Errorf("expected ExpiresAt to be nil, got %v", found.ExpiresAt)
	}
	if found.MaxDownloads != nil {
		t.Errorf("expected MaxDownloads to be nil, got %v", found.MaxDownloads)
	}
}

func TestShareRepository_TrackSessionDownload_NewSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:            "share-123",
		Slug:          "my-share",
		Name:          "Test Share",
		DownloadCount: 0,
	}
	_ = repo.Create(ctx, share)

	counted, err := repo.TrackSessionDownload(ctx, share.ID, "session-1")
	if err != nil {
		t.Fatalf("failed to track session download: %v", err)
	}
	if !counted {
		t.Error("expected first session to be counted")
	}

	found, _ := repo.GetByID(ctx, share.ID)
	if found.DownloadCount != 1 {
		t.Errorf("expected download_count 1, got %d", found.DownloadCount)
	}
}

func TestShareRepository_TrackSessionDownload_DuplicateSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}
	_ = repo.Create(ctx, share)

	// First call
	_, _ = repo.TrackSessionDownload(ctx, share.ID, "session-1")

	// Second call with same session
	counted, err := repo.TrackSessionDownload(ctx, share.ID, "session-1")
	if err != nil {
		t.Fatalf("failed to track duplicate session: %v", err)
	}
	if counted {
		t.Error("expected duplicate session not to be counted")
	}

	found, _ := repo.GetByID(ctx, share.ID)
	if found.DownloadCount != 1 {
		t.Errorf("expected download_count 1, got %d", found.DownloadCount)
	}
}

func TestShareRepository_TrackSessionDownload_DifferentSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewShareRepository(db.DB())
	ctx := context.Background()

	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}
	_ = repo.Create(ctx, share)

	counted1, _ := repo.TrackSessionDownload(ctx, share.ID, "session-1")
	counted2, _ := repo.TrackSessionDownload(ctx, share.ID, "session-2")

	if !counted1 || !counted2 {
		t.Error("expected both different sessions to be counted")
	}

	found, _ := repo.GetByID(ctx, share.ID)
	if found.DownloadCount != 2 {
		t.Errorf("expected download_count 2, got %d", found.DownloadCount)
	}
}
