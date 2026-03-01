package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sharer "github.com/amalgamated-tools/sharer"
	"github.com/amalgamated-tools/sharer/internal/config"
	"github.com/amalgamated-tools/sharer/internal/database"
	"github.com/amalgamated-tools/sharer/internal/handler"
	"github.com/amalgamated-tools/sharer/internal/repository"
	"github.com/amalgamated-tools/sharer/internal/service"
	"github.com/amalgamated-tools/sharer/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg := config.Load()

	// Validate required config
	if cfg.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required")
	}

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize storage
	store, err := initStorage(cfg)
	if err != nil {
		return err
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())

	// Initialize services
	jwtSecret := []byte(cfg.JWTSecret)
	authService := service.NewAuthService(userRepo, jwtSecret)
	shareService := service.NewShareService(shareRepo, fileRepo, store)
	fileService := service.NewFileService(fileRepo, shareRepo, store)

	// Initialize OIDC service (optional, based on config)
	var oidcService *service.OIDCService
	if cfg.OIDCEnabled {
		var err error
		oidcService, err = service.NewOIDCService(cfg, userRepo)
		if err != nil {
			log.Printf("Warning: failed to initialize OIDC: %v", err)
		} else {
			log.Println("OIDC authentication enabled")
		}
	}

	// Get embedded frontend
	frontendFS, err := sharer.FrontendFS()
	if err != nil {
		log.Printf("Warning: failed to load embedded frontend: %v", err)
	}

	// Initialize router
	router := handler.NewRouter(handler.RouterConfig{
		AuthService:  authService,
		ShareService: shareService,
		FileService:  fileService,
		UserRepo:     userRepo,
		ShareRepo:    shareRepo,
		FileRepo:     fileRepo,
		Storage:      store,
		JWTSecret:    cfg.JWTSecret,
		BaseURL:      cfg.BaseURL,
		OIDCService:  oidcService,
		FrontendFS:   frontendFS,
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
		log.Printf("Sharer starting on http://localhost:%d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// initStorage initializes the storage backend based on configuration.
func initStorage(cfg *config.Config) (storage.Storage, error) {
	switch cfg.StorageType {
	case "s3":
		s3Store, err := storage.NewS3Storage(context.Background(), storage.S3Config{
			Endpoint:   cfg.S3Endpoint,
			Bucket:     cfg.S3Bucket,
			AccessKey:  cfg.S3AccessKey,
			SecretKey:  cfg.S3SecretKey,
			Region:     cfg.S3Region,
			PathPrefix: cfg.S3PathPrefix,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize S3 storage: %w", err)
		}
		return s3Store, nil
	default:
		return storage.NewLocalStorage(cfg.StorageLocalPath), nil
	}
}
