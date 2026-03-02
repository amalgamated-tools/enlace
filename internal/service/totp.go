package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

// Sentinel errors for TOTP operations.
var (
	ErrTOTPAlreadyEnabled  = errors.New("2FA is already enabled")
	ErrTOTPNotEnabled      = errors.New("2FA is not enabled")
	ErrTOTPNotSetup        = errors.New("2FA setup not started")
	ErrInvalidTOTPCode     = errors.New("invalid 2FA code")
	ErrInvalidRecoveryCode = errors.New("invalid recovery code")
)

const (
	recoveryCodeCount  = 10
	recoveryCodeBytes  = 4 // 4 bytes = 8 hex characters
	recoveryBcryptCost = 10
	totpIssuer         = "Enlace"
	pendingTokenExpiry = 5 * time.Minute
)

// TOTPService handles TOTP two-factor authentication operations.
type TOTPService struct {
	totpRepo  *repository.TOTPRepository
	userRepo  *repository.UserRepository
	jwtSecret []byte
}

// NewTOTPService creates a new TOTPService instance.
func NewTOTPService(totpRepo *repository.TOTPRepository, userRepo *repository.UserRepository, jwtSecret []byte) *TOTPService {
	return &TOTPService{
		totpRepo:  totpRepo,
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

// BeginSetup generates a new TOTP secret for the user and returns the provisioning URI
// and a base64-encoded QR code PNG.
func (s *TOTPService) BeginSetup(ctx context.Context, userID string) (string, string, string, error) {
	// Check if 2FA is already enabled
	existing, err := s.totpRepo.GetByUserID(ctx, userID)
	if err == nil && existing.Enabled {
		return "", "", "", ErrTOTPAlreadyEnabled
	}

	// Get user email for the TOTP account name
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get user: %w", err)
	}

	// Generate TOTP key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: user.Email,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// Generate QR code as base64 PNG
	png, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate QR code: %w", err)
	}
	qrBase64 := base64.StdEncoding.EncodeToString(png)

	// Store the secret (not yet enabled)
	totpRecord := &model.UserTOTP{
		UserID:  userID,
		Secret:  key.Secret(),
		Enabled: false,
	}
	if err := s.totpRepo.UpsertTOTP(ctx, totpRecord); err != nil {
		return "", "", "", fmt.Errorf("failed to save TOTP secret: %w", err)
	}

	return key.Secret(), qrBase64, key.URL(), nil
}

// ConfirmSetup verifies the TOTP code against the pending secret,
// enables 2FA, generates recovery codes, and returns the plain codes.
func (s *TOTPService) ConfirmSetup(ctx context.Context, userID, code string) ([]string, error) {
	totpRecord, err := s.totpRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTOTPNotSetup
		}
		return nil, err
	}

	if totpRecord.Enabled {
		return nil, ErrTOTPAlreadyEnabled
	}

	// Validate the TOTP code
	if !totp.Validate(code, totpRecord.Secret) {
		return nil, ErrInvalidTOTPCode
	}

	// Enable 2FA
	if err := s.totpRepo.Enable(ctx, userID); err != nil {
		return nil, fmt.Errorf("failed to enable 2FA: %w", err)
	}

	// Generate recovery codes
	codes, plainCodes, err := s.generateRecoveryCodes(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recovery codes: %w", err)
	}

	if err := s.totpRepo.SaveRecoveryCodes(ctx, codes); err != nil {
		return nil, fmt.Errorf("failed to save recovery codes: %w", err)
	}

	return plainCodes, nil
}

// Verify validates a TOTP code for an enabled user.
func (s *TOTPService) Verify(ctx context.Context, userID, code string) error {
	totpRecord, err := s.totpRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTOTPNotEnabled
		}
		return err
	}

	if !totpRecord.Enabled {
		return ErrTOTPNotEnabled
	}

	if !totp.Validate(code, totpRecord.Secret) {
		return ErrInvalidTOTPCode
	}

	return nil
}

// VerifyRecoveryCode checks a recovery code and consumes it (single-use).
func (s *TOTPService) VerifyRecoveryCode(ctx context.Context, userID, code string) error {
	codes, err := s.totpRepo.GetRecoveryCodes(ctx, userID)
	if err != nil {
		return err
	}

	// Normalize the code (remove dashes)
	normalized := normalizeRecoveryCode(code)

	for _, stored := range codes {
		if err := bcrypt.CompareHashAndPassword([]byte(stored.CodeHash), []byte(normalized)); err == nil {
			// Code matches - consume it
			return s.totpRepo.DeleteRecoveryCode(ctx, stored.ID)
		}
	}

	return ErrInvalidRecoveryCode
}

// Disable removes 2FA and all recovery codes for the user.
func (s *TOTPService) Disable(ctx context.Context, userID string) error {
	if err := s.totpRepo.DeleteRecoveryCodesByUser(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete recovery codes: %w", err)
	}
	if err := s.totpRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete TOTP config: %w", err)
	}
	return nil
}

// RegenerateRecoveryCodes generates new recovery codes, replacing old ones.
func (s *TOTPService) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	// Verify 2FA is enabled
	totpRecord, err := s.totpRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTOTPNotEnabled
		}
		return nil, err
	}
	if !totpRecord.Enabled {
		return nil, ErrTOTPNotEnabled
	}

	codes, plainCodes, err := s.generateRecoveryCodes(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recovery codes: %w", err)
	}

	if err := s.totpRepo.SaveRecoveryCodes(ctx, codes); err != nil {
		return nil, fmt.Errorf("failed to save recovery codes: %w", err)
	}

	return plainCodes, nil
}

// GetStatus returns whether 2FA is enabled for the user.
func (s *TOTPService) GetStatus(ctx context.Context, userID string) (bool, error) {
	totpRecord, err := s.totpRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return totpRecord.Enabled, nil
}

// GeneratePendingToken creates a short-lived JWT with a TFA claim.
func (s *TOTPService) GeneratePendingToken(userID string, isAdmin bool) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		TFA:     true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(pendingTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidatePendingToken validates a pending 2FA token and returns the claims.
func (s *TOTPService) ValidatePendingToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
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

	if !claims.TFA {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// generateRecoveryCodes creates N random codes and their bcrypt hashes.
func (s *TOTPService) generateRecoveryCodes(userID string) ([]*model.RecoveryCode, []string, error) {
	var codes []*model.RecoveryCode
	var plainCodes []string

	for i := 0; i < recoveryCodeCount; i++ {
		raw := make([]byte, recoveryCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}

		hexStr := hex.EncodeToString(raw)
		formatted := formatRecoveryCode(hexStr)
		plainCodes = append(plainCodes, formatted)

		hash, err := bcrypt.GenerateFromPassword([]byte(hexStr), recoveryBcryptCost)
		if err != nil {
			return nil, nil, err
		}

		codes = append(codes, &model.RecoveryCode{
			ID:       uuid.NewString(),
			UserID:   userID,
			CodeHash: string(hash),
		})
	}

	return codes, plainCodes, nil
}

// formatRecoveryCode formats 8 hex chars as "xxxx-xxxx".
func formatRecoveryCode(raw string) string {
	if len(raw) != 8 {
		return raw
	}
	return raw[:4] + "-" + raw[4:]
}

// normalizeRecoveryCode removes dashes from a recovery code for comparison.
func normalizeRecoveryCode(code string) string {
	result := make([]byte, 0, len(code))
	for i := range len(code) {
		if code[i] != '-' {
			result = append(result, code[i])
		}
	}
	return string(result)
}
