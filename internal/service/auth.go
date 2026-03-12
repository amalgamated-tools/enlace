package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

// Sentinel errors for authentication operations.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUserNotFound       = errors.New("user not found")
)

// Token expiration durations.
const (
	accessTokenExpiry  = 15 * time.Minute
	refreshTokenExpiry = 7 * 24 * time.Hour
	bcryptCost         = 12
)

// TokenPair represents an access and refresh token pair.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Token type constants for distinguishing access and refresh tokens.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims represents the JWT claims used for all token types (access, refresh, and pending 2FA).
type Claims struct {
	UserID    string `json:"uid"`
	IsAdmin   bool   `json:"adm"`
	TFA       bool   `json:"tfa,omitempty"`        // true for pending 2FA tokens
	TokenType string `json:"token_type,omitempty"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// AuthService handles user authentication operations.
type AuthService struct {
	userRepo  *repository.UserRepository
	jwtSecret []byte
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(userRepo *repository.UserRepository, jwtSecret []byte) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*model.User, error) {
	// Check if email already exists
	exists, err := s.userRepo.EmailExists(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailExists
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}

	// Auto-admin: first user in the database becomes admin
	count, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(passwordHash),
		DisplayName:  displayName,
		IsAdmin:      count == 0,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns a token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.generateTokenPair(user.ID, user.IsAdmin)
}

// ValidateToken validates an access token and returns the claims.
func (s *AuthService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		// Ensure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshTokens validates a refresh token and generates new token pairs.
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Only allow refresh tokens; reject access tokens and tokens without a valid type
	if claims.TokenType != TokenTypeRefresh {
		return nil, ErrInvalidToken
	}

	// Verify user still exists
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	return s.generateTokenPair(user.ID, user.IsAdmin)
}

// GetUser retrieves a user by their ID.
func (s *AuthService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// GetUserByEmail retrieves a user by their email address.
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// UpdateProfile updates a user's profile (display_name and/or email).
// Empty string values are ignored (no update for that field).
func (s *AuthService) UpdateProfile(ctx context.Context, userID, displayName, email string) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Create new user object with updates (immutability)
	updatedUser := &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		DisplayName:  user.DisplayName,
		IsAdmin:      user.IsAdmin,
		OIDCSubject:  user.OIDCSubject,
		OIDCIssuer:   user.OIDCIssuer,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}

	// Apply updates only for non-empty values
	if displayName != "" {
		updatedUser.DisplayName = displayName
	}
	if email != "" {
		// Check if email is being changed and already exists
		if email != user.Email {
			exists, err := s.userRepo.EmailExists(ctx, email)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, ErrEmailExists
			}
		}
		updatedUser.Email = email
	}

	if err := s.userRepo.Update(ctx, updatedUser); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return updatedUser, nil
}

// UpdatePassword updates a user's password after verifying the old password.
func (s *AuthService) UpdatePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}

	// Create new user object with updated password (immutability)
	updatedUser := &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: string(newHash),
		DisplayName:  user.DisplayName,
		IsAdmin:      user.IsAdmin,
		OIDCSubject:  user.OIDCSubject,
		OIDCIssuer:   user.OIDCIssuer,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}

	return s.userRepo.Update(ctx, updatedUser)
}

// GenerateTokensForUser creates a token pair for a given user ID.
// Used by OIDC flow where we already have verified user identity.
func (s *AuthService) GenerateTokensForUser(userID string, isAdmin bool) (*TokenPair, error) {
	return s.generateTokenPair(userID, isAdmin)
}

// VerifyPassword verifies a user's password by their user ID.
func (s *AuthService) VerifyPassword(ctx context.Context, userID, password string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	return nil
}

// generateTokenPair creates a new access and refresh token pair.
func (s *AuthService) generateTokenPair(userID string, isAdmin bool) (*TokenPair, error) {
	accessToken, err := s.generateAccessToken(userID, isAdmin)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(userID, isAdmin)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateAccessToken creates a new JWT access token.
func (s *AuthService) generateAccessToken(userID string, isAdmin bool) (string, error) {
	return s.GenerateAccessTokenWithExpiry(userID, isAdmin, accessTokenExpiry)
}

// GenerateAccessTokenWithExpiry creates a JWT access token with a custom expiry duration.
// This is exposed for testing expired token scenarios.
func (s *AuthService) GenerateAccessTokenWithExpiry(userID string, isAdmin bool, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		IsAdmin:   isAdmin,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// generateRefreshToken creates a new JWT refresh token with longer expiry.
func (s *AuthService) generateRefreshToken(userID string, isAdmin bool) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		IsAdmin:   isAdmin,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// generateTokenID creates a unique identifier for a token.
func generateTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fall back to UUID if random fails
		return uuid.NewString()
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
