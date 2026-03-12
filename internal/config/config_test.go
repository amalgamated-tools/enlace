package config_test

import (
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// unset any env vars that might affect the config
	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_PATH") // this seems to only affect local tests
	cfg := config.Load()

	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.DatabasePath != "./enlace.db" {
		t.Errorf("expected database path ./enlace.db, got %s", cfg.DatabasePath)
	}
	if cfg.StorageType != "local" {
		t.Errorf("expected storage type local, got %s", cfg.StorageType)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("PORT", "9000")
	defer func() {
		os.Unsetenv("PORT")
	}()

	cfg := config.Load()

	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
}

func TestLoad_OIDCConfig(t *testing.T) {
	os.Setenv("OIDC_ENABLED", "true")
	os.Setenv("OIDC_ISSUER_URL", "https://auth.example.com")
	os.Setenv("OIDC_CLIENT_ID", "enlace")
	os.Setenv("OIDC_CLIENT_SECRET", "secret123")
	os.Setenv("OIDC_REDIRECT_URL", "http://localhost:8080/api/v1/auth/oidc/callback")
	defer func() {
		os.Unsetenv("OIDC_ENABLED")
		os.Unsetenv("OIDC_ISSUER_URL")
		os.Unsetenv("OIDC_CLIENT_ID")
		os.Unsetenv("OIDC_CLIENT_SECRET")
		os.Unsetenv("OIDC_REDIRECT_URL")
	}()

	cfg := config.Load()

	if !cfg.OIDCEnabled {
		t.Error("expected OIDC to be enabled")
	}
	if cfg.OIDCIssuerURL != "https://auth.example.com" {
		t.Errorf("expected issuer URL https://auth.example.com, got %s", cfg.OIDCIssuerURL)
	}
	if cfg.OIDCClientID != "enlace" {
		t.Errorf("expected client ID enlace, got %s", cfg.OIDCClientID)
	}
	if cfg.OIDCClientSecret != "secret123" {
		t.Errorf("expected client secret secret123, got %s", cfg.OIDCClientSecret)
	}
	if cfg.OIDCRedirectURL != "http://localhost:8080/api/v1/auth/oidc/callback" {
		t.Errorf("unexpected redirect URL: %s", cfg.OIDCRedirectURL)
	}
}

func TestLoad_OIDCDefaults(t *testing.T) {
	cfg := config.Load()

	if cfg.OIDCEnabled {
		t.Error("expected OIDC to be disabled by default")
	}
	if cfg.OIDCScopes != "openid email profile" {
		t.Errorf("expected default scopes 'openid email profile', got %s", cfg.OIDCScopes)
	}
}

func TestLoad_JWTSecret_GeneratesAndPersists(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	cfg := config.Load()

	if cfg.JWTSecret == "" {
		t.Fatal("expected a non-empty JWT secret")
	}

	// Verify the secret is valid base64url with 256 bits (32 bytes) of entropy
	decoded, err := base64.RawURLEncoding.DecodeString(cfg.JWTSecret)
	if err != nil {
		t.Fatalf("JWT secret is not valid base64url: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("expected 32 bytes of key material, got %d", len(decoded))
	}

	// Verify the secret was persisted to disk
	secretPath := filepath.Join(dataDir, "jwt_secret")
	written, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("expected jwt_secret file to exist: %v", err)
	}
	if string(written) != cfg.JWTSecret {
		t.Errorf("persisted secret %q does not match loaded secret %q", string(written), cfg.JWTSecret)
	}
}

func TestLoad_JWTSecret_ReadsExistingFile(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	existingSecret := "my-pre-existing-secret-value"
	secretPath := filepath.Join(dataDir, "jwt_secret")
	if err := os.WriteFile(secretPath, []byte(existingSecret), 0600); err != nil {
		t.Fatalf("failed to write test secret file: %v", err)
	}

	cfg := config.Load()

	if cfg.JWTSecret != existingSecret {
		t.Errorf("expected JWT secret %q, got %q", existingSecret, cfg.JWTSecret)
	}
}

func TestConfig_LogValue_RedactsSecrets(t *testing.T) {
	cfg := &config.Config{
		Port:             8080,
		DatabasePath:     "./enlace.db",
		JWTSecret:        "super-secret-jwt",
		BaseURL:          "http://localhost:8080",
		StorageType:      "s3",
		StorageLocalPath: "./uploads",
		S3Endpoint:       "https://s3.example.com",
		S3Bucket:         "my-bucket",
		S3AccessKey:      "AKIAIOSFODNN7EXAMPLE",
		S3SecretKey:      "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		S3Region:         "us-east-1",
		SMTPHost:         "smtp.example.com",
		SMTPPort:         587,
		SMTPUser:         "smtp-user@example.com",
		SMTPPass:         "smtp-password",
		SMTPFrom:         "noreply@example.com",
		OIDCEnabled:      true,
		OIDCClientID:     "my-client-id",
		OIDCClientSecret: "my-oidc-secret",
		OIDCIssuerURL:    "https://auth.example.com",
	}

	// Render the config via slog to a text buffer and verify secrets are not present.
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Debug("config", slog.Any("config", cfg))
	logged := buf.String()

	secretFields := []string{
		"super-secret-jwt",
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"smtp-user@example.com",
		"smtp-password",
		"my-oidc-secret",
	}
	for _, secret := range secretFields {
		if strings.Contains(logged, secret) {
			t.Errorf("log output must not contain secret %q; got: %s", secret, logged)
		}
	}

	// Non-secret fields should still appear.
	safeFields := []string{
		"http://localhost:8080",
		"smtp.example.com",
		"my-client-id",
		"https://auth.example.com",
		"us-east-1",
		"my-bucket",
	}
	for _, field := range safeFields {
		if !strings.Contains(logged, field) {
			t.Errorf("log output should contain non-secret field %q; got: %s", field, logged)
		}
	}
}

func TestConfig_LogValue_EmptySecrets(t *testing.T) {
	cfg := &config.Config{
		Port:         8080,
		DatabasePath: "./enlace.db",
		BaseURL:      "http://localhost:8080",
		StorageType:  "local",
		// All secret fields are empty (zero value)
	}

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Debug("config", slog.Any("config", cfg))
	logged := buf.String()

	// When a secret field is empty it should log as empty, not "***"
	if strings.Contains(logged, "***") {
		t.Errorf("empty secret fields should not be logged as ***, got: %s", logged)
	}

	// Verify secret field keys are present with empty values
	emptySecretFields := []string{
		`config.jwt_secret=""`,
		`config.smtp_pass=""`,
		`config.smtp_user=""`,
		`config.s3_access_key=""`,
		`config.s3_secret_key=""`,
		`config.oidc_client_secret=""`,
	}
	for _, field := range emptySecretFields {
		if !strings.Contains(logged, field) {
			t.Errorf("log output should contain empty secret field %q; got: %s", field, logged)
		}
	}
}

func TestLoad_JWTSecret_StableAcrossCalls(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	cfg1 := config.Load()
	cfg2 := config.Load()

	if cfg1.JWTSecret != cfg2.JWTSecret {
		t.Errorf("JWT secret changed between Load() calls: %q vs %q", cfg1.JWTSecret, cfg2.JWTSecret)
	}
}
