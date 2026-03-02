package handler

import (
	"context"
	"io/fs"
	"net/http"

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
	AuthService  *service.AuthService
	ShareService *service.ShareService
	FileService  *service.FileService
	EmailService *service.EmailService

	// Repositories (for middleware/handlers that need direct access)
	UserRepo  *repository.UserRepository
	ShareRepo *repository.ShareRepository
	FileRepo  *repository.FileRepository

	// Storage
	Storage storage.Storage

	// Configuration
	JWTSecret string
	BaseURL   string

	// OIDC Service (optional)
	OIDCService *service.OIDCService

	// Frontend filesystem (embedded)
	FrontendFS fs.FS

	// Swagger/API docs
	SwaggerEnabled bool

	// CORS allowed origins (comma-separated). Defaults to BaseURL if empty.
	CORSOrigins []string
}

// NewRouter creates a new Chi router with all routes configured.
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
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
	r.Get("/health", healthHandler)

	// Create handlers
	authHandler := NewAuthHandler(cfg.AuthService)
	var shareHandler *ShareHandler
	if cfg.EmailService != nil {
		shareHandler = NewShareHandler(cfg.ShareService, cfg.FileService, cfg.EmailService, cfg.BaseURL)
	} else {
		shareHandler = NewShareHandler(cfg.ShareService, cfg.FileService, nil, cfg.BaseURL)
	}
	fileHandler := NewFileHandler(cfg.FileService, cfg.ShareService)
	userHandler := NewUserHandler(cfg.AuthService)
	adminHandler := NewAdminHandler(cfg.UserRepo)
	publicHandler := NewPublicHandler(cfg.ShareService, cfg.FileService, []byte(cfg.JWTSecret))
	oidcHandler := NewOIDCHandler(newOIDCServiceAdapter(cfg.OIDCService), newAuthTokenAdapter(cfg.AuthService), cfg.BaseURL)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)

			// OIDC routes
			r.Route("/oidc", func(r chi.Router) {
				r.Get("/config", oidcHandler.Config)
				r.Get("/login", oidcHandler.Login)
				r.Get("/callback", oidcHandler.Callback)
			})
		})

		// Share routes - require authentication
		r.Route("/shares", func(r chi.Router) {
			r.Use(intMiddleware.RequireAuth(cfg.AuthService))
			r.Get("/", shareHandler.List)
			r.Post("/", shareHandler.Create)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", shareHandler.Get)
				r.Patch("/", shareHandler.Update)
				r.Delete("/", shareHandler.Delete)
				r.Post("/notify", shareHandler.SendNotification)
				r.Get("/recipients", shareHandler.ListRecipients)

				// File routes for a specific share
				r.Get("/files", fileHandler.ListByShare)
				r.Post("/files", fileHandler.Upload)
			})
		})

		// File routes - require authentication
		r.Route("/files", func(r chi.Router) {
			r.Use(intMiddleware.RequireAuth(cfg.AuthService))
			r.Delete("/{id}", fileHandler.Delete)
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
	if cfg.SwaggerEnabled {
		r.Get("/swagger/*", httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		))
	}

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
//	@Summary	Health check
//	@Tags		system
//	@Produce	json
//	@Success	200	{object}	APIResponse
//	@Router		/health [get]
func healthHandler(w http.ResponseWriter, r *http.Request) {
	Success(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
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

func (a *oidcServiceAdapter) IsEnabled() bool {
	return a.svc != nil && a.svc.IsEnabled()
}

func (a *oidcServiceAdapter) GenerateState() (string, error) {
	return a.svc.GenerateState()
}

func (a *oidcServiceAdapter) GenerateCodeVerifier() (string, error) {
	return a.svc.GenerateCodeVerifier()
}

func (a *oidcServiceAdapter) GetAuthURL(state, codeVerifier string) string {
	return a.svc.GetAuthURL(state, codeVerifier)
}

func (a *oidcServiceAdapter) GetLinkAuthURL(state, codeVerifier string) string {
	return a.svc.GetLinkAuthURL(state, codeVerifier)
}

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

func (a *oidcServiceAdapter) LinkOIDC(ctx context.Context, userID string, info *OIDCUserInfo) error {
	svcInfo := &service.OIDCUserInfo{
		Subject:     info.Subject,
		Email:       info.Email,
		DisplayName: info.DisplayName,
		Issuer:      info.Issuer,
	}
	return a.svc.LinkOIDC(ctx, userID, svcInfo)
}

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
