package config_test

import (
	"os"
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
	os.Setenv("JWT_SECRET", "testsecret")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("JWT_SECRET")
	}()

	cfg := config.Load()

	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
	if cfg.JWTSecret != "testsecret" {
		t.Errorf("expected JWT secret testsecret, got %s", cfg.JWTSecret)
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
