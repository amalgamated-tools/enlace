//go:generate swag init -g main.go -o docs --parseDependency --parseInternal -d ../../

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	enlace "github.com/amalgamated-tools/enlace"
	"github.com/amalgamated-tools/enlace/internal/config"
	"github.com/amalgamated-tools/enlace/internal/crypto"
	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/handler"
	"github.com/amalgamated-tools/enlace/internal/otel"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

var version = "dev"

//	@title			Enlace API
//	@version		1.0
//	@description	File sharing API with support for password-protected shares, expiring links, reverse shares, and admin user management.

//	@host		localhost:8080
//	@BasePath	/

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Enter your Bearer token: Bearer {token}

// @securityDefinitions.apikey	ShareToken
// @in							header
// @name						X-Share-Token
// @description				Share access token for password-protected shares
func main() {
	otel.SetupLogger(version)
	cancelCtx, cancelAll := context.WithCancel(context.Background())

	if err := realMain(cancelCtx); err != nil {
		slog.ErrorContext(cancelCtx, "error occurred", slog.Any("error", err))
		cancelAll()
	}
}

// This is the real main function. That's why it's called realMain.
func realMain(cancelCtx context.Context) error { //nolint:contextcheck // The newctx context comes from the StartTracer function, so it's already wrapped.
	flagSet := flag.NewFlagSet("enlace", flag.ExitOnError)

	var (
		port    int
		showVer bool
	)
	flagSet.IntVar(&port, "port", 0, "port number to run http server on")
	flagSet.BoolVar(&showVer, "version", false, "show version and exit")

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if showVer {
		fmt.Println(otel.Version)
		os.Exit(0)
	}
	slog.Info("enlace", slog.String("version", version))
	cfg := config.Load()
	slog.DebugContext(cancelCtx, "configuration loaded", slog.Any("config", cfg))

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	totpRepo := repository.NewTOTPRepository(db.DB())
	settingsRepo := repository.NewSettingsRepository(db.DB())

	// Initialize storage
	store, err := initStorage(cancelCtx, cfg, settingsRepo)
	if err != nil {
		return err
	}

	// Initialize services
	jwtSecret := []byte(cfg.JWTSecret)
	authService := service.NewAuthService(userRepo, jwtSecret)
	shareService := service.NewShareService(shareRepo, fileRepo, store)
	fileService := service.NewFileService(fileRepo, shareRepo, store)

	// Initialize recipient repository and email service
	recipientRepo := repository.NewRecipientRepository(db.DB())
	emailService := service.NewEmailService(service.SMTPConfig{
		Host:      cfg.SMTPHost,
		Port:      cfg.SMTPPort,
		User:      cfg.SMTPUser,
		Pass:      cfg.SMTPPass,
		From:      cfg.SMTPFrom,
		TLSPolicy: cfg.SMTPTLSPolicy,
	}, recipientRepo, cfg.BaseURL)
	totpService := service.NewTOTPService(totpRepo, userRepo, jwtSecret)

	// Initialize OIDC service (optional, based on config)
	var oidcService *service.OIDCService
	if cfg.OIDCEnabled {
		var err error
		oidcService, err = service.NewOIDCService(cfg, userRepo, totpService)
		if err != nil {
			slog.WarnContext(cancelCtx, "failed to initialize OIDC", slog.Any("error", err))
		} else {
			slog.InfoContext(cancelCtx, "OIDC authentication enabled")
		}
	}

	// Get embedded frontend
	frontendFS, err := enlace.FrontendFS()
	if err != nil {
		slog.WarnContext(cancelCtx, "failed to load embedded frontend", slog.Any("error", err))
	}

	// Parse CORS origins
	var corsOrigins []string
	if cfg.CORSOrigins != "" {
		for _, o := range strings.Split(cfg.CORSOrigins, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				corsOrigins = append(corsOrigins, trimmed)
			}
		}
	}

	// Initialize router
	router := handler.NewRouter(handler.RouterConfig{
		AuthService:    authService,
		ShareService:   shareService,
		FileService:    fileService,
		EmailService:   emailService,
		UserRepo:       userRepo,
		ShareRepo:      shareRepo,
		FileRepo:       fileRepo,
		Storage:        store,
		SettingsRepo:   settingsRepo,
		JWTSecret:      cfg.JWTSecret,
		BaseURL:        cfg.BaseURL,
		OIDCService:    oidcService,
		FrontendFS:     frontendFS,
		SwaggerEnabled: cfg.SwaggerEnabled,
		CORSOrigins:    corsOrigins,
		TOTPService:    totpService,
		Require2FA:     cfg.Require2FA,
	})

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("Enlace starting", "url", fmt.Sprintf("http://localhost:%d", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	slog.Info("Server stopped")
	return nil
}

// initStorage initializes the storage backend based on configuration.
// It checks DB settings first, falling back to env-var config.
func initStorage(ctx context.Context, cfg *config.Config, settingsRepo *repository.SettingsRepository) (storage.Storage, error) {
	storageKeys := []string{
		"storage_type", "storage_local_path",
		"s3_endpoint", "s3_bucket", "s3_access_key", "s3_secret_key", "s3_region", "s3_path_prefix",
	}

	dbSettings, err := settingsRepo.GetMultiple(ctx, storageKeys)
	if err != nil {
		slog.WarnContext(ctx, "failed to load storage settings from database, using env config", slog.Any("error", err))
		dbSettings = map[string]string{}
	}

	// Decrypt the S3 secret key if it was stored encrypted
	if raw, ok := dbSettings["s3_secret_key"]; ok && raw != "" {
		encKey := crypto.DeriveKey([]byte(cfg.JWTSecret), "storage-secret-encryption")
		decrypted, err := crypto.Decrypt(raw, encKey)
		if err != nil {
			slog.WarnContext(ctx, "failed to decrypt s3_secret_key from database, ignoring DB value", slog.Any("error", err))
			delete(dbSettings, "s3_secret_key")
		} else {
			dbSettings["s3_secret_key"] = decrypted
		}
	}

	// Helper: get from DB first, then fall back to config value
	getVal := func(dbKey string, envVal string) string {
		if v, ok := dbSettings[dbKey]; ok && v != "" {
			return v
		}
		return envVal
	}

	storageType := getVal("storage_type", cfg.StorageType)

	switch storageType {
	case "s3":
		s3Store, err := storage.NewS3Storage(ctx, storage.S3Config{
			Endpoint:   getVal("s3_endpoint", cfg.S3Endpoint),
			Bucket:     getVal("s3_bucket", cfg.S3Bucket),
			AccessKey:  getVal("s3_access_key", cfg.S3AccessKey),
			SecretKey:  getVal("s3_secret_key", cfg.S3SecretKey),
			Region:     getVal("s3_region", cfg.S3Region),
			PathPrefix: getVal("s3_path_prefix", cfg.S3PathPrefix),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize S3 storage: %w", err)
		}
		return s3Store, nil
	default:
		return storage.NewLocalStorage(getVal("storage_local_path", cfg.StorageLocalPath)), nil
	}
}
