package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/amalgamated-tools/enlace/internal/config"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

// OIDC errors
var (
	ErrOIDCDisabled         = errors.New("OIDC is not enabled")
	ErrOIDCStateMismatch    = errors.New("state mismatch")
	ErrOIDCNoEmail          = errors.New("OIDC provider did not return email")
	ErrOIDCAlreadyLinked    = errors.New("OIDC account already linked to another user")
	ErrOIDCEmailNotVerified = errors.New("OIDC provider email is not verified")
)

// OIDCUserInfo contains user information from the OIDC provider.
type OIDCUserInfo struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	Issuer        string
}

// TOTPDisabler can remove 2FA configuration for a user.
// SSO and 2FA are mutually exclusive, so 2FA is removed when OIDC is linked.
type TOTPDisabler interface {
	Disable(ctx context.Context, userID string) error
}

// OIDCService handles OIDC authentication operations.
type OIDCService struct {
	provider     *oidc.Provider
	oauth2Cfg    *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	userRepo     *repository.UserRepository
	issuerURL    string
	totpDisabler TOTPDisabler
}

// NewOIDCService creates a new OIDCService instance.
// Returns nil if OIDC is not enabled.
func NewOIDCService(cfg *config.Config, userRepo *repository.UserRepository, totpDisabler TOTPDisabler) (*OIDCService, error) {
	if !cfg.OIDCEnabled {
		return nil, nil
	}

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, err
	}

	scopes := strings.Split(cfg.OIDCScopes, " ")
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})

	return &OIDCService{
		provider:     provider,
		oauth2Cfg:    oauth2Cfg,
		verifier:     verifier,
		userRepo:     userRepo,
		issuerURL:    cfg.OIDCIssuerURL,
		totpDisabler: totpDisabler,
	}, nil
}

// GenerateState creates a secure random state token.
func (s *OIDCService) GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeVerifier creates a PKCE code verifier.
func (s *OIDCService) GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GetAuthURL returns the OIDC provider's authorization URL with PKCE.
func (s *OIDCService) GetAuthURL(state, codeVerifier string) string {
	return s.oauth2Cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", s.s256Challenge(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// GetLinkAuthURL returns the authorization URL for account linking with PKCE.
func (s *OIDCService) GetLinkAuthURL(state, codeVerifier string) string {
	return s.oauth2Cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("code_challenge", s.s256Challenge(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// ExchangeCode exchanges an authorization code for tokens and returns user info.
func (s *OIDCService) ExchangeCode(ctx context.Context, code, codeVerifier string) (*OIDCUserInfo, error) {
	token, err := s.oauth2Cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token in response")
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		PreferredName string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}

	if claims.Email == "" {
		return nil, ErrOIDCNoEmail
	}

	displayName := claims.Name
	if displayName == "" {
		displayName = claims.PreferredName
	}
	if displayName == "" {
		displayName = strings.Split(claims.Email, "@")[0]
	}

	return &OIDCUserInfo{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		DisplayName:   displayName,
		Issuer:        s.issuerURL,
	}, nil
}

// FindOrCreateUser finds an existing user or creates a new one from OIDC info.
// If a user with matching email exists, it links the OIDC identity.
func (s *OIDCService) FindOrCreateUser(ctx context.Context, info *OIDCUserInfo) (*model.User, error) {
	// First, try to find by OIDC identity
	user, err := s.userRepo.GetByOIDC(ctx, info.Issuer, info.Subject)
	if err == nil {
		// Best-effort: clean up any stale 2FA data left from a partial failure
		// during a previous link attempt where Update succeeded but disable2FA failed.
		_ = s.disable2FA(ctx, user.ID)
		return user, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	// Try to find by email (auto-link)
	user, err = s.userRepo.GetByEmail(ctx, info.Email)
	if err == nil {
		if !info.EmailVerified {
			return nil, ErrOIDCEmailNotVerified
		}
		// Link OIDC to existing user
		linkedUser := &model.User{
			ID:           user.ID,
			Email:        user.Email,
			PasswordHash: user.PasswordHash,
			DisplayName:  user.DisplayName,
			IsAdmin:      user.IsAdmin,
			OIDCSubject:  info.Subject,
			OIDCIssuer:   info.Issuer,
			CreatedAt:    user.CreatedAt,
		}
		if err := s.userRepo.Update(ctx, linkedUser); err != nil {
			return nil, err
		}
		// SSO and 2FA are mutually exclusive — remove any existing 2FA
		if err := s.disable2FA(ctx, user.ID); err != nil {
			return nil, fmt.Errorf("failed to remove 2FA for OIDC-linked user: %w", err)
		}
		return linkedUser, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	// Create new user
	// Auto-admin: first user in the database becomes admin
	count, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	newUser := &model.User{
		ID:          uuid.NewString(),
		Email:       info.Email,
		DisplayName: info.DisplayName,
		IsAdmin:     count == 0,
		OIDCSubject: info.Subject,
		OIDCIssuer:  info.Issuer,
	}
	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, err
	}
	return newUser, nil
}

// LinkOIDC links an OIDC identity to an existing user.
func (s *OIDCService) LinkOIDC(ctx context.Context, userID string, info *OIDCUserInfo) error {
	// Check if OIDC identity is already linked to another user
	existing, err := s.userRepo.GetByOIDC(ctx, info.Issuer, info.Subject)
	if err == nil && existing.ID != userID {
		return ErrOIDCAlreadyLinked
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}

	// Get current user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Link OIDC
	linkedUser := &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		DisplayName:  user.DisplayName,
		IsAdmin:      user.IsAdmin,
		OIDCSubject:  info.Subject,
		OIDCIssuer:   info.Issuer,
		CreatedAt:    user.CreatedAt,
	}
	if err := s.userRepo.Update(ctx, linkedUser); err != nil {
		return err
	}

	// SSO and 2FA are mutually exclusive — remove any existing 2FA
	if err := s.disable2FA(ctx, userID); err != nil {
		return fmt.Errorf("failed to remove 2FA for OIDC-linked user: %w", err)
	}

	return nil
}

// UnlinkOIDC removes OIDC identity from a user.
// Fails if user has no password (would lock them out).
func (s *OIDCService) UnlinkOIDC(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.PasswordHash == "" {
		return errors.New("cannot unlink OIDC from account without password")
	}

	unlinkedUser := &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		DisplayName:  user.DisplayName,
		IsAdmin:      user.IsAdmin,
		OIDCSubject:  "",
		OIDCIssuer:   "",
		CreatedAt:    user.CreatedAt,
	}
	return s.userRepo.Update(ctx, unlinkedUser)
}

// disable2FA removes 2FA configuration for a user if a TOTPDisabler is configured.
// This is a no-op if no TOTPDisabler was provided.
func (s *OIDCService) disable2FA(ctx context.Context, userID string) error {
	if s.totpDisabler == nil {
		return nil
	}
	return s.totpDisabler.Disable(ctx, userID)
}

// IsEnabled returns whether OIDC is enabled.
func (s *OIDCService) IsEnabled() bool {
	return s != nil
}

// s256Challenge computes the S256 PKCE code challenge from a verifier.
func (s *OIDCService) s256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
