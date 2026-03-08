package handler

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/amalgamated-tools/enlace/docs"
	intMiddleware "github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

// RouterConfig contains all dependencies required to create the router.
type RouterConfig struct {
	// Services
	AuthService    *service.AuthService
	ShareService   *service.ShareService
	FileService    *service.FileService
	EmailService   *service.EmailService
	APIKeyService  *service.APIKeyService
	WebhookService *service.WebhookService

	// Repositories (for middleware/handlers that need direct access)
	UserRepo  *repository.UserRepository
	ShareRepo *repository.ShareRepository
	FileRepo  *repository.FileRepository

	// Storage
	Storage storage.Storage

	// Settings repository (for admin storage config)
	SettingsRepo SettingsRepositoryInterface

	// Configuration
	JWTSecret string
	BaseURL   string

	// OIDC Service (optional)
	OIDCService *service.OIDCService

	// Frontend filesystem (embedded)
	FrontendFS fs.FS

	// CORS allowed origins (comma-separated). Defaults to BaseURL if empty.
	CORSOrigins []string

	// TOTP Service (optional, for 2FA)
	TOTPService *service.TOTPService

	// 2FA enforcement
	Require2FA bool

	// E2E encryption feature flag
	E2EEncryptionEnabled bool

	// TrustedProxyCIDRs is the list of trusted reverse-proxy CIDRs whose
	// X-Forwarded-For / X-Real-IP headers are trusted for client-IP extraction.
	TrustedProxyCIDRs []string
}

// NewRouter creates a new Chi router with all routes configured.
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Set a timeout for requests
	r.Use(middleware.Timeout(60 * 1000000000)) // 60 seconds in nanoseconds

	// CORS middleware
	allowedOrigins := cfg.CORSOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{cfg.BaseURL}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Share-Token"},
		ExposedHeaders:   []string{"Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health check endpoint (always accessible)
	r.Get("/health", healthHandler(cfg.EmailService, cfg.E2EEncryptionEnabled))

	// Create handlers
	var totpServiceAdapter TOTPServiceInterface
	if cfg.TOTPService != nil {
		totpServiceAdapter = newTOTPServiceAdapter(cfg.TOTPService)
	}
	authHandler := NewAuthHandler(cfg.AuthService, totpServiceAdapter, cfg.Require2FA)
	var totpHandler *TOTPHandler
	if totpServiceAdapter != nil {
		totpHandler = NewTOTPHandler(totpServiceAdapter, newAuthTokenAdapter(cfg.AuthService), newPasswordVerifierAdapter(cfg.AuthService), cfg.Require2FA, cfg.AuthService)
	}
	shareHandler := NewShareHandler(cfg.ShareService, cfg.FileService, cfg.EmailService, WithShareWebhookEmitter(cfg.WebhookService))
	fileHandler := NewFileHandler(cfg.FileService, cfg.ShareService, WithSettingsRepo(cfg.SettingsRepo), WithFileWebhookEmitter(cfg.WebhookService))
	userHandler := NewUserHandler(cfg.AuthService)
	adminHandler := NewAdminHandler(cfg.UserRepo)
	var apiKeyHandler *APIKeyHandler
	if cfg.APIKeyService != nil {
		apiKeyHandler = NewAPIKeyHandler(cfg.APIKeyService)
	}
	var webhookHandler *WebhookAdminHandler
	if cfg.WebhookService != nil {
		webhookHandler = NewWebhookAdminHandler(cfg.WebhookService)
	}
	var storageConfigHandler *StorageConfigHandler
	var fileRestrictionsHandler *FileRestrictionsHandler
	var smtpConfigHandler *SMTPConfigHandler
	if cfg.SettingsRepo != nil {
		storageConfigHandler = NewStorageConfigHandler(cfg.SettingsRepo, []byte(cfg.JWTSecret))
		fileRestrictionsHandler = NewFileRestrictionsHandler(cfg.SettingsRepo)
		smtpConfigHandler = NewSMTPConfigHandler(cfg.SettingsRepo, []byte(cfg.JWTSecret))
	}
	secureCookies := strings.HasPrefix(cfg.BaseURL, "https://")
	publicHandler := NewPublicHandler(cfg.ShareService, cfg.FileService, []byte(cfg.JWTSecret), WithPublicSettingsRepo(cfg.SettingsRepo), WithSecureCookies(secureCookies), WithPublicWebhookEmitter(cfg.WebhookService))
	oidcHandler := NewOIDCHandler(newOIDCServiceAdapter(cfg.OIDCService), newAuthTokenAdapter(cfg.AuthService), cfg.BaseURL, []byte(cfg.JWTSecret))

	// Rate limiters
	tfaRateLimiter := intMiddleware.TFAVerifyRateLimiter(cfg.TrustedProxyCIDRs...)
	loginRateLimiter := intMiddleware.LoginRateLimiter(cfg.TrustedProxyCIDRs...)
	registerRateLimiter := intMiddleware.RegisterRateLimiter(cfg.TrustedProxyCIDRs...)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.With(registerRateLimiter.Limit).Post("/register", authHandler.Register)
			r.With(loginRateLimiter.Limit).Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)

			// 2FA verification routes (public, rate-limited)
			if totpHandler != nil {
				r.Route("/2fa", func(r chi.Router) {
					r.Use(tfaRateLimiter.Limit)
					r.Post("/verify", totpHandler.Verify)
					r.Post("/recovery", totpHandler.Recovery)
				})
			}

			// OIDC routes
			r.Route("/oidc", func(r chi.Router) {
				r.Get("/config", oidcHandler.Config)
				r.Get("/login", oidcHandler.Login)
				r.Get("/callback", oidcHandler.Callback)
				r.Post("/exchange", oidcHandler.ExchangeOIDCTokens)
			})
		})

		// Share routes - require authentication
		r.Route("/shares", func(r chi.Router) {
			authOpts := []intMiddleware.RequireAuthOption{}
			if cfg.APIKeyService != nil {
				authOpts = append(authOpts, intMiddleware.WithAPIKeyAuth(cfg.APIKeyService))
			}
			r.Use(intMiddleware.RequireAuth(cfg.AuthService, authOpts...))
			r.With(intMiddleware.RequireScope("shares:read")).Get("/", shareHandler.List)
			r.With(intMiddleware.RequireScope("shares:write")).Post("/", shareHandler.Create)

			r.Route("/{id}", func(r chi.Router) {
				r.With(intMiddleware.RequireScope("shares:read")).Get("/", shareHandler.Get)
				r.With(intMiddleware.RequireScope("shares:write")).Patch("/", shareHandler.Update)
				r.With(intMiddleware.RequireScope("shares:write")).Delete("/", shareHandler.Delete)
				r.With(intMiddleware.RequireScope("shares:write")).Post("/notify", shareHandler.SendNotification)
				r.With(intMiddleware.RequireScope("shares:read")).Get("/recipients", shareHandler.ListRecipients)

				// File routes for a specific share
				r.With(intMiddleware.RequireScope("files:read")).Get("/files", fileHandler.ListByShare)
				r.With(intMiddleware.RequireScope("files:write")).Post("/files", fileHandler.Upload)
			})
		})

		// File routes - require authentication
		r.Route("/files", func(r chi.Router) {
			authOpts := []intMiddleware.RequireAuthOption{}
			if cfg.APIKeyService != nil {
				authOpts = append(authOpts, intMiddleware.WithAPIKeyAuth(cfg.APIKeyService))
			}
			r.Use(intMiddleware.RequireAuth(cfg.AuthService, authOpts...))
			r.With(intMiddleware.RequireScope("files:write")).Delete("/{id}", fileHandler.Delete)
		})

		// User profile routes - require authentication
		r.Route("/me", func(r chi.Router) {
			r.Use(intMiddleware.RequireAuth(cfg.AuthService))
			r.Get("/", userHandler.GetProfile)
			r.Patch("/", userHandler.UpdateProfile)
			r.Put("/password", userHandler.UpdatePassword)

			// OIDC linking routes
			r.Route("/oidc", func(r chi.Router) {
				r.Get("/link", oidcHandler.Link)
				r.Get("/callback", oidcHandler.LinkCallback)
				r.Delete("/", oidcHandler.Unlink)
			})

			// 2FA management routes
			if totpHandler != nil {
				r.Route("/2fa", func(r chi.Router) {
					r.Get("/status", totpHandler.GetStatus)
					r.Post("/setup", totpHandler.BeginSetup)
					r.Post("/confirm", totpHandler.ConfirmSetup)
					r.Post("/disable", totpHandler.Disable)
					r.Post("/recovery-codes", totpHandler.RegenerateRecoveryCodes)
				})
			}
		})

		// Admin routes - require authentication and admin role
		r.Route("/admin", func(r chi.Router) {
			r.Use(intMiddleware.RequireAuth(cfg.AuthService))
			r.Use(intMiddleware.RequireAdmin)
			r.Route("/users", func(r chi.Router) {
				r.Get("/", adminHandler.ListUsers)
				r.Post("/", adminHandler.CreateUser)
				r.Get("/{id}", adminHandler.GetUser)
				r.Patch("/{id}", adminHandler.UpdateUser)
				r.Delete("/{id}", adminHandler.DeleteUser)
			})
			if storageConfigHandler != nil {
				r.Route("/storage", func(r chi.Router) {
					r.Get("/", storageConfigHandler.GetStorageConfig)
					r.Put("/", storageConfigHandler.UpdateStorageConfig)
					r.Delete("/", storageConfigHandler.DeleteStorageConfig)
					r.Post("/test", storageConfigHandler.TestStorageConnection)
				})
			}
			if fileRestrictionsHandler != nil {
				r.Route("/files", func(r chi.Router) {
					r.Get("/", fileRestrictionsHandler.GetFileRestrictions)
					r.Put("/", fileRestrictionsHandler.UpdateFileRestrictions)
					r.Delete("/", fileRestrictionsHandler.DeleteFileRestrictions)
				})
			}
			if smtpConfigHandler != nil {
				r.Route("/smtp", func(r chi.Router) {
					r.Get("/", smtpConfigHandler.GetSMTPConfig)
					r.Put("/", smtpConfigHandler.UpdateSMTPConfig)
					r.Delete("/", smtpConfigHandler.DeleteSMTPConfig)
				})
			}
			if apiKeyHandler != nil {
				r.Route("/api-keys", func(r chi.Router) {
					r.Get("/", apiKeyHandler.List)
					r.Post("/", apiKeyHandler.Create)
					r.Delete("/{id}", apiKeyHandler.Revoke)
				})
			}
			if webhookHandler != nil {
				r.Route("/webhooks", func(r chi.Router) {
					r.Get("/", webhookHandler.ListSubscriptions)
					r.Post("/", webhookHandler.CreateSubscription)
					r.Get("/events", webhookHandler.ListEvents)
					r.Get("/deliveries", webhookHandler.ListDeliveries)
					r.Patch("/{id}", webhookHandler.UpdateSubscription)
					r.Delete("/{id}", webhookHandler.DeleteSubscription)
				})
			}
		})
	})

	// Public share access routes (no auth required)
	r.Route("/s/{slug}", func(r chi.Router) {
		r.Get("/", publicHandler.ViewShare)
		r.Post("/verify", publicHandler.VerifyPassword)
		r.Get("/files/{fileId}", publicHandler.DownloadFile)
		r.Get("/files/{fileId}/preview", publicHandler.PreviewFile)
		r.Post("/upload", publicHandler.UploadToReverseShare)
	})

	// Swagger UI (API documentation)
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Serve frontend (catch-all)
	if cfg.FrontendFS != nil {
		frontendHandler, err := NewFrontendHandler(cfg.FrontendFS)
		if err == nil {
			r.NotFound(frontendHandler.ServeHTTP)
		}
	}

	return r
}

// healthHandler returns the health status of the service.
//
//	@Summary		Health check
//	@Description	Returns the application health status and feature flags. Used by load balancers, container orchestrators, and the frontend to verify the service is running and discover available features.
//	@Tags			system
//	@Produce	json
//	@Success	200	{object}	APIResponse
//	@Router		/health [get]
func healthHandler(emailService *service.EmailService, e2eEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		emailConfigured := emailService != nil && emailService.IsConfigured()
		Success(w, http.StatusOK, map[string]interface{}{
			"status":                 "ok",
			"email_configured":       emailConfigured,
			"e2e_encryption_enabled": e2eEnabled,
		})
	}
}

// oidcServiceAdapter adapts *service.OIDCService to OIDCServiceInterface.
type oidcServiceAdapter struct {
	svc *service.OIDCService
}

func newOIDCServiceAdapter(svc *service.OIDCService) OIDCServiceInterface {
	if svc == nil {
		return nil
	}
	return &oidcServiceAdapter{svc: svc}
}

// IsEnabled reports whether OIDC is configured and available.
func (a *oidcServiceAdapter) IsEnabled() bool {
	return a.svc != nil && a.svc.IsEnabled()
}

// GenerateState creates an OIDC state value for a new auth flow.
func (a *oidcServiceAdapter) GenerateState() (string, error) {
	return a.svc.GenerateState()
}

// GenerateCodeVerifier creates a PKCE code verifier for a new auth flow.
func (a *oidcServiceAdapter) GenerateCodeVerifier() (string, error) {
	return a.svc.GenerateCodeVerifier()
}

// GetAuthURL returns the OIDC authorization URL for sign-in.
func (a *oidcServiceAdapter) GetAuthURL(state, codeVerifier string) string {
	return a.svc.GetAuthURL(state, codeVerifier)
}

// GetLinkAuthURL returns the OIDC authorization URL for account linking.
func (a *oidcServiceAdapter) GetLinkAuthURL(state, codeVerifier string) string {
	return a.svc.GetLinkAuthURL(state, codeVerifier)
}

// ExchangeCode exchanges an authorization code for OIDC user information.
func (a *oidcServiceAdapter) ExchangeCode(ctx context.Context, code, codeVerifier string) (*OIDCUserInfo, error) {
	info, err := a.svc.ExchangeCode(ctx, code, codeVerifier)
	if err != nil {
		return nil, err
	}
	return &OIDCUserInfo{
		Subject:     info.Subject,
		Email:       info.Email,
		DisplayName: info.DisplayName,
		Issuer:      info.Issuer,
	}, nil
}

// FindOrCreateUser resolves the OIDC identity to a local user account.
func (a *oidcServiceAdapter) FindOrCreateUser(ctx context.Context, info *OIDCUserInfo) (*OIDCUser, error) {
	svcInfo := &service.OIDCUserInfo{
		Subject:     info.Subject,
		Email:       info.Email,
		DisplayName: info.DisplayName,
		Issuer:      info.Issuer,
	}
	user, err := a.svc.FindOrCreateUser(ctx, svcInfo)
	if err != nil {
		return nil, err
	}
	return &OIDCUser{
		ID:      user.ID,
		IsAdmin: user.IsAdmin,
	}, nil
}

// LinkOIDC links an OIDC identity to an existing local user.
func (a *oidcServiceAdapter) LinkOIDC(ctx context.Context, userID string, info *OIDCUserInfo) error {
	svcInfo := &service.OIDCUserInfo{
		Subject:     info.Subject,
		Email:       info.Email,
		DisplayName: info.DisplayName,
		Issuer:      info.Issuer,
	}
	return a.svc.LinkOIDC(ctx, userID, svcInfo)
}

// UnlinkOIDC removes the linked OIDC identity from a local user.
func (a *oidcServiceAdapter) UnlinkOIDC(ctx context.Context, userID string) error {
	return a.svc.UnlinkOIDC(ctx, userID)
}

// authTokenAdapter adapts *service.AuthService to AuthTokenServiceInterface.
type authTokenAdapter struct {
	svc *service.AuthService
}

func newAuthTokenAdapter(svc *service.AuthService) AuthTokenServiceInterface {
	if svc == nil {
		return nil
	}
	return &authTokenAdapter{svc: svc}
}

// GenerateTokensForUser creates an access and refresh token pair for a user.
func (a *authTokenAdapter) GenerateTokensForUser(userID string, isAdmin bool) (*TokenPair, error) {
	tokens, err := a.svc.GenerateTokensForUser(userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

// totpServiceAdapterImpl adapts *service.TOTPService to TOTPServiceInterface.
type totpServiceAdapterImpl struct {
	svc *service.TOTPService
}

func newTOTPServiceAdapter(svc *service.TOTPService) TOTPServiceInterface {
	if svc == nil {
		return nil
	}
	return &totpServiceAdapterImpl{svc: svc}
}

// BeginSetup starts TOTP enrollment for the user and returns setup details.
func (a *totpServiceAdapterImpl) BeginSetup(ctx context.Context, userID string) (string, string, string, error) {
	return a.svc.BeginSetup(ctx, userID)
}

// ConfirmSetup confirms TOTP enrollment and returns recovery codes.
func (a *totpServiceAdapterImpl) ConfirmSetup(ctx context.Context, userID, code string) ([]string, error) {
	return a.svc.ConfirmSetup(ctx, userID, code)
}

// Verify validates a TOTP code for the user.
func (a *totpServiceAdapterImpl) Verify(ctx context.Context, userID, code string) error {
	return a.svc.Verify(ctx, userID, code)
}

// VerifyRecoveryCode validates a recovery code for the user.
func (a *totpServiceAdapterImpl) VerifyRecoveryCode(ctx context.Context, userID, code string) error {
	return a.svc.VerifyRecoveryCode(ctx, userID, code)
}

// Disable turns off TOTP for the user.
func (a *totpServiceAdapterImpl) Disable(ctx context.Context, userID string) error {
	return a.svc.Disable(ctx, userID)
}

// RegenerateRecoveryCodes replaces the user's recovery codes.
func (a *totpServiceAdapterImpl) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	return a.svc.RegenerateRecoveryCodes(ctx, userID)
}

// GetStatus reports whether TOTP is enabled for the user.
func (a *totpServiceAdapterImpl) GetStatus(ctx context.Context, userID string) (bool, error) {
	return a.svc.GetStatus(ctx, userID)
}

// GeneratePendingToken creates a temporary token for a pending 2FA challenge.
func (a *totpServiceAdapterImpl) GeneratePendingToken(userID string, isAdmin bool) (string, error) {
	return a.svc.GeneratePendingToken(userID, isAdmin)
}

// ValidatePendingToken parses and validates a pending 2FA challenge token.
func (a *totpServiceAdapterImpl) ValidatePendingToken(tokenStr string) (*service.Claims, error) {
	return a.svc.ValidatePendingToken(tokenStr)
}

// passwordVerifierAdapter adapts *service.AuthService to PasswordVerifier.
type passwordVerifierAdapter struct {
	svc *service.AuthService
}

func newPasswordVerifierAdapter(svc *service.AuthService) PasswordVerifier {
	if svc == nil {
		return nil
	}
	return &passwordVerifierAdapter{svc: svc}
}

// VerifyPassword checks the supplied password for the user.
func (a *passwordVerifierAdapter) VerifyPassword(ctx context.Context, userID, password string) error {
	return a.svc.VerifyPassword(ctx, userID, password)
}
