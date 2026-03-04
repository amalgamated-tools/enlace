package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
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
	recoveryCodeBytes  = 10 // 10 bytes = 80 bits of entropy
	recoveryBcryptCost = 10
	totpIssuer         = "Enlace"
	pendingTokenExpiry = 5 * time.Minute

	// encryptedSecretPrefix tags secrets that were encrypted with AES-GCM.
	// Secrets without this prefix are treated as legacy plaintext.
	encryptedSecretPrefix = "enc:"
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
	if err == nil {
		if existing.Enabled {
			return "", "", "", ErrTOTPAlreadyEnabled
		}
	} else if !errors.Is(err, repository.ErrNotFound) {
		return "", "", "", fmt.Errorf("failed to get TOTP config: %w", err)
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

	// Store the secret encrypted at rest
	encryptedSecret, err := s.encryptSecret(key.Secret())
	if err != nil {
		return "", "", "", fmt.Errorf("failed to encrypt TOTP secret: %w", err)
	}
	totpRecord := &model.UserTOTP{
		UserID:  userID,
		Secret:  encryptedSecret,
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

	// Validate the TOTP code against the decrypted secret
	secret, err := s.decryptSecret(totpRecord.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt TOTP secret: %w", err)
	}
	if !totp.Validate(code, secret) {
		return nil, ErrInvalidTOTPCode
	}

	// Generate recovery codes
	codes, plainCodes, err := s.generateRecoveryCodes(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recovery codes: %w", err)
	}

	// Atomically enable 2FA and save recovery codes
	if err := s.totpRepo.EnableAndSaveRecoveryCodes(ctx, userID, codes); err != nil {
		return nil, fmt.Errorf("failed to enable 2FA: %w", err)
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

	secret, err := s.decryptSecret(totpRecord.Secret)
	if err != nil {
		return fmt.Errorf("failed to decrypt TOTP secret: %w", err)
	}
	if !totp.Validate(code, secret) {
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
			if err := s.totpRepo.DeleteRecoveryCode(ctx, stored.ID); err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return ErrInvalidRecoveryCode
				}
				return err
			}
			return nil
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
		UserID:    userID,
		IsAdmin:   isAdmin,
		TFA:       true,
		TokenType: TokenTypeAccess,
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

// formatRecoveryCode formats hex chars as "xxxx-xxxx-xxxx-xxxx-xxxx".
func formatRecoveryCode(raw string) string {
	var result []byte
	for i, c := range raw {
		if i > 0 && i%4 == 0 {
			result = append(result, '-')
		}
		result = append(result, byte(c))
	}
	return string(result)
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

// deriveEncryptionKey derives a 32-byte AES key from the JWT secret using SHA-256.
func deriveEncryptionKey(jwtSecret []byte) []byte {
	hash := sha256.Sum256(append([]byte("totp-secret-encryption:"), jwtSecret...))
	return hash[:]
}

// encryptSecret encrypts a TOTP secret using AES-GCM and tags the
// output with encryptedSecretPrefix so decryptSecret can distinguish
// encrypted values from legacy plaintext.
func (s *TOTPService) encryptSecret(plaintext string) (string, error) {
	key := deriveEncryptionKey(s.jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedSecretPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptSecret decrypts an AES-GCM encrypted TOTP secret.
// Tagged secrets (prefixed with "enc:") are decrypted; failures are
// returned as errors. Untagged secrets are tried as old-format encrypted
// values first; if that fails they are returned as legacy plaintext.
func (s *TOTPService) decryptSecret(encoded string) (string, error) {
	if strings.HasPrefix(encoded, encryptedSecretPrefix) {
		return s.decryptAESGCM(strings.TrimPrefix(encoded, encryptedSecretPrefix))
	}

	// Untagged: try old-format (bare base64) decrypt, fall back to plaintext
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encoded, nil
	}
	key := deriveEncryptionKey(s.jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return encoded, nil
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encoded, nil
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return encoded, nil
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		// base64-decodable but not valid ciphertext — legacy plaintext
		return encoded, nil
	}
	return string(plaintext), nil
}

// decryptAESGCM decodes base64 and decrypts AES-GCM ciphertext.
func (s *TOTPService) decryptAESGCM(b64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted secret: %w", err)
	}
	key := deriveEncryptionKey(s.jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("encrypted secret too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt TOTP secret (wrong key or corrupt data): %w", err)
	}
	return string(plaintext), nil
}
