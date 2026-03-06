package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
)

func setupAuthService(t *testing.T) (*service.AuthService, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	authService := service.NewAuthService(userRepo, []byte("test-secret-key-for-jwt-signing"))

	return authService, func() { db.Close() }
}

func TestRequireAuth_ValidToken(t *testing.T) {
	authService, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register and login to get a valid token
	user, err := authService.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	tokens, err := authService.Login(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	// Create a handler that checks if user info is in context
	var capturedUserID string
	var capturedIsAdmin bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = middleware.GetUserID(r.Context())
		capturedIsAdmin = middleware.GetIsAdmin(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with RequireAuth middleware
	handler := middleware.RequireAuth(authService)(testHandler)

	// Create request with valid token
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if capturedUserID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, capturedUserID)
	}
	if capturedIsAdmin != false {
		t.Errorf("expected isAdmin to be false, got %v", capturedIsAdmin)
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	authService, cleanup := setupAuthService(t)
	defer cleanup()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAuth(authService)(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if rec.Body.String() != `{"error":"missing authorization header"}` {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}

func TestRequireAuth_InvalidFormat_NoBearer(t *testing.T) {
	authService, cleanup := setupAuthService(t)
	defer cleanup()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAuth(authService)(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "sometoken")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if rec.Body.String() != `{"error":"invalid authorization format"}` {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}

func TestRequireAuth_InvalidFormat_WrongScheme(t *testing.T) {
	authService, cleanup := setupAuthService(t)
	defer cleanup()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAuth(authService)(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic sometoken")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if rec.Body.String() != `{"error":"invalid authorization format"}` {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	authService, cleanup := setupAuthService(t)
	defer cleanup()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAuth(authService)(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if rec.Body.String() != `{"error":"invalid token"}` {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	authService, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register user
	user, err := authService.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Generate expired token
	expiredToken, err := authService.GenerateAccessTokenWithExpiry(user.ID, user.IsAdmin, -1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAuth(authService)(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequireAdmin_AdminUser(t *testing.T) {
	// Create a handler that tracks if it was called
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAdmin(testHandler)

	// Create request with admin context
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	ctx = context.WithValue(ctx, middleware.IsAdminKey, true)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !handlerCalled {
		t.Error("expected handler to be called for admin user")
	}
}

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	// Create a handler that tracks if it was called
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAdmin(testHandler)

	// Create request with non-admin context
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	ctx = context.WithValue(ctx, middleware.IsAdminKey, false)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if rec.Body.String() != `{"error":"admin access required"}` {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
	if handlerCalled {
		t.Error("expected handler to NOT be called for non-admin user")
	}
}

func TestRequireAdmin_NoContextValues(t *testing.T) {
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequireAdmin(testHandler)

	// Create request without any context values
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if handlerCalled {
		t.Error("expected handler to NOT be called when no context values")
	}
}

func TestGetUserID_WithValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "test-user-id")
	userID := middleware.GetUserID(ctx)
	if userID != "test-user-id" {
		t.Errorf("expected user ID 'test-user-id', got %q", userID)
	}
}

func TestGetUserID_WithoutValue(t *testing.T) {
	ctx := context.Background()
	userID := middleware.GetUserID(ctx)
	if userID != "" {
		t.Errorf("expected empty user ID, got %q", userID)
	}
}

func TestGetUserID_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, 12345)
	userID := middleware.GetUserID(ctx)
	if userID != "" {
		t.Errorf("expected empty user ID for wrong type, got %q", userID)
	}
}

func TestGetIsAdmin_WithTrueValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.IsAdminKey, true)
	isAdmin := middleware.GetIsAdmin(ctx)
	if !isAdmin {
		t.Error("expected isAdmin to be true")
	}
}

func TestGetIsAdmin_WithFalseValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.IsAdminKey, false)
	isAdmin := middleware.GetIsAdmin(ctx)
	if isAdmin {
		t.Error("expected isAdmin to be false")
	}
}

func TestGetIsAdmin_WithoutValue(t *testing.T) {
	ctx := context.Background()
	isAdmin := middleware.GetIsAdmin(ctx)
	if isAdmin {
		t.Error("expected isAdmin to be false when not set")
	}
}

func TestGetIsAdmin_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.IsAdminKey, "true")
	isAdmin := middleware.GetIsAdmin(ctx)
	if isAdmin {
		t.Error("expected isAdmin to be false for wrong type")
	}
}

func TestRequireAuthAndRequireAdmin_ChainedMiddleware(t *testing.T) {
	authService, cleanup := setupAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Register user
	_, err := authService.Register(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	tokens, err := authService.Login(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Chain RequireAuth -> RequireAdmin -> testHandler
	handler := middleware.RequireAuth(authService)(middleware.RequireAdmin(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should fail because user is not admin
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if handlerCalled {
		t.Error("expected handler to NOT be called for non-admin user")
	}
}

func TestRequireAuth_APIKeyWithScope(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	apiKeyRepo := repository.NewAPIKeyRepository(db.DB())
	authService := service.NewAuthService(userRepo, []byte("test-secret-key-for-jwt-signing"))
	apiKeyService := service.NewAPIKeyService(apiKeyRepo)

	ctx := context.Background()
	user, err := authService.Register(ctx, "scope-ok@example.com", "password123", "Scope OK")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	_, token, err := apiKeyService.Create(ctx, user.ID, "scope-ok", []string{"shares:read"})
	if err != nil {
		t.Fatalf("failed to create api key: %v", err)
	}

	var authType string
	handler := middleware.RequireAuth(authService, middleware.WithAPIKeyAuth(apiKeyService))(
		middleware.RequireScope("shares:read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authType = middleware.GetAuthType(r.Context())
			w.WriteHeader(http.StatusOK)
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/shares", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if authType != "api_key" {
		t.Fatalf("expected auth type api_key, got %q", authType)
	}
}

func TestRequireAuth_APIKeyInsufficientScope(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db.DB())
	apiKeyRepo := repository.NewAPIKeyRepository(db.DB())
	authService := service.NewAuthService(userRepo, []byte("test-secret-key-for-jwt-signing"))
	apiKeyService := service.NewAPIKeyService(apiKeyRepo)

	ctx := context.Background()
	user, err := authService.Register(ctx, "scope-no@example.com", "password123", "Scope NO")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	_, token, err := apiKeyService.Create(ctx, user.ID, "scope-no", []string{"shares:read"})
	if err != nil {
		t.Fatalf("failed to create api key: %v", err)
	}

	handler := middleware.RequireAuth(authService, middleware.WithAPIKeyAuth(apiKeyService))(
		middleware.RequireScope("shares:write")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	req := httptest.NewRequest(http.MethodPost, "/shares", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}
