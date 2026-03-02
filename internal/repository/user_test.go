package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

func setupTestDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	return db
}

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
		IsAdmin:      false,
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Verify user was created
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if found.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, found.Email)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
	}
	_ = repo.Create(ctx, user)

	found, err := repo.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, found.ID)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "notfound@example.com")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
		IsAdmin:      false,
	}
	_ = repo.Create(ctx, user)

	// Update user
	updatedUser := &model.User{
		ID:           user.ID,
		Email:        "updated@example.com",
		PasswordHash: "newhash",
		DisplayName:  "Updated User",
		IsAdmin:      true,
	}

	err := repo.Update(ctx, updatedUser)
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	// Verify update
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if found.Email != "updated@example.com" {
		t.Errorf("expected email %s, got %s", "updated@example.com", found.Email)
	}
	if found.DisplayName != "Updated User" {
		t.Errorf("expected display name %s, got %s", "Updated User", found.DisplayName)
	}
	if !found.IsAdmin {
		t.Error("expected IsAdmin to be true")
	}
}

func TestUserRepository_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	user := &model.User{
		ID:           "nonexistent",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
	}

	err := repo.Update(ctx, user)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
	}
	_ = repo.Create(ctx, user)

	err := repo.Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	// Verify deletion
	_, err = repo.GetByID(ctx, user.ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestUserRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent-id")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	// Create multiple users
	users := []*model.User{
		{ID: "user-1", Email: "user1@example.com", PasswordHash: "hash1", DisplayName: "User 1"},
		{ID: "user-2", Email: "user2@example.com", PasswordHash: "hash2", DisplayName: "User 2"},
		{ID: "user-3", Email: "user3@example.com", PasswordHash: "hash3", DisplayName: "User 3"},
	}
	for _, u := range users {
		_ = repo.Create(ctx, u)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 users, got %d", len(list))
	}
}

func TestUserRepository_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 users, got %d", len(list))
	}
}

func TestUserRepository_EmailExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		DisplayName:  "Test User",
	}
	_ = repo.Create(ctx, user)

	exists, err := repo.EmailExists(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("failed to check email exists: %v", err)
	}
	if !exists {
		t.Error("expected email to exist")
	}

	exists, err = repo.EmailExists(ctx, "notfound@example.com")
	if err != nil {
		t.Fatalf("failed to check email exists: %v", err)
	}
	if exists {
		t.Error("expected email to not exist")
	}
}

func TestUserRepository_GetByOIDC(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	// Create user with OIDC
	user := &model.User{
		ID:          "oidc-user-id",
		Email:       "oidc@example.com",
		DisplayName: "OIDC User",
		OIDCSubject: "sub123",
		OIDCIssuer:  "https://auth.example.com",
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Find by OIDC
	found, err := repo.GetByOIDC(ctx, "https://auth.example.com", "sub123")
	if err != nil {
		t.Fatalf("failed to get by OIDC: %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, found.ID)
	}

	// Not found case
	_, err = repo.GetByOIDC(ctx, "https://other.com", "sub123")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_UpdateOIDC(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	// Create user without OIDC
	user := &model.User{
		ID:           "link-user-id",
		Email:        "link@example.com",
		PasswordHash: "hash",
		DisplayName:  "Link User",
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Link OIDC
	user.OIDCSubject = "newsub"
	user.OIDCIssuer = "https://new.example.com"
	if err := repo.Update(ctx, user); err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	// Verify
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if found.OIDCSubject != "newsub" {
		t.Errorf("expected OIDCSubject 'newsub', got %s", found.OIDCSubject)
	}
	if found.OIDCIssuer != "https://new.example.com" {
		t.Errorf("expected OIDCIssuer 'https://new.example.com', got %s", found.OIDCIssuer)
	}
}
