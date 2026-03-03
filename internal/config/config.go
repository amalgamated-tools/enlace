package config

import (
	"os"
	"strconv"
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
}

func Load() *Config {
	return &Config{
		Port:             getEnvInt("PORT", 8080),
		DatabasePath:     getEnv("DATABASE_PATH", "./enlace.db"),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		BaseURL:          getEnv("BASE_URL", "http://localhost:8080"),
		StorageType:      getEnv("STORAGE_TYPE", "local"),
		StorageLocalPath: getEnv("STORAGE_LOCAL_PATH", "./uploads"),
		S3Endpoint:       getEnv("S3_ENDPOINT", ""),
		S3Bucket:         getEnv("S3_BUCKET", ""),
		S3AccessKey:      getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:      getEnv("S3_SECRET_KEY", ""),
		S3Region:         getEnv("S3_REGION", ""),
		S3PathPrefix:     getEnv("S3_PATH_PREFIX", ""),
		SMTPHost:         getEnv("SMTP_HOST", ""),
		SMTPPort:         getEnvInt("SMTP_PORT", 587),
		SMTPUser:         getEnv("SMTP_USER", ""),
		SMTPPass:         getEnv("SMTP_PASS", ""),
		SMTPFrom:         getEnv("SMTP_FROM", "noreply@example.com"),
		SMTPTLSPolicy:    getEnv("SMTP_TLS_POLICY", "opportunistic"),
		OIDCEnabled:      getEnvBool("OIDC_ENABLED", false),
		OIDCIssuerURL:    getEnv("OIDC_ISSUER_URL", ""),
		OIDCClientID:     getEnv("OIDC_CLIENT_ID", ""),
		OIDCClientSecret: getEnv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:  getEnv("OIDC_REDIRECT_URL", ""),
		OIDCScopes:       getEnv("OIDC_SCOPES", "openid email profile"),
		SwaggerEnabled:   getEnvBool("SWAGGER_ENABLED", false),
		CORSOrigins:      getEnv("CORS_ORIGINS", ""),
		Require2FA:       getEnvBool("REQUIRE_2FA", false),
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
