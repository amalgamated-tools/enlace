package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
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

func TestAuthService_Register_FirstUserIsAdmin(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// First user should be admin
	first, err := svc.Register(ctx, "first@example.com", "password123", "First User")
	if err != nil {
		t.Fatalf("failed to register first user: %v", err)
	}
	if !first.IsAdmin {
		t.Error("expected first user to be admin")
	}

	// Second user should not be admin
	second, err := svc.Register(ctx, "second@example.com", "password123", "Second User")
	if err != nil {
		t.Fatalf("failed to register second user: %v", err)
	}
	if second.IsAdmin {
		t.Error("expected second user to not be admin")
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

	if claims.TokenType != service.TokenTypeAccess {
		t.Errorf("expected token type %s, got %s", service.TokenTypeAccess, claims.TokenType)
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

func TestAuthService_RefreshTokens_RejectsAccessToken(t *testing.T) {
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

	// Attempt to refresh using an access token instead of a refresh token
	_, err = svc.RefreshTokens(ctx, tokens.AccessToken)
	if err != service.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken when using access token for refresh, got %v", err)
	}
}

func TestAuthService_RefreshTokens_RefreshTokenHasCorrectType(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	tokens, err := svc.Login(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	claims, err := svc.ValidateToken(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}

	if claims.TokenType != service.TokenTypeRefresh {
		t.Errorf("expected refresh token to have token type %q, got %q", service.TokenTypeRefresh, claims.TokenType)
	}
}

func TestAuthService_RefreshTokens_RejectsUnknownTokenType(t *testing.T) {
	ctx := context.Background()
	jwtSecret := []byte("test-secret-key-for-jwt-signing")

	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewAuthService(userRepo, jwtSecret)

	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	tests := []struct {
		name      string
		tokenType string
	}{
		{"empty token type", ""},
		{"unknown token type", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			claims := &jwt.MapClaims{
				"uid": user.ID,
				"adm": false,
				"exp": jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
				"iat": jwt.NewNumericDate(now),
				"nbf": jwt.NewNumericDate(now),
			}
			if tt.tokenType != "" {
				(*claims)["token_type"] = tt.tokenType
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			tokenStr, err := token.SignedString(jwtSecret)
			if err != nil {
				t.Fatalf("failed to sign token: %v", err)
			}

			_, err = svc.RefreshTokens(ctx, tokenStr)
			if err != service.ErrInvalidToken {
				t.Errorf("expected ErrInvalidToken for %s, got %v", tt.name, err)
			}
		})
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

func TestAuthService_UpdatePassword_OIDCFieldsPreserved(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewAuthService(userRepo, []byte("test-secret-key-for-jwt-signing"))

	ctx := context.Background()

	// Create a user with OIDC fields and a password
	oidcUser := &model.User{
		ID:          "oidc-pwd-user",
		Email:       "oidc-pwd@example.com",
		DisplayName: "OIDC Pwd User",
		OIDCSubject: "subject-456",
		OIDCIssuer:  "https://issuer.example.com",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	// Set a password hash so UpdatePassword can verify the old password
	hash, err := bcrypt.GenerateFromPassword([]byte("oldpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	oidcUser.PasswordHash = string(hash)
	if err := userRepo.Create(ctx, oidcUser); err != nil {
		t.Fatalf("failed to create OIDC user: %v", err)
	}

	// Update password
	if err := svc.UpdatePassword(ctx, oidcUser.ID, "oldpassword", "newpassword"); err != nil {
		t.Fatalf("failed to update password: %v", err)
	}

	// Verify OIDC fields are preserved
	fetched, err := userRepo.GetByID(ctx, oidcUser.ID)
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if fetched.OIDCSubject != "subject-456" {
		t.Errorf("expected OIDCSubject 'subject-456', got %s", fetched.OIDCSubject)
	}
	if fetched.OIDCIssuer != "https://issuer.example.com" {
		t.Errorf("expected OIDCIssuer 'https://issuer.example.com', got %s", fetched.OIDCIssuer)
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

	// Register a regular user (register a dummy first so this one isn't auto-admin)
	_, err = svc.Register(ctx, "first@example.com", "password123", "First User")
	if err != nil {
		t.Fatalf("failed to register first user: %v", err)
	}

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

func TestAuthService_UpdateProfile(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register
	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Update display name only
	updated, err := svc.UpdateProfile(ctx, user.ID, "New Name", "")
	if err != nil {
		t.Fatalf("failed to update profile: %v", err)
	}
	if updated.DisplayName != "New Name" {
		t.Errorf("expected display name 'New Name', got %s", updated.DisplayName)
	}
	if updated.Email != "test@example.com" {
		t.Errorf("expected email to remain test@example.com, got %s", updated.Email)
	}
}

func TestAuthService_UpdateProfile_EmailChange(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Update email
	updated, err := svc.UpdateProfile(ctx, user.ID, "", "new@example.com")
	if err != nil {
		t.Fatalf("failed to update profile: %v", err)
	}
	if updated.Email != "new@example.com" {
		t.Errorf("expected email 'new@example.com', got %s", updated.Email)
	}
	if updated.DisplayName != "Test User" {
		t.Errorf("expected display name to remain 'Test User', got %s", updated.DisplayName)
	}
}

func TestAuthService_UpdateProfile_EmailConflict(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register two users
	user1, err := svc.Register(ctx, "user1@example.com", "password123", "User 1")
	if err != nil {
		t.Fatalf("failed to register user1: %v", err)
	}
	_, err = svc.Register(ctx, "user2@example.com", "password123", "User 2")
	if err != nil {
		t.Fatalf("failed to register user2: %v", err)
	}

	// Try to change user1's email to user2's email
	_, err = svc.UpdateProfile(ctx, user1.ID, "", "user2@example.com")
	if err != service.ErrEmailExists {
		t.Errorf("expected ErrEmailExists, got %v", err)
	}
}

func TestAuthService_UpdateProfile_SameEmail(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Update with same email should succeed (no conflict)
	updated, err := svc.UpdateProfile(ctx, user.ID, "", "test@example.com")
	if err != nil {
		t.Fatalf("failed to update profile with same email: %v", err)
	}
	if updated.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %s", updated.Email)
	}
}

func TestAuthService_UpdateProfile_UserNotFound(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.UpdateProfile(ctx, "nonexistent-id", "Name", "email@example.com")
	if err != service.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestAuthService_UpdateProfile_BothFields(t *testing.T) {
	svc, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := svc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Update both display name and email
	updated, err := svc.UpdateProfile(ctx, user.ID, "Updated Name", "updated@example.com")
	if err != nil {
		t.Fatalf("failed to update profile: %v", err)
	}
	if updated.DisplayName != "Updated Name" {
		t.Errorf("expected display name 'Updated Name', got %s", updated.DisplayName)
	}
	if updated.Email != "updated@example.com" {
		t.Errorf("expected email 'updated@example.com', got %s", updated.Email)
	}
}

func TestAuthService_UpdateProfile_OIDCFieldsPreserved(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewAuthService(userRepo, []byte("test-secret-key-for-jwt-signing"))

	ctx := context.Background()

	// Create a user with OIDC fields set
	oidcUser := &model.User{
		ID:          "oidc-user-id",
		Email:       "oidc@example.com",
		DisplayName: "OIDC User",
		OIDCSubject: "subject-123",
		OIDCIssuer:  "https://issuer.example.com",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := userRepo.Create(ctx, oidcUser); err != nil {
		t.Fatalf("failed to create OIDC user: %v", err)
	}

	// Update the profile display name
	updated, err := svc.UpdateProfile(ctx, oidcUser.ID, "New OIDC Name", "")
	if err != nil {
		t.Fatalf("failed to update profile: %v", err)
	}

	if updated.DisplayName != "New OIDC Name" {
		t.Errorf("expected display name 'New OIDC Name', got %s", updated.DisplayName)
	}
	if updated.OIDCSubject != "subject-123" {
		t.Errorf("expected OIDCSubject 'subject-123', got %s", updated.OIDCSubject)
	}
	if updated.OIDCIssuer != "https://issuer.example.com" {
		t.Errorf("expected OIDCIssuer 'https://issuer.example.com', got %s", updated.OIDCIssuer)
	}

	// Verify persistence by re-fetching
	fetched, err := userRepo.GetByID(ctx, oidcUser.ID)
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if fetched.OIDCSubject != "subject-123" {
		t.Errorf("persisted OIDCSubject: expected 'subject-123', got %s", fetched.OIDCSubject)
	}
	if fetched.OIDCIssuer != "https://issuer.example.com" {
		t.Errorf("persisted OIDCIssuer: expected 'https://issuer.example.com', got %s", fetched.OIDCIssuer)
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
