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

	"github.com/amalgamated-tools/sharer/internal/handler"
	"github.com/amalgamated-tools/sharer/internal/middleware"
	"github.com/amalgamated-tools/sharer/internal/model"
	"github.com/amalgamated-tools/sharer/internal/service"
)

// UserServiceInterface defines the interface for user service operations.
type UserServiceInterface interface {
	GetUser(ctx context.Context, userID string) (*model.User, error)
	UpdateProfile(ctx context.Context, userID, displayName, email string) (*model.User, error)
	UpdatePassword(ctx context.Context, userID, oldPassword, newPassword string) error
}

// mockUserService implements UserServiceInterface for testing.
type mockUserService struct {
	getUserFn        func(ctx context.Context, userID string) (*model.User, error)
	updateProfileFn  func(ctx context.Context, userID, displayName, email string) (*model.User, error)
	updatePasswordFn func(ctx context.Context, userID, oldPassword, newPassword string) error
}

func (m *mockUserService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) UpdateProfile(ctx context.Context, userID, displayName, email string) (*model.User, error) {
	if m.updateProfileFn != nil {
		return m.updateProfileFn(ctx, userID, displayName, email)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) UpdatePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if m.updatePasswordFn != nil {
		return m.updatePasswordFn(ctx, userID, oldPassword, newPassword)
	}
	return errors.New("not implemented")
}

// Helper function to create a request with user ID in context.
func createRequestWithUserID(method, path string, body string, userID string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	return req.WithContext(ctx)
}

// Helper function to setup user router for testing.
func setupUserRouter(h *handler.UserHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/me", func(r chi.Router) {
		r.Get("/", h.GetProfile)
		r.Patch("/", h.UpdateProfile)
		r.Put("/password", h.UpdatePassword)
	})
	return r
}

func TestUserHandler_GetProfile_Success(t *testing.T) {
	mock := &mockUserService{
		getUserFn: func(ctx context.Context, userID string) (*model.User, error) {
			if userID != "user-123" {
				t.Errorf("expected userID user-123, got %s", userID)
			}
			return &model.User{
				ID:          "user-123",
				Email:       "user@example.com",
				DisplayName: "Test User",
			}, nil
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	req := createRequestWithUserID(http.MethodGet, "/api/v1/me", "", "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
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

func TestUserHandler_GetProfile_NoUserID(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	// Create request without user ID in context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUserHandler_GetProfile_UserNotFound(t *testing.T) {
	mock := &mockUserService{
		getUserFn: func(ctx context.Context, userID string) (*model.User, error) {
			return nil, service.ErrUserNotFound
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	req := createRequestWithUserID(http.MethodGet, "/api/v1/me", "", "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUserHandler_GetProfile_InternalError(t *testing.T) {
	mock := &mockUserService{
		getUserFn: func(ctx context.Context, userID string) (*model.User, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	req := createRequestWithUserID(http.MethodGet, "/api/v1/me", "", "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestUserHandler_UpdateProfile_Success(t *testing.T) {
	mock := &mockUserService{
		updateProfileFn: func(ctx context.Context, userID, displayName, email string) (*model.User, error) {
			if userID != "user-123" {
				t.Errorf("expected userID user-123, got %s", userID)
			}
			if displayName != "New Name" {
				t.Errorf("expected displayName New Name, got %s", displayName)
			}
			if email != "new@example.com" {
				t.Errorf("expected email new@example.com, got %s", email)
			}
			return &model.User{
				ID:          "user-123",
				Email:       email,
				DisplayName: displayName,
			}, nil
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"display_name": "New Name", "email": "new@example.com"}`
	req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
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
	if response.Data.DisplayName != "New Name" {
		t.Errorf("expected display_name New Name, got %s", response.Data.DisplayName)
	}
	if response.Data.Email != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", response.Data.Email)
	}
}

func TestUserHandler_UpdateProfile_PartialUpdate(t *testing.T) {
	mock := &mockUserService{
		updateProfileFn: func(ctx context.Context, userID, displayName, email string) (*model.User, error) {
			// When only display_name is provided, email should be empty
			if displayName != "New Name" {
				t.Errorf("expected displayName New Name, got %s", displayName)
			}
			return &model.User{
				ID:          "user-123",
				Email:       "existing@example.com",
				DisplayName: displayName,
			}, nil
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"display_name": "New Name"}`
	req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestUserHandler_UpdateProfile_NoUserID(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"display_name": "New Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUserHandler_UpdateProfile_InvalidJSON(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", `{invalid`, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUserHandler_UpdateProfile_ValidationErrors(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "invalid email format",
			body:      `{"email": "invalid-email"}`,
			wantField: "email",
		},
		{
			name:      "empty email",
			body:      `{"email": ""}`,
			wantField: "email",
		},
		{
			name:      "empty display_name",
			body:      `{"display_name": ""}`,
			wantField: "display_name",
		},
		{
			name:      "whitespace display_name",
			body:      `{"display_name": "   "}`,
			wantField: "display_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", tt.body, "user-123")
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

func TestUserHandler_UpdateProfile_EmailExists(t *testing.T) {
	mock := &mockUserService{
		updateProfileFn: func(ctx context.Context, userID, displayName, email string) (*model.User, error) {
			return nil, service.ErrEmailExists
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"email": "existing@example.com"}`
	req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestUserHandler_UpdateProfile_UserNotFound(t *testing.T) {
	mock := &mockUserService{
		updateProfileFn: func(ctx context.Context, userID, displayName, email string) (*model.User, error) {
			return nil, service.ErrUserNotFound
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"display_name": "New Name"}`
	req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUserHandler_UpdateProfile_InternalError(t *testing.T) {
	mock := &mockUserService{
		updateProfileFn: func(ctx context.Context, userID, displayName, email string) (*model.User, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"display_name": "New Name"}`
	req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestUserHandler_UpdatePassword_Success(t *testing.T) {
	mock := &mockUserService{
		updatePasswordFn: func(ctx context.Context, userID, oldPassword, newPassword string) error {
			if userID != "user-123" {
				t.Errorf("expected userID user-123, got %s", userID)
			}
			if oldPassword != "oldpass123" {
				t.Errorf("expected oldPassword oldpass123, got %s", oldPassword)
			}
			if newPassword != "newpass456" {
				t.Errorf("expected newPassword newpass456, got %s", newPassword)
			}
			return nil
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"old_password": "oldpass123", "new_password": "newpass456"}`
	req := createRequestWithUserID(http.MethodPut, "/api/v1/me/password", body, "user-123")
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

func TestUserHandler_UpdatePassword_NoUserID(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"old_password": "oldpass123", "new_password": "newpass456"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUserHandler_UpdatePassword_InvalidJSON(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	req := createRequestWithUserID(http.MethodPut, "/api/v1/me/password", `{invalid`, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUserHandler_UpdatePassword_ValidationErrors(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "missing old_password",
			body:      `{"new_password": "newpass456"}`,
			wantField: "old_password",
		},
		{
			name:      "missing new_password",
			body:      `{"old_password": "oldpass123"}`,
			wantField: "new_password",
		},
		{
			name:      "short new_password",
			body:      `{"old_password": "oldpass123", "new_password": "short"}`,
			wantField: "new_password",
		},
		{
			name:      "empty old_password",
			body:      `{"old_password": "", "new_password": "newpass456"}`,
			wantField: "old_password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createRequestWithUserID(http.MethodPut, "/api/v1/me/password", tt.body, "user-123")
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

func TestUserHandler_UpdatePassword_InvalidCredentials(t *testing.T) {
	mock := &mockUserService{
		updatePasswordFn: func(ctx context.Context, userID, oldPassword, newPassword string) error {
			return service.ErrInvalidCredentials
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"old_password": "wrongpassword", "new_password": "newpass456"}`
	req := createRequestWithUserID(http.MethodPut, "/api/v1/me/password", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUserHandler_UpdatePassword_UserNotFound(t *testing.T) {
	mock := &mockUserService{
		updatePasswordFn: func(ctx context.Context, userID, oldPassword, newPassword string) error {
			return service.ErrUserNotFound
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"old_password": "oldpass123", "new_password": "newpass456"}`
	req := createRequestWithUserID(http.MethodPut, "/api/v1/me/password", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUserHandler_UpdatePassword_InternalError(t *testing.T) {
	mock := &mockUserService{
		updatePasswordFn: func(ctx context.Context, userID, oldPassword, newPassword string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{"old_password": "oldpass123", "new_password": "newpass456"}`
	req := createRequestWithUserID(http.MethodPut, "/api/v1/me/password", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestUserHandler_UpdateProfile_NoFields(t *testing.T) {
	mock := &mockUserService{}
	h := handler.NewUserHandler(mock)
	router := setupUserRouter(h)

	body := `{}`
	req := createRequestWithUserID(http.MethodPatch, "/api/v1/me", body, "user-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
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
