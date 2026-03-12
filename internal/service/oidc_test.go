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

	svc, err := service.NewOIDCService(cfg, nil, nil)
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

	_, err := service.NewOIDCService(cfg, nil, nil)
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
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", nil)
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

	// Call the service method — should find the existing OIDC-linked user
	found, err := svc.FindOrCreateUser(ctx, &service.OIDCUserInfo{
		Subject:     "sub-123",
		Email:       "oidc@example.com",
		DisplayName: "OIDC User",
		Issuer:      "https://issuer.example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser failed: %v", err)
	}
	if found.ID != existingUser.ID {
		t.Errorf("expected user ID %s, got %s", existingUser.ID, found.ID)
	}
	if found.Email != "oidc@example.com" {
		t.Errorf("expected email 'oidc@example.com', got %s", found.Email)
	}
}

func TestOIDCService_FindOrCreateUser_AutoLinkByEmail(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", nil)
	ctx := context.Background()

	// Create a user without OIDC linkage
	existingUser := &model.User{
		ID:          "user-email-only",
		Email:       "existing@example.com",
		DisplayName: "Existing User",
	}
	if err := userRepo.Create(ctx, existingUser); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// FindOrCreateUser should auto-link OIDC to the existing email user
	found, err := svc.FindOrCreateUser(ctx, &service.OIDCUserInfo{
		Subject:       "sub-new",
		Email:         "existing@example.com",
		EmailVerified: true,
		DisplayName:   "Existing User",
		Issuer:        "https://issuer.example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser failed: %v", err)
	}
	if found.ID != existingUser.ID {
		t.Errorf("expected user ID %s, got %s", existingUser.ID, found.ID)
	}
	if found.OIDCSubject != "sub-new" {
		t.Errorf("expected OIDCSubject 'sub-new', got %s", found.OIDCSubject)
	}
	if found.OIDCIssuer != "https://issuer.example.com" {
		t.Errorf("expected OIDCIssuer 'https://issuer.example.com', got %s", found.OIDCIssuer)
	}
}

func TestOIDCService_FindOrCreateUser_UnverifiedEmailDoesNotAutoLink(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", nil)
	ctx := context.Background()

	existingUser := &model.User{
		ID:          "user-email-only",
		Email:       "existing@example.com",
		DisplayName: "Existing User",
	}
	if err := userRepo.Create(ctx, existingUser); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	_, err = svc.FindOrCreateUser(ctx, &service.OIDCUserInfo{
		Subject:       "sub-new",
		Email:         "existing@example.com",
		EmailVerified: false,
		DisplayName:   "Existing User",
		Issuer:        "https://issuer.example.com",
	})
	if err == nil {
		t.Fatal("expected error for unverified email auto-link")
	}
	if !errors.Is(err, service.ErrOIDCEmailNotVerified) {
		t.Fatalf("expected ErrOIDCEmailNotVerified, got %v", err)
	}

	found, err := userRepo.GetByID(ctx, existingUser.ID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if found.OIDCSubject != "" {
		t.Errorf("expected OIDCSubject to remain empty, got %s", found.OIDCSubject)
	}
	if found.OIDCIssuer != "" {
		t.Errorf("expected OIDCIssuer to remain empty, got %s", found.OIDCIssuer)
	}
}

func TestOIDCService_FindOrCreateUser_NewUser(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", nil)
	ctx := context.Background()

	// FindOrCreateUser should create a brand new user
	created, err := svc.FindOrCreateUser(ctx, &service.OIDCUserInfo{
		Subject:     "sub-brand-new",
		Email:       "brand-new@example.com",
		DisplayName: "Brand New User",
		Issuer:      "https://issuer.example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty user ID for new user")
	}
	if created.Email != "brand-new@example.com" {
		t.Errorf("expected email 'brand-new@example.com', got %s", created.Email)
	}
	if created.OIDCSubject != "sub-brand-new" {
		t.Errorf("expected OIDCSubject 'sub-brand-new', got %s", created.OIDCSubject)
	}
	if created.OIDCIssuer != "https://issuer.example.com" {
		t.Errorf("expected OIDCIssuer 'https://issuer.example.com', got %s", created.OIDCIssuer)
	}
}

func TestOIDCService_FindOrCreateUser_FirstUserIsAdmin(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", nil)
	ctx := context.Background()

	// First OIDC user should be admin
	first, err := svc.FindOrCreateUser(ctx, &service.OIDCUserInfo{
		Subject:     "sub-first",
		Email:       "first@example.com",
		DisplayName: "First User",
		Issuer:      "https://issuer.example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser failed: %v", err)
	}
	if !first.IsAdmin {
		t.Error("expected first OIDC user to be admin")
	}

	// Second OIDC user should not be admin
	second, err := svc.FindOrCreateUser(ctx, &service.OIDCUserInfo{
		Subject:     "sub-second",
		Email:       "second@example.com",
		DisplayName: "Second User",
		Issuer:      "https://issuer.example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser failed: %v", err)
	}
	if second.IsAdmin {
		t.Error("expected second OIDC user to not be admin")
	}
}

func TestOIDCService_UnlinkOIDC_RequiresPassword(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", nil)
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

	// UnlinkOIDC should fail because user has no password
	err = svc.UnlinkOIDC(ctx, "user-no-pw")
	if err == nil {
		t.Error("expected error when unlinking OIDC from user without password")
	}
}

func TestOIDCService_UnlinkOIDC_WithPassword(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", nil)
	ctx := context.Background()

	// Create a user with both password and OIDC linkage
	user := &model.User{
		ID:           "user-with-pw",
		Email:        "withpw@example.com",
		PasswordHash: "hashed-password",
		DisplayName:  "Password User",
		OIDCSubject:  "sub-789",
		OIDCIssuer:   "https://issuer.example.com",
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// UnlinkOIDC should succeed
	if err := svc.UnlinkOIDC(ctx, "user-with-pw"); err != nil {
		t.Fatalf("UnlinkOIDC failed: %v", err)
	}

	// Verify OIDC fields were cleared
	found, err := userRepo.GetByID(ctx, "user-with-pw")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if found.OIDCSubject != "" {
		t.Errorf("expected empty OIDCSubject, got %s", found.OIDCSubject)
	}
	if found.OIDCIssuer != "" {
		t.Errorf("expected empty OIDCIssuer, got %s", found.OIDCIssuer)
	}
}

// mockTOTPDisabler records calls to Disable for test assertions.
type mockTOTPDisabler struct {
	disabledUsers []string
}

func (m *mockTOTPDisabler) Disable(ctx context.Context, userID string) error {
	m.disabledUsers = append(m.disabledUsers, userID)
	return nil
}

func TestOIDCService_FindOrCreateUser_AutoLinkRemoves2FA(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	disabler := &mockTOTPDisabler{}
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", disabler)
	ctx := context.Background()

	// Create a user without OIDC linkage (has email only)
	existingUser := &model.User{
		ID:          "user-with-2fa",
		Email:       "twofa@example.com",
		DisplayName: "2FA User",
	}
	if err := userRepo.Create(ctx, existingUser); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Auto-link via email should remove 2FA
	_, err = svc.FindOrCreateUser(ctx, &service.OIDCUserInfo{
		Subject:       "sub-new",
		Email:         "twofa@example.com",
		EmailVerified: true,
		DisplayName:   "2FA User",
		Issuer:        "https://issuer.example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser failed: %v", err)
	}

	if len(disabler.disabledUsers) != 1 {
		t.Fatalf("expected 1 Disable call, got %d", len(disabler.disabledUsers))
	}
	if disabler.disabledUsers[0] != "user-with-2fa" {
		t.Errorf("expected Disable for user-with-2fa, got %s", disabler.disabledUsers[0])
	}
}

func TestOIDCService_LinkOIDC_Removes2FA(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	disabler := &mockTOTPDisabler{}
	svc := service.NewOIDCServiceForTest(userRepo, "https://issuer.example.com", disabler)
	ctx := context.Background()

	// Create a user with a password (can link OIDC)
	user := &model.User{
		ID:           "user-link-2fa",
		Email:        "link@example.com",
		PasswordHash: "hashed-password",
		DisplayName:  "Link User",
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Link OIDC should remove 2FA
	err = svc.LinkOIDC(ctx, "user-link-2fa", &service.OIDCUserInfo{
		Subject:     "sub-link",
		Email:       "link@example.com",
		DisplayName: "Link User",
		Issuer:      "https://issuer.example.com",
	})
	if err != nil {
		t.Fatalf("LinkOIDC failed: %v", err)
	}

	if len(disabler.disabledUsers) != 1 {
		t.Fatalf("expected 1 Disable call, got %d", len(disabler.disabledUsers))
	}
	if disabler.disabledUsers[0] != "user-link-2fa" {
		t.Errorf("expected Disable for user-link-2fa, got %s", disabler.disabledUsers[0])
	}
}
