package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/amalgamated-tools/sharer/internal/database"
	"github.com/amalgamated-tools/sharer/internal/repository"
	"github.com/amalgamated-tools/sharer/internal/service"
)

func setupAuthService(t *testing.T) (*service.AuthService, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	authService := service.NewAuthService(userRepo, []byte("test-secret-key-for-jwt-signing"))

	return authService, func() { db.Close() }
}

func TestAuthService_Register(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.DisplayName != "Test User" {
		t.Errorf("expected display name Test User, got %s", user.DisplayName)
	}
	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
	// Password should be hashed, not plain
	if user.PasswordHash == "password123" {
		t.Error("password should be hashed, not stored in plain text")
	}
	if user.PasswordHash == "" {
		t.Error("password hash should not be empty")
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register first user: %v", err)
	}

	_, err = svc.Register(ctx, "test@example.com", "password456", "Another User")
	if err != service.ErrEmailExists {
		t.Errorf("expected ErrEmailExists, got %v", err)
	}
}

func TestAuthService_Login(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register first
	_, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Login
	tokens, err := svc.Login(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("expected access token to be set")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected refresh token to be set")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register first
	_, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Try login with wrong password
	_, err = svc.Login(ctx, "test@example.com", "wrongpassword")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.Login(ctx, "nonexistent@example.com", "password123")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ValidateToken(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register and login
	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	tokens, err := svc.Login(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	// Validate access token
	claims, err := svc.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, claims.UserID)
	}
}

func TestAuthService_ValidateToken_Invalid(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	_, err := svc.ValidateToken("invalid-token")
	if err != service.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthService_ValidateToken_ExpiredToken(t *testing.T) {
	// Create service with ability to generate expired tokens for testing
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewAuthService(userRepo, []byte("test-secret-key-for-jwt-signing"))

	ctx := context.Background()

	// Register user
	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Generate expired token using internal method
	expiredToken, err := svc.GenerateAccessTokenWithExpiry(user.ID, user.IsAdmin, -1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	_, err = svc.ValidateToken(expiredToken)
	if err != service.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestAuthService_RefreshTokens(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register and login
	_, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	tokens, err := svc.Login(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	// Refresh tokens
	newTokens, err := svc.RefreshTokens(ctx, tokens.RefreshToken)
	if err != nil {
		t.Fatalf("failed to refresh tokens: %v", err)
	}

	if newTokens.AccessToken == "" {
		t.Error("expected new access token to be set")
	}
	if newTokens.RefreshToken == "" {
		t.Error("expected new refresh token to be set")
	}
	// New tokens should be different from old ones
	if newTokens.AccessToken == tokens.AccessToken {
		t.Error("expected new access token to be different from old one")
	}
}

func TestAuthService_RefreshTokens_InvalidToken(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.RefreshTokens(ctx, "invalid-refresh-token")
	if err != service.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthService_GetUser(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register
	registeredUser, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Get user
	user, err := svc.GetUser(ctx, registeredUser.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if user.ID != registeredUser.ID {
		t.Errorf("expected user ID %s, got %s", registeredUser.ID, user.ID)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
}

func TestAuthService_GetUser_NotFound(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.GetUser(ctx, "nonexistent-user-id")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestAuthService_UpdatePassword(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register
	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Update password
	err = svc.UpdatePassword(ctx, user.ID, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("failed to update password: %v", err)
	}

	// Login with new password should work
	_, err = svc.Login(ctx, "test@example.com", "newpassword456")
	if err != nil {
		t.Fatalf("failed to login with new password: %v", err)
	}

	// Login with old password should fail
	_, err = svc.Login(ctx, "test@example.com", "password123")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials when using old password, got %v", err)
	}
}

func TestAuthService_UpdatePassword_WrongOldPassword(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register
	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Try to update password with wrong old password
	err = svc.UpdatePassword(ctx, user.ID, "wrongpassword", "newpassword456")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_UpdatePassword_UserNotFound(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	err := svc.UpdatePassword(ctx, "nonexistent-user-id", "oldpassword", "newpassword")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestAuthService_TokenClaimsContainAdminStatus(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewAuthService(userRepo, []byte("test-secret-key"))

	ctx := context.Background()

	// Register a regular user
	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Login and check claims
	tokens, err := svc.Login(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	claims, err := svc.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, claims.UserID)
	}
	if claims.IsAdmin != false {
		t.Errorf("expected IsAdmin to be false, got %v", claims.IsAdmin)
	}
}

func TestAuthService_GenerateTokensForUser(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	tokens, err := svc.GenerateTokensForUser("user-123", false)
	if err != nil {
		t.Fatalf("failed to generate tokens: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}

	// Verify token is valid
	claims, err := svc.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected user ID 'user-123', got %s", claims.UserID)
	}
}
