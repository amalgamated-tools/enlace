package service_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// mockStorage implements storage.Storage for testing.
type mockStorage struct {
	deletedKeys []string
	deleteErr   error
}

func (m *mockStorage) Put(_ context.Context, _ string, _ io.Reader, _ int64, _ string) error {
	return nil
}

func (m *mockStorage) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockStorage) Delete(_ context.Context, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deletedKeys = append(m.deletedKeys, key)
	return nil
}

func (m *mockStorage) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func setupShareService(t *testing.T) (*service.ShareService, *mockStorage, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	mockStore := &mockStorage{}
	shareService := service.NewShareService(shareRepo, fileRepo, mockStore)

	return shareService, mockStore, func() { db.Close() }
}

func setupShareServiceWithUser(t *testing.T) (*service.ShareService, *mockStorage, string, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	mockStore := &mockStorage{}

	// Create a user for testing
	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hash",
		DisplayName:  "Test User",
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	shareService := service.NewShareService(shareRepo, fileRepo, mockStore)

	return shareService, mockStore, user.ID, func() { db.Close() }
}

func TestShareService_Create(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	input := service.CreateShareInput{
		CreatorID:   userID,
		Name:        "Test Share",
		Description: "A test share",
	}

	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if share.ID == "" {
		t.Error("expected share ID to be set")
	}
	if share.Name != "Test Share" {
		t.Errorf("expected name 'Test Share', got %s", share.Name)
	}
	if share.Description != "A test share" {
		t.Errorf("expected description 'A test share', got %s", share.Description)
	}
	if share.Slug == "" {
		t.Error("expected slug to be generated")
	}
	if len(share.Slug) != 8 {
		t.Errorf("expected slug length 8, got %d", len(share.Slug))
	}
}

func TestShareService_Create_WithCustomSlug(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
		Slug:      "my-custom-slug",
	}

	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if share.Slug != "my-custom-slug" {
		t.Errorf("expected slug 'my-custom-slug', got %s", share.Slug)
	}
}

func TestShareService_Create_SlugExists(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create first share with custom slug
	input1 := service.CreateShareInput{
		CreatorID: userID,
		Name:      "First Share",
		Slug:      "duplicate-slug",
	}
	_, err := svc.Create(ctx, input1)
	if err != nil {
		t.Fatalf("failed to create first share: %v", err)
	}

	// Try to create second share with same slug
	input2 := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Second Share",
		Slug:      "duplicate-slug",
	}
	_, err = svc.Create(ctx, input2)
	if !errors.Is(err, service.ErrSlugExists) {
		t.Errorf("expected ErrSlugExists, got %v", err)
	}
}

func TestShareService_Create_WithPassword(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	password := "secret123"
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Protected Share",
		Password:  &password,
	}

	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if !share.HasPassword() {
		t.Error("expected share to have password")
	}
	// Password should be hashed
	if share.PasswordHash != nil && *share.PasswordHash == password {
		t.Error("password should be hashed, not stored in plain text")
	}
}

func TestShareService_Create_WithExpiry(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	expiresAt := time.Now().Add(24 * time.Hour)
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Expiring Share",
		ExpiresAt: &expiresAt,
	}

	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if share.ExpiresAt == nil {
		t.Error("expected expiry to be set")
	}
}

func TestShareService_Create_WithLimits(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	maxDownloads := 10
	maxViews := 100
	input := service.CreateShareInput{
		CreatorID:    userID,
		Name:         "Limited Share",
		MaxDownloads: &maxDownloads,
		MaxViews:     &maxViews,
	}

	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if share.MaxDownloads == nil || *share.MaxDownloads != 10 {
		t.Errorf("expected max downloads 10, got %v", share.MaxDownloads)
	}
	if share.MaxViews == nil || *share.MaxViews != 100 {
		t.Errorf("expected max views 100, got %v", share.MaxViews)
	}
}

func TestShareService_Create_ReverseShare(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	input := service.CreateShareInput{
		CreatorID:      userID,
		Name:           "Upload Share",
		IsReverseShare: true,
	}

	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if !share.IsReverseShare {
		t.Error("expected share to be a reverse share")
	}
}

func TestShareService_GetByID(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Get by ID
	share, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	if share.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, share.ID)
	}
	if share.Name != created.Name {
		t.Errorf("expected name %s, got %s", created.Name, share.Name)
	}
}

func TestShareService_GetByID_NotFound(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.GetByID(ctx, "nonexistent-id")
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestShareService_GetBySlug(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share with custom slug
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
		Slug:      "test-slug",
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Get by slug
	share, err := svc.GetBySlug(ctx, "test-slug")
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	if share.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, share.ID)
	}
}

func TestShareService_GetBySlug_NotFound(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.GetBySlug(ctx, "nonexistent-slug")
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestShareService_Update(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share
	input := service.CreateShareInput{
		CreatorID:   userID,
		Name:        "Original Name",
		Description: "Original Description",
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Update the share
	newName := "Updated Name"
	newDesc := "Updated Description"
	updateInput := service.UpdateShareInput{
		Name:        &newName,
		Description: &newDesc,
	}

	updated, err := svc.Update(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("failed to update share: %v", err)
	}

	if updated.Name != newName {
		t.Errorf("expected name %s, got %s", newName, updated.Name)
	}
	if updated.Description != newDesc {
		t.Errorf("expected description %s, got %s", newDesc, updated.Description)
	}
}

func TestShareService_Update_NotFound(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	newName := "Updated Name"
	updateInput := service.UpdateShareInput{
		Name: &newName,
	}

	_, err := svc.Update(ctx, "nonexistent-id", updateInput)
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestShareService_Update_SetPassword(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share without password
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Add password
	password := "newsecret"
	updateInput := service.UpdateShareInput{
		Password: &password,
	}

	updated, err := svc.Update(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("failed to update share: %v", err)
	}

	if !updated.HasPassword() {
		t.Error("expected share to have password after update")
	}
}

func TestShareService_Update_ClearPassword(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share with password
	password := "secret123"
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
		Password:  &password,
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if !created.HasPassword() {
		t.Fatal("share should have password initially")
	}

	// Clear password
	updateInput := service.UpdateShareInput{
		ClearPassword: true,
	}

	updated, err := svc.Update(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("failed to update share: %v", err)
	}

	if updated.HasPassword() {
		t.Error("expected share to not have password after clearing")
	}
}

func TestShareService_Update_ClearExpiry(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share with expiry
	expiresAt := time.Now().Add(24 * time.Hour)
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
		ExpiresAt: &expiresAt,
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if created.ExpiresAt == nil {
		t.Fatal("share should have expiry initially")
	}

	// Clear expiry
	updateInput := service.UpdateShareInput{
		ClearExpiry: true,
	}

	updated, err := svc.Update(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("failed to update share: %v", err)
	}

	if updated.ExpiresAt != nil {
		t.Error("expected share to not have expiry after clearing")
	}
}

func TestShareService_Delete(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Delete the share
	err = svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to delete share: %v", err)
	}

	// Verify it's gone
	_, err = svc.GetByID(ctx, created.ID)
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound after deletion, got %v", err)
	}
}

func TestShareService_Delete_NotFound(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	err := svc.Delete(ctx, "nonexistent-id")
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestShareService_Delete_WithFiles(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	mockStore := &mockStorage{}

	// Create user
	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hash",
		DisplayName:  "Test User",
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	svc := service.NewShareService(shareRepo, fileRepo, mockStore)
	ctx := context.Background()

	// Create a share
	input := service.CreateShareInput{
		CreatorID: user.ID,
		Name:      "Share with files",
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Add files directly to repository
	files := []*model.File{
		{ID: "file-1", ShareID: share.ID, Name: "file1.txt", StorageKey: "key1", Size: 100, MimeType: "text/plain"},
		{ID: "file-2", ShareID: share.ID, Name: "file2.txt", StorageKey: "key2", Size: 200, MimeType: "text/plain"},
	}
	for _, f := range files {
		if err := fileRepo.Create(ctx, f); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Delete the share
	err = svc.Delete(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to delete share: %v", err)
	}

	// Verify storage was cleaned up
	if len(mockStore.deletedKeys) != 2 {
		t.Errorf("expected 2 deleted keys, got %d", len(mockStore.deletedKeys))
	}
	expectedKeys := map[string]bool{"key1": true, "key2": true}
	for _, key := range mockStore.deletedKeys {
		if !expectedKeys[key] {
			t.Errorf("unexpected deleted key: %s", key)
		}
	}
}

func TestShareService_ListByCreator(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple shares
	for i := 0; i < 3; i++ {
		input := service.CreateShareInput{
			CreatorID: userID,
			Name:      "Test Share",
		}
		_, err := svc.Create(ctx, input)
		if err != nil {
			t.Fatalf("failed to create share: %v", err)
		}
	}

	// List shares
	shares, err := svc.ListByCreator(ctx, userID)
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}

	if len(shares) != 3 {
		t.Errorf("expected 3 shares, got %d", len(shares))
	}
}

func TestShareService_ListByCreator_Empty(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	shares, err := svc.ListByCreator(ctx, "unknown-user")
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}

	if len(shares) != 0 {
		t.Errorf("expected 0 shares, got %d", len(shares))
	}
}

func TestShareService_VerifyPassword(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share with password
	password := "secret123"
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Protected Share",
		Password:  &password,
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Verify correct password
	if !svc.VerifyPassword(ctx, share.ID, "secret123") {
		t.Error("expected password verification to succeed")
	}

	// Verify wrong password
	if svc.VerifyPassword(ctx, share.ID, "wrongpassword") {
		t.Error("expected password verification to fail for wrong password")
	}
}

func TestShareService_VerifyPassword_NoPassword(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share without password
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Public Share",
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Should return true for shares without password
	if !svc.VerifyPassword(ctx, share.ID, "anypassword") {
		t.Error("expected password verification to succeed for share without password")
	}
}

func TestShareService_VerifyPassword_NotFound(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	if svc.VerifyPassword(ctx, "nonexistent-id", "password") {
		t.Error("expected password verification to fail for nonexistent share")
	}
}

func TestShareService_ValidateAccess(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a valid share
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Valid Share",
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Validate access
	err = svc.ValidateAccess(ctx, share)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestShareService_ValidateAccess_Expired(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create an expired share
	expiresAt := time.Now().Add(-24 * time.Hour)
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Expired Share",
		ExpiresAt: &expiresAt,
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Validate access
	err = svc.ValidateAccess(ctx, share)
	if !errors.Is(err, service.ErrShareExpired) {
		t.Errorf("expected ErrShareExpired, got %v", err)
	}
}

func TestShareService_ValidateAccess_DownloadLimitReached(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	mockStore := &mockStorage{}

	// Create user
	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hash",
		DisplayName:  "Test User",
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	svc := service.NewShareService(shareRepo, fileRepo, mockStore)
	ctx := context.Background()

	// Create a share with max downloads of 1
	maxDownloads := 1
	input := service.CreateShareInput{
		CreatorID:    user.ID,
		Name:         "Limited Share",
		MaxDownloads: &maxDownloads,
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Increment download count to reach limit
	err = svc.IncrementDownloadCount(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to increment download count: %v", err)
	}

	// Fetch updated share
	share, err = svc.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	// Validate access
	err = svc.ValidateAccess(ctx, share)
	if !errors.Is(err, service.ErrDownloadLimit) {
		t.Errorf("expected ErrDownloadLimit, got %v", err)
	}
}

func TestShareService_ValidateAccess_ViewLimitReached(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	mockStore := &mockStorage{}

	// Create user
	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hash",
		DisplayName:  "Test User",
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	svc := service.NewShareService(shareRepo, fileRepo, mockStore)
	ctx := context.Background()

	// Create a share with max views of 1
	maxViews := 1
	input := service.CreateShareInput{
		CreatorID: user.ID,
		Name:      "Limited Share",
		MaxViews:  &maxViews,
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Increment view count to reach limit
	err = svc.IncrementViewCount(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to increment view count: %v", err)
	}

	// Fetch updated share
	share, err = svc.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	// Validate access
	err = svc.ValidateAccess(ctx, share)
	if !errors.Is(err, service.ErrViewLimit) {
		t.Errorf("expected ErrViewLimit, got %v", err)
	}
}

func TestShareService_IncrementDownloadCount(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if share.DownloadCount != 0 {
		t.Errorf("expected initial download count 0, got %d", share.DownloadCount)
	}

	// Increment download count
	err = svc.IncrementDownloadCount(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to increment download count: %v", err)
	}

	// Verify count increased
	updated, err := svc.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	if updated.DownloadCount != 1 {
		t.Errorf("expected download count 1, got %d", updated.DownloadCount)
	}
}

func TestShareService_IncrementDownloadCount_NotFound(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	err := svc.IncrementDownloadCount(ctx, "nonexistent-id")
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestShareService_IncrementViewCount(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
	}
	share, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if share.ViewCount != 0 {
		t.Errorf("expected initial view count 0, got %d", share.ViewCount)
	}

	// Increment view count
	err = svc.IncrementViewCount(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to increment view count: %v", err)
	}

	// Verify count increased
	updated, err := svc.GetByID(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to get share: %v", err)
	}

	if updated.ViewCount != 1 {
		t.Errorf("expected view count 1, got %d", updated.ViewCount)
	}
}

func TestShareService_IncrementViewCount_NotFound(t *testing.T) {
	svc, _, cleanup := setupShareService(t)
	defer cleanup()

	ctx := context.Background()

	err := svc.IncrementViewCount(ctx, "nonexistent-id")
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestShareService_GeneratedSlugIsUnique(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple shares and verify slugs are unique
	slugs := make(map[string]bool)
	for i := 0; i < 10; i++ {
		input := service.CreateShareInput{
			CreatorID: userID,
			Name:      "Test Share",
		}
		share, err := svc.Create(ctx, input)
		if err != nil {
			t.Fatalf("failed to create share: %v", err)
		}

		if slugs[share.Slug] {
			t.Errorf("duplicate slug generated: %s", share.Slug)
		}
		slugs[share.Slug] = true
	}
}

func TestShareService_Update_SetLimits(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a share without limits
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Update with limits
	maxDownloads := 50
	maxViews := 500
	updateInput := service.UpdateShareInput{
		MaxDownloads: &maxDownloads,
		MaxViews:     &maxViews,
	}

	updated, err := svc.Update(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("failed to update share: %v", err)
	}

	if updated.MaxDownloads == nil || *updated.MaxDownloads != 50 {
		t.Errorf("expected max downloads 50, got %v", updated.MaxDownloads)
	}
	if updated.MaxViews == nil || *updated.MaxViews != 500 {
		t.Errorf("expected max views 500, got %v", updated.MaxViews)
	}
}

func TestShareService_Update_SetReverseShare(t *testing.T) {
	svc, _, userID, cleanup := setupShareServiceWithUser(t)
	defer cleanup()

	ctx := context.Background()

	// Create a regular share
	input := service.CreateShareInput{
		CreatorID: userID,
		Name:      "Test Share",
	}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	if created.IsReverseShare {
		t.Fatal("share should not be reverse share initially")
	}

	// Update to reverse share
	isReverse := true
	updateInput := service.UpdateShareInput{
		IsReverseShare: &isReverse,
	}

	updated, err := svc.Update(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("failed to update share: %v", err)
	}

	if !updated.IsReverseShare {
		t.Error("expected share to be reverse share after update")
	}
}
