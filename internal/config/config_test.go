package config_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
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

func TestLoad_JWTSecret_StableAcrossCalls(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	cfg1 := config.Load()
	cfg2 := config.Load()

	if cfg1.JWTSecret != cfg2.JWTSecret {
		t.Errorf("JWT secret changed between Load() calls: %q vs %q", cfg1.JWTSecret, cfg2.JWTSecret)
	}
}
