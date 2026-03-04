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
	// Swagger/API docs
	SwaggerEnabled bool
	// CORS
	CORSOrigins string
	// 2FA enforcement
	Require2FA bool
	// Trusted reverse-proxy CIDRs whose X-Forwarded-For / X-Real-IP headers
	// are trusted for client-IP extraction (e.g. rate limiting).
	TrustedProxyCIDRs []string
}

func Load() *Config {
	return &Config{
		Port:              getEnvInt("PORT", 8080),
		DatabasePath:      getEnv("DATABASE_PATH", "./enlace.db"),
		JWTSecret:         loadJWTSecret(),
		BaseURL:           getEnv("BASE_URL", "http://localhost:8080"),
		StorageType:       getEnv("STORAGE_TYPE", "local"),
		StorageLocalPath:  getEnv("STORAGE_LOCAL_PATH", "./uploads"),
		S3Endpoint:        getEnv("S3_ENDPOINT", ""),
		S3Bucket:          getEnv("S3_BUCKET", ""),
		S3AccessKey:       getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:       getEnv("S3_SECRET_KEY", ""),
		S3Region:          getEnv("S3_REGION", ""),
		S3PathPrefix:      getEnv("S3_PATH_PREFIX", ""),
		SMTPHost:          getEnv("SMTP_HOST", ""),
		SMTPPort:          getEnvInt("SMTP_PORT", 587),
		SMTPUser:          getEnv("SMTP_USER", ""),
		SMTPPass:          getEnv("SMTP_PASS", ""),
		SMTPFrom:          getEnv("SMTP_FROM", "noreply@example.com"),
		SMTPTLSPolicy:     getEnv("SMTP_TLS_POLICY", "opportunistic"),
		OIDCEnabled:       getEnvBool("OIDC_ENABLED", false),
		OIDCIssuerURL:     getEnv("OIDC_ISSUER_URL", ""),
		OIDCClientID:      getEnv("OIDC_CLIENT_ID", ""),
		OIDCClientSecret:  getEnv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:   getEnv("OIDC_REDIRECT_URL", ""),
		OIDCScopes:        getEnv("OIDC_SCOPES", "openid email profile"),
		SwaggerEnabled:    getEnvBool("SWAGGER_ENABLED", false),
		CORSOrigins:       getEnv("CORS_ORIGINS", ""),
		Require2FA:        getEnvBool("REQUIRE_2FA", false),
		TrustedProxyCIDRs: getEnvStringSlice("TRUSTED_PROXIES", ","),
	}
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
