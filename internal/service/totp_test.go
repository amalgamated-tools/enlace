package service_test

import (
	"context"
	"encoding/base32"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
)

var testJWTSecret = []byte("test-secret-key-for-jwt-signing")

func setupTOTPService(t *testing.T) (*service.TOTPService, *service.AuthService, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	totpRepo := repository.NewTOTPRepository(db.DB())
	authService := service.NewAuthService(userRepo, testJWTSecret)
	totpService := service.NewTOTPService(totpRepo, userRepo, testJWTSecret)

	return totpService, authService, func() { db.Close() }
}

func createTestUser(t *testing.T, authSvc *service.AuthService) string {
	t.Helper()
	ctx := context.Background()
	user, err := authSvc.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register test user: %v", err)
	}
	return user.ID
}

func setupAndConfirmTOTP(t *testing.T, svc *service.TOTPService, userID string) (secret string, recoveryCodes []string) {
	t.Helper()
	ctx := context.Background()

	secret, _, _, err := svc.BeginSetup(ctx, userID)
	if err != nil {
		t.Fatalf("failed to begin TOTP setup: %v", err)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}

	recoveryCodes, err = svc.ConfirmSetup(ctx, userID, code)
	if err != nil {
		t.Fatalf("failed to confirm TOTP setup: %v", err)
	}

	return secret, recoveryCodes
}

func TestTOTPService_BeginSetup(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	secret, qrCode, provisioningURI, err := svc.BeginSetup(ctx, userID)
	if err != nil {
		t.Fatalf("failed to begin TOTP setup: %v", err)
	}

	if secret == "" {
		t.Error("expected secret to be non-empty")
	}
	if qrCode == "" {
		t.Error("expected QR code to be non-empty")
	}
	if provisioningURI == "" {
		t.Error("expected provisioning URI to be non-empty")
	}
}

func TestTOTPService_BeginSetup_AlreadyEnabled(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	setupAndConfirmTOTP(t, svc, userID)

	_, _, _, err := svc.BeginSetup(ctx, userID)
	if !errors.Is(err, service.ErrTOTPAlreadyEnabled) {
		t.Errorf("expected ErrTOTPAlreadyEnabled, got %v", err)
	}
}

func TestTOTPService_ConfirmSetup(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	secret, _, _, err := svc.BeginSetup(ctx, userID)
	if err != nil {
		t.Fatalf("failed to begin TOTP setup: %v", err)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}

	recoveryCodes, err := svc.ConfirmSetup(ctx, userID, code)
	if err != nil {
		t.Fatalf("failed to confirm TOTP setup: %v", err)
	}

	if len(recoveryCodes) != 10 {
		t.Errorf("expected 10 recovery codes, got %d", len(recoveryCodes))
	}

	for i, rc := range recoveryCodes {
		if !strings.Contains(rc, "-") {
			t.Errorf("recovery code %d (%q) does not contain a dash", i, rc)
		}
	}
}

func TestTOTPService_ConfirmSetup_InvalidCode(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	_, _, _, err := svc.BeginSetup(ctx, userID)
	if err != nil {
		t.Fatalf("failed to begin TOTP setup: %v", err)
	}

	_, err = svc.ConfirmSetup(ctx, userID, "000000")
	if !errors.Is(err, service.ErrInvalidTOTPCode) {
		t.Errorf("expected ErrInvalidTOTPCode, got %v", err)
	}
}

func TestTOTPService_ConfirmSetup_NotStarted(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	_, err := svc.ConfirmSetup(ctx, userID, "123456")
	if !errors.Is(err, service.ErrTOTPNotSetup) {
		t.Errorf("expected ErrTOTPNotSetup, got %v", err)
	}
}

func TestTOTPService_Verify(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	secret, _ := setupAndConfirmTOTP(t, svc, userID)

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}

	err = svc.Verify(ctx, userID, code)
	if err != nil {
		t.Errorf("expected Verify to succeed, got %v", err)
	}
}

func TestTOTPService_Verify_InvalidCode(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	setupAndConfirmTOTP(t, svc, userID)

	err := svc.Verify(ctx, userID, "000000")
	if !errors.Is(err, service.ErrInvalidTOTPCode) {
		t.Errorf("expected ErrInvalidTOTPCode, got %v", err)
	}
}

func TestTOTPService_Verify_NotEnabled(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	err := svc.Verify(ctx, userID, "123456")
	if !errors.Is(err, service.ErrTOTPNotEnabled) {
		t.Errorf("expected ErrTOTPNotEnabled, got %v", err)
	}
}

func TestTOTPService_VerifyRecoveryCode(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	_, recoveryCodes := setupAndConfirmTOTP(t, svc, userID)

	// Use the first recovery code
	codeToUse := recoveryCodes[0]
	err := svc.VerifyRecoveryCode(ctx, userID, codeToUse)
	if err != nil {
		t.Fatalf("expected first use of recovery code to succeed, got %v", err)
	}

	// Second use of the same code should fail (consumed)
	err = svc.VerifyRecoveryCode(ctx, userID, codeToUse)
	if !errors.Is(err, service.ErrInvalidRecoveryCode) {
		t.Errorf("expected ErrInvalidRecoveryCode on second use, got %v", err)
	}
}

func TestTOTPService_VerifyRecoveryCode_Invalid(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	setupAndConfirmTOTP(t, svc, userID)

	err := svc.VerifyRecoveryCode(ctx, userID, "invalid-recovery-code")
	if !errors.Is(err, service.ErrInvalidRecoveryCode) {
		t.Errorf("expected ErrInvalidRecoveryCode, got %v", err)
	}
}

func TestTOTPService_Disable(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	setupAndConfirmTOTP(t, svc, userID)

	// Verify 2FA is enabled
	enabled, err := svc.GetStatus(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if !enabled {
		t.Fatal("expected 2FA to be enabled before disabling")
	}

	// Disable 2FA
	err = svc.Disable(ctx, userID)
	if err != nil {
		t.Fatalf("failed to disable 2FA: %v", err)
	}

	// Verify 2FA is disabled
	enabled, err = svc.GetStatus(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get status after disable: %v", err)
	}
	if enabled {
		t.Error("expected 2FA to be disabled after calling Disable")
	}
}

func TestTOTPService_RegenerateRecoveryCodes(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	_, oldCodes := setupAndConfirmTOTP(t, svc, userID)

	// Regenerate recovery codes
	newCodes, err := svc.RegenerateRecoveryCodes(ctx, userID)
	if err != nil {
		t.Fatalf("failed to regenerate recovery codes: %v", err)
	}

	if len(newCodes) != 10 {
		t.Errorf("expected 10 new recovery codes, got %d", len(newCodes))
	}

	// Old codes should no longer work
	err = svc.VerifyRecoveryCode(ctx, userID, oldCodes[0])
	if !errors.Is(err, service.ErrInvalidRecoveryCode) {
		t.Errorf("expected old recovery code to be invalid after regeneration, got %v", err)
	}

	// New codes should work
	err = svc.VerifyRecoveryCode(ctx, userID, newCodes[0])
	if err != nil {
		t.Errorf("expected new recovery code to be valid, got %v", err)
	}
}

func TestTOTPService_RegenerateRecoveryCodes_NotEnabled(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	_, err := svc.RegenerateRecoveryCodes(ctx, userID)
	if !errors.Is(err, service.ErrTOTPNotEnabled) {
		t.Errorf("expected ErrTOTPNotEnabled, got %v", err)
	}
}

func TestTOTPService_GetStatus(t *testing.T) {
	svc, authSvc, cleanup := setupTOTPService(t)
	defer cleanup()

	ctx := context.Background()
	userID := createTestUser(t, authSvc)

	// Before setup, status should be false
	enabled, err := svc.GetStatus(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if enabled {
		t.Error("expected 2FA status to be false before setup")
	}

	// After setup and confirm, status should be true
	setupAndConfirmTOTP(t, svc, userID)

	enabled, err = svc.GetStatus(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get status after setup: %v", err)
	}
	if !enabled {
		t.Error("expected 2FA status to be true after setup and confirm")
	}
}

func TestTOTPService_GeneratePendingToken(t *testing.T) {
	svc, _, cleanup := setupTOTPService(t)
	defer cleanup()

	token, err := svc.GeneratePendingToken("user-123", false)
	if err != nil {
		t.Fatalf("failed to generate pending token: %v", err)
	}

	if token == "" {
		t.Error("expected pending token to be non-empty")
	}
}

func TestTOTPService_ValidatePendingToken(t *testing.T) {
	svc, _, cleanup := setupTOTPService(t)
	defer cleanup()

	userID := "user-456"
	isAdmin := true

	token, err := svc.GeneratePendingToken(userID, isAdmin)
	if err != nil {
		t.Fatalf("failed to generate pending token: %v", err)
	}

	claims, err := svc.ValidatePendingToken(token)
	if err != nil {
		t.Fatalf("failed to validate pending token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected UserID %q, got %q", userID, claims.UserID)
	}
	if claims.IsAdmin != isAdmin {
		t.Errorf("expected IsAdmin %v, got %v", isAdmin, claims.IsAdmin)
	}
	if !claims.TFA {
		t.Error("expected TFA claim to be true")
	}
}

func TestTOTPService_ValidatePendingToken_Invalid(t *testing.T) {
	svc, _, cleanup := setupTOTPService(t)
	defer cleanup()

	_, err := svc.ValidatePendingToken("invalid-token-string")
	if !errors.Is(err, service.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestTOTPService_EncryptDecrypt(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	totpRepo := repository.NewTOTPRepository(db.DB())
	authService := service.NewAuthService(userRepo, testJWTSecret)
	totpService := service.NewTOTPService(totpRepo, userRepo, testJWTSecret)

	ctx := context.Background()

	user, err := authService.Register(ctx, "encrypt@example.com", "password123", "Encrypt User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Begin setup to get the plaintext secret
	secret, _, _, err := totpService.BeginSetup(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to begin TOTP setup: %v", err)
	}

	// Read the stored secret directly from the DB
	storedRecord, err := totpRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get TOTP record from DB: %v", err)
	}

	// The stored secret should NOT be the same as the plaintext secret (it should be encrypted)
	if storedRecord.Secret == secret {
		t.Error("expected stored secret to be encrypted (different from plaintext secret)")
	}

	// Confirm the setup works (which proves decryption works internally)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}

	_, err = totpService.ConfirmSetup(ctx, user.ID, code)
	if err != nil {
		t.Fatalf("failed to confirm setup (decrypt should work): %v", err)
	}

	// Verify also works (which again uses decryption)
	code, err = totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code for verify: %v", err)
	}

	err = totpService.Verify(ctx, user.ID, code)
	if err != nil {
		t.Errorf("expected Verify to succeed after encrypt/decrypt, got %v", err)
	}
}

func TestTOTPService_DecryptLegacyPlaintext(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	totpRepo := repository.NewTOTPRepository(db.DB())
	authService := service.NewAuthService(userRepo, testJWTSecret)
	totpService := service.NewTOTPService(totpRepo, userRepo, testJWTSecret)

	ctx := context.Background()

	user, err := authService.Register(ctx, "legacy@example.com", "password123", "Legacy User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Create a plaintext TOTP secret that is NOT valid base64.
	// The decryptSecret function treats non-base64 strings as legacy plaintext.
	// Using a 6-byte raw value encoded as base32 without padding produces a
	// 10-character string, which is not valid base64 (length not divisible by 4).
	rawSecret := []byte("legacy")
	plaintextSecret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(rawSecret)

	// Store the plaintext secret directly in the DB (simulating legacy data)
	_, err = db.DB().ExecContext(ctx,
		`INSERT INTO user_totp (user_id, secret, enabled, created_at) VALUES (?, ?, 1, ?)`,
		user.ID, plaintextSecret, time.Now(),
	)
	if err != nil {
		t.Fatalf("failed to insert legacy TOTP record: %v", err)
	}

	// Verify with a valid TOTP code should work (decryptSecret falls back to plaintext)
	code, err := totp.GenerateCode(plaintextSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}

	err = totpService.Verify(ctx, user.ID, code)
	if err != nil {
		t.Errorf("expected Verify to succeed with legacy plaintext secret, got %v", err)
	}
}
