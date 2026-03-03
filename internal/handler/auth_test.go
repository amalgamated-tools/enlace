package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/enlace/internal/handler"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// AuthServiceInterface defines the interface for auth operations.
// This allows us to use a mock in tests.
type AuthServiceInterface interface {
	Register(ctx context.Context, email, password, displayName string) (*model.User, error)
	Login(ctx context.Context, email, password string) (*service.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*service.TokenPair, error)
	GetUser(ctx context.Context, userID string) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
}

// mockAuthService implements AuthServiceInterface for testing.
type mockAuthService struct {
	registerFn       func(ctx context.Context, email, password, displayName string) (*model.User, error)
	loginFn          func(ctx context.Context, email, password string) (*service.TokenPair, error)
	refreshTokensFn  func(ctx context.Context, refreshToken string) (*service.TokenPair, error)
	getUserFn        func(ctx context.Context, userID string) (*model.User, error)
	getUserByEmailFn func(ctx context.Context, email string) (*model.User, error)
}

func (m *mockAuthService) Register(ctx context.Context, email, password, displayName string) (*model.User, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, email, password, displayName)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (*service.TokenPair, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, email, password)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) RefreshTokens(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
	if m.refreshTokensFn != nil {
		return m.refreshTokensFn(ctx, refreshToken)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return nil, errors.New("not implemented")
}

// Helper function to create a new router with auth routes for testing.
func setupAuthRouter(h *handler.AuthHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
	})
	return r
}

func TestAuthHandler_Register_Success(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, email, password, displayName string) (*model.User, error) {
			return &model.User{
				ID:          "user-123",
				Email:       email,
				DisplayName: displayName,
			}, nil
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"email": "user@example.com", "password": "password123", "display_name": "Test User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.ID != "user-123" {
		t.Errorf("expected ID user-123, got %s", response.Data.ID)
	}
	if response.Data.Email != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", response.Data.Email)
	}
	if response.Data.DisplayName != "Test User" {
		t.Errorf("expected display_name Test User, got %s", response.Data.DisplayName)
	}
}

func TestAuthHandler_Register_ValidationErrors(t *testing.T) {
	mock := &mockAuthService{}
	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "missing email",
			body:      `{"password": "password123", "display_name": "Test User"}`,
			wantField: "email",
		},
		{
			name:      "invalid email",
			body:      `{"email": "invalid-email", "password": "password123", "display_name": "Test User"}`,
			wantField: "email",
		},
		{
			name:      "missing password",
			body:      `{"email": "user@example.com", "display_name": "Test User"}`,
			wantField: "password",
		},
		{
			name:      "short password",
			body:      `{"email": "user@example.com", "password": "short", "display_name": "Test User"}`,
			wantField: "password",
		},
		{
			name:      "missing display_name",
			body:      `{"email": "user@example.com", "password": "password123"}`,
			wantField: "display_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}

			var response struct {
				Success bool              `json:"success"`
				Error   string            `json:"error"`
				Fields  map[string]string `json:"fields"`
			}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response.Success {
				t.Error("expected success to be false")
			}
			if _, ok := response.Fields[tt.wantField]; !ok {
				t.Errorf("expected field error for %s, got fields: %v", tt.wantField, response.Fields)
			}
		})
	}
}

func TestAuthHandler_Register_EmailExists(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, email, password, displayName string) (*model.User, error) {
			return nil, service.ErrEmailExists
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"email": "existing@example.com", "password": "password123", "display_name": "Test User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success to be false")
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	mock := &mockAuthService{}
	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	mock := &mockAuthService{
		loginFn: func(ctx context.Context, email, password string) (*service.TokenPair, error) {
			return &service.TokenPair{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-456",
			}, nil
		},
		getUserByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{
				ID:          "user-123",
				Email:       email,
				DisplayName: "Test User",
			}, nil
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"email": "user@example.com", "password": "password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			User         struct {
				ID          string `json:"id"`
				Email       string `json:"email"`
				DisplayName string `json:"display_name"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.AccessToken != "access-token-123" {
		t.Errorf("expected access_token access-token-123, got %s", response.Data.AccessToken)
	}
	if response.Data.RefreshToken != "refresh-token-456" {
		t.Errorf("expected refresh_token refresh-token-456, got %s", response.Data.RefreshToken)
	}
}

func TestAuthHandler_Login_ValidationErrors(t *testing.T) {
	mock := &mockAuthService{}
	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "missing email",
			body:      `{"password": "password123"}`,
			wantField: "email",
		},
		{
			name:      "invalid email",
			body:      `{"email": "invalid-email", "password": "password123"}`,
			wantField: "email",
		},
		{
			name:      "missing password",
			body:      `{"email": "user@example.com"}`,
			wantField: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}

			var response struct {
				Success bool              `json:"success"`
				Error   string            `json:"error"`
				Fields  map[string]string `json:"fields"`
			}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response.Success {
				t.Error("expected success to be false")
			}
			if _, ok := response.Fields[tt.wantField]; !ok {
				t.Errorf("expected field error for %s, got fields: %v", tt.wantField, response.Fields)
			}
		})
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	mock := &mockAuthService{
		loginFn: func(ctx context.Context, email, password string) (*service.TokenPair, error) {
			return nil, service.ErrInvalidCredentials
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"email": "user@example.com", "password": "wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success to be false")
	}
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	mock := &mockAuthService{
		refreshTokensFn: func(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
			return &service.TokenPair{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
			}, nil
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"refresh_token": "old-refresh-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.AccessToken != "new-access-token" {
		t.Errorf("expected access_token new-access-token, got %s", response.Data.AccessToken)
	}
	if response.Data.RefreshToken != "new-refresh-token" {
		t.Errorf("expected refresh_token new-refresh-token, got %s", response.Data.RefreshToken)
	}
}

func TestAuthHandler_Refresh_ValidationErrors(t *testing.T) {
	mock := &mockAuthService{}
	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response struct {
		Success bool              `json:"success"`
		Error   string            `json:"error"`
		Fields  map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success to be false")
	}
	if _, ok := response.Fields["refresh_token"]; !ok {
		t.Errorf("expected field error for refresh_token, got fields: %v", response.Fields)
	}
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	mock := &mockAuthService{
		refreshTokensFn: func(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
			return nil, service.ErrInvalidToken
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"refresh_token": "invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success to be false")
	}
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	mock := &mockAuthService{}
	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
}

func TestAuthHandler_Register_InternalError(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, email, password, displayName string) (*model.User, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"email": "user@example.com", "password": "password123", "display_name": "Test User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestAuthHandler_Login_InternalError(t *testing.T) {
	mock := &mockAuthService{
		loginFn: func(ctx context.Context, email, password string) (*service.TokenPair, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"email": "user@example.com", "password": "password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestAuthHandler_Refresh_InternalError(t *testing.T) {
	mock := &mockAuthService{
		refreshTokensFn: func(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewAuthHandler(mock, nil, false)
	router := setupAuthRouter(h)

	body := `{"refresh_token": "some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestAuthHandler_Login_OIDCUserSkips2FA(t *testing.T) {
	mock := &mockAuthService{
		loginFn: func(ctx context.Context, email, password string) (*service.TokenPair, error) {
			return &service.TokenPair{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-456",
			}, nil
		},
		getUserByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{
				ID:          "user-123",
				Email:       email,
				DisplayName: "OIDC User",
				OIDCSubject: "sub-123",
				OIDCIssuer:  "https://issuer.example.com",
			}, nil
		},
	}

	// Mock TOTP service that says 2FA is enabled — should be skipped for OIDC user
	totpMock := &mockTOTPService{
		getStatusFn: func(ctx context.Context, userID string) (bool, error) {
			t.Error("GetStatus should not be called for OIDC user")
			return true, nil
		},
	}

	h := handler.NewAuthHandler(mock, totpMock, false)
	router := setupAuthRouter(h)

	body := `{"email": "oidc@example.com", "password": "password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			Requires2FA  bool   `json:"requires_2fa"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.AccessToken != "access-token-123" {
		t.Errorf("expected access_token, got %s", response.Data.AccessToken)
	}
	if response.Data.Requires2FA {
		t.Error("expected requires_2fa to be false for OIDC user")
	}
}

func TestAuthHandler_Login_OIDCUserSkipsRequire2FASetup(t *testing.T) {
	mock := &mockAuthService{
		loginFn: func(ctx context.Context, email, password string) (*service.TokenPair, error) {
			return &service.TokenPair{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-456",
			}, nil
		},
		getUserByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{
				ID:          "user-123",
				Email:       email,
				DisplayName: "OIDC User",
				OIDCSubject: "sub-123",
				OIDCIssuer:  "https://issuer.example.com",
			}, nil
		},
	}

	// require2FA is true, but should be skipped for OIDC user
	h := handler.NewAuthHandler(mock, nil, true)
	router := setupAuthRouter(h)

	body := `{"email": "oidc@example.com", "password": "password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken      string `json:"access_token"`
			Requires2FASetup bool   `json:"requires_2fa_setup"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.AccessToken != "access-token-123" {
		t.Errorf("expected access_token, got %s", response.Data.AccessToken)
	}
	if response.Data.Requires2FASetup {
		t.Error("expected requires_2fa_setup to be false for OIDC user")
	}
}
