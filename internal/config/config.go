package config

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const redacted = "***"

var (
	_,
	b, _, _ = runtime.Caller(0)
)

type Config struct {
	Port             int
	DatabasePath     string
	JWTSecret        string
	BaseURL          string
	StorageType      string
	StorageLocalPath string
	S3Endpoint       string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3Region         string
	S3PathPrefix     string
	SMTPHost         string
	SMTPPort         int
	SMTPUser         string
	SMTPPass         string
	SMTPFrom         string
	SMTPTLSPolicy    string
	// OIDC configuration
	OIDCEnabled      bool
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       string
	// CORS
	CORSOrigins string
	// 2FA enforcement
	Require2FA bool
	// Trusted reverse-proxy CIDRs whose X-Forwarded-For / X-Real-IP headers
	// are trusted for client-IP extraction (e.g. rate limiting).
	TrustedProxyCIDRs []string
	// Direct transfer presigned URL expiry in seconds.
	DirectTransferExpiry int
}

// Load reads environment-backed settings and returns the application config.
func Load() *Config {
	return &Config{
		Port:                 getEnvInt("PORT", 8080),
		DatabasePath:         getEnv("DATABASE_PATH", "./enlace.db"),
		JWTSecret:            loadJWTSecret(),
		BaseURL:              getEnv("BASE_URL", "http://localhost:8080"),
		StorageType:          getEnv("STORAGE_TYPE", "local"),
		StorageLocalPath:     getEnv("STORAGE_LOCAL_PATH", "./uploads"),
		S3Endpoint:           getEnv("S3_ENDPOINT", ""),
		S3Bucket:             getEnv("S3_BUCKET", ""),
		S3AccessKey:          getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:          getEnv("S3_SECRET_KEY", ""),
		S3Region:             getEnv("S3_REGION", ""),
		S3PathPrefix:         getEnv("S3_PATH_PREFIX", ""),
		SMTPHost:             getEnv("SMTP_HOST", ""),
		SMTPPort:             getEnvInt("SMTP_PORT", 587),
		SMTPUser:             getEnv("SMTP_USER", ""),
		SMTPPass:             getEnv("SMTP_PASS", ""),
		SMTPFrom:             getEnv("SMTP_FROM", "noreply@example.com"),
		SMTPTLSPolicy:        getEnv("SMTP_TLS_POLICY", "opportunistic"),
		OIDCEnabled:          getEnvBool("OIDC_ENABLED", false),
		OIDCIssuerURL:        getEnv("OIDC_ISSUER_URL", ""),
		OIDCClientID:         getEnv("OIDC_CLIENT_ID", ""),
		OIDCClientSecret:     getEnv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:      getEnv("OIDC_REDIRECT_URL", ""),
		OIDCScopes:           getEnv("OIDC_SCOPES", "openid email profile"),
		CORSOrigins:          getEnv("CORS_ORIGINS", ""),
		Require2FA:           getEnvBool("REQUIRE_2FA", false),
		TrustedProxyCIDRs:    getEnvStringSlice("TRUSTED_PROXIES", ","),
		DirectTransferExpiry: getEnvInt("DIRECT_TRANSFER_EXPIRY_SECONDS", 600),
	}
}

// LogValue implements slog.LogValuer so that logging a *Config never exposes
// secret fields. All credential-like values are replaced with "***".
func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("port", c.Port),
		slog.String("database_path", c.DatabasePath),
		slog.String("jwt_secret", maskSecret(c.JWTSecret)),
		slog.String("base_url", c.BaseURL),
		slog.String("storage_type", c.StorageType),
		slog.String("storage_local_path", c.StorageLocalPath),
		slog.String("s3_endpoint", c.S3Endpoint),
		slog.String("s3_bucket", c.S3Bucket),
		slog.String("s3_access_key", maskSecret(c.S3AccessKey)),
		slog.String("s3_secret_key", maskSecret(c.S3SecretKey)),
		slog.String("s3_region", c.S3Region),
		slog.String("s3_path_prefix", c.S3PathPrefix),
		slog.String("smtp_host", c.SMTPHost),
		slog.Int("smtp_port", c.SMTPPort),
		slog.String("smtp_user", maskSecret(c.SMTPUser)),
		slog.String("smtp_pass", maskSecret(c.SMTPPass)),
		slog.String("smtp_from", c.SMTPFrom),
		slog.String("smtp_tls_policy", c.SMTPTLSPolicy),
		slog.Bool("oidc_enabled", c.OIDCEnabled),
		slog.String("oidc_issuer_url", c.OIDCIssuerURL),
		slog.String("oidc_client_id", c.OIDCClientID),
		slog.String("oidc_client_secret", maskSecret(c.OIDCClientSecret)),
		slog.String("oidc_redirect_url", c.OIDCRedirectURL),
		slog.String("oidc_scopes", c.OIDCScopes),
		slog.String("cors_origins", c.CORSOrigins),
		slog.Bool("require_2fa", c.Require2FA),
		slog.Int("direct_transfer_expiry", c.DirectTransferExpiry),
	)
}

// maskSecret returns redacted when s is non-empty, or an empty string otherwise.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return redacted
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true" || val == "1"
	}
	return defaultVal
}

// getEnvStringSlice returns a slice of non-empty strings from the environment variable
// identified by key, split by sep. Returns nil when the variable is unset or empty.
func getEnvStringSlice(key, sep string) []string {
	val := os.Getenv(key)
	if val == "" {
		return nil
	}
	var result []string
	for _, s := range strings.Split(val, sep) {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func loadJWTSecret() string {
	// this value is stored in a file to ensure it persists across restarts but is not easily accessible as an environment variable
	dataDir := getEnv("DATA_DIR", "./data")
	secretPath := filepath.Join(dataDir, "jwt_secret")
	if secretBytes, err := os.ReadFile(secretPath); err == nil {
		return string(secretBytes)
	}
	// If the file doesn't exist or can't be read, generate a new 256-bit secret and save it
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		slog.Error("Failed to generate cryptographically secure JWT secret", "error", err)
		os.Exit(1)
	}
	secret := base64.RawURLEncoding.EncodeToString(key)
	if err := os.MkdirAll(dataDir, 0700); err == nil {
		if err := os.WriteFile(secretPath, []byte(secret), 0600); err == nil {
			return secret
		}
	}
	slog.Error("Failed to load or save JWT secret, check file permissions and ensure the data directory is writable")
	return secret // return the generated secret even if we couldn't save it, to ensure the application can still run
}

// GetProjectRoot returns the root directory of the project.
func GetProjectRoot() string {
	return filepath.Join(filepath.Dir(b), "../..") //nolint:gocritic // This is a safe operation.
}
