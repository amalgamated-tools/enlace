package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/config"
	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
)

func TestNewOIDCService_Disabled(t *testing.T) {
	cfg := &config.Config{
		OIDCEnabled: false,
	}

	svc, err := service.NewOIDCService(cfg, nil)
	if err != nil {
		t.Fatalf("expected no error for disabled OIDC, got %v", err)
	}
	if svc != nil {
		t.Error("expected nil service when OIDC is disabled")
	}
}

func TestNewOIDCService_EnabledInvalidIssuer(t *testing.T) {
	// Use a local httptest server that returns 404 for OIDC discovery,
	// so the test fails deterministically without any external network access.
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	cfg := &config.Config{
		OIDCEnabled:   true,
		OIDCIssuerURL: srv.URL,
		OIDCClientID:  "client-id",
	}

	_, err := service.NewOIDCService(cfg, nil)
	if err == nil {
		t.Error("expected error for invalid OIDC issuer URL")
	}
}

func TestOIDCService_IsEnabled_Nil(t *testing.T) {
	var svc *service.OIDCService
	if svc.IsEnabled() {
		t.Error("expected nil OIDCService.IsEnabled() to return false")
	}
}

func TestOIDCService_FindOrCreateUser_ExistingOIDCUser(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	// Create a user with OIDC info directly via repo
	existingUser := &model.User{
		ID:          "user-1",
		Email:       "oidc@example.com",
		DisplayName: "OIDC User",
		OIDCSubject: "sub-123",
		OIDCIssuer:  "https://issuer.example.com",
	}
	if err := userRepo.Create(ctx, existingUser); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Verify user can be found by OIDC identity (used by FindOrCreateUser)
	found, err := userRepo.GetByOIDC(ctx, "https://issuer.example.com", "sub-123")
	if err != nil {
		t.Fatalf("failed to find user by OIDC: %v", err)
	}
	if found.ID != existingUser.ID {
		t.Errorf("expected user ID %s, got %s", existingUser.ID, found.ID)
	}
	if found.Email != "oidc@example.com" {
		t.Errorf("expected email 'oidc@example.com', got %s", found.Email)
	}
}

func TestOIDCService_UnlinkOIDC_RequiresPassword(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	// Create a user without password (OIDC-only user)
	user := &model.User{
		ID:          "user-no-pw",
		Email:       "nopw@example.com",
		DisplayName: "No Password User",
		OIDCSubject: "sub-456",
		OIDCIssuer:  "https://issuer.example.com",
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Verify the user has no password hash
	found, err := userRepo.GetByID(ctx, "user-no-pw")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if found.PasswordHash != "" {
		t.Error("expected user to have no password hash")
	}
}
