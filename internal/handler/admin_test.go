package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/sharer/internal/handler"
	"github.com/amalgamated-tools/sharer/internal/middleware"
	"github.com/amalgamated-tools/sharer/internal/model"
	"github.com/amalgamated-tools/sharer/internal/repository"
)

// mockUserRepository implements handler.UserRepositoryInterface for testing.
type mockUserRepository struct {
	listFn        func(ctx context.Context) ([]*model.User, error)
	createFn      func(ctx context.Context, user *model.User) error
	getByIDFn     func(ctx context.Context, id string) (*model.User, error)
	getByEmailFn  func(ctx context.Context, email string) (*model.User, error)
	updateFn      func(ctx context.Context, user *model.User) error
	deleteFn      func(ctx context.Context, id string) error
	emailExistsFn func(ctx context.Context, email string) (bool, error)
}

func (m *mockUserRepository) List(ctx context.Context) ([]*model.User, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) Create(ctx context.Context, user *model.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	return errors.New("not implemented")
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) Update(ctx context.Context, user *model.User) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
	return errors.New("not implemented")
}

func (m *mockUserRepository) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errors.New("not implemented")
}

func (m *mockUserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	if m.emailExistsFn != nil {
		return m.emailExistsFn(ctx, email)
	}
	return false, errors.New("not implemented")
}

// Helper function to create a router with admin routes for testing.
func setupAdminRouter(h *handler.AdminHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/admin/users", func(r chi.Router) {
		r.Get("/", h.ListUsers)
		r.Post("/", h.CreateUser)
		r.Get("/{id}", h.GetUser)
		r.Patch("/{id}", h.UpdateUser)
		r.Delete("/{id}", h.DeleteUser)
	})
	return r
}

// withAdminRequestContext adds admin user context to a request.
func withAdminRequestContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, "admin-123")
	ctx = context.WithValue(ctx, middleware.IsAdminKey, true)
	return r.WithContext(ctx)
}

// --- ListUsers Tests ---

func TestAdminHandler_ListUsers_Success(t *testing.T) {
	now := time.Now()
	mock := &mockUserRepository{
		listFn: func(ctx context.Context) ([]*model.User, error) {
			return []*model.User{
				{
					ID:          "user-1",
					Email:       "user1@example.com",
					DisplayName: "User One",
					IsAdmin:     false,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
				{
					ID:          "user-2",
					Email:       "user2@example.com",
					DisplayName: "User Two",
					IsAdmin:     true,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			}, nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			IsAdmin     bool   `json:"is_admin"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if len(response.Data) != 2 {
		t.Errorf("expected 2 users, got %d", len(response.Data))
	}
	if response.Data[0].Email != "user1@example.com" {
		t.Errorf("expected first user email user1@example.com, got %s", response.Data[0].Email)
	}
}

func TestAdminHandler_ListUsers_EmptyList(t *testing.T) {
	mock := &mockUserRepository{
		listFn: func(ctx context.Context) ([]*model.User, error) {
			return []*model.User{}, nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool          `json:"success"`
		Data    []interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if len(response.Data) != 0 {
		t.Errorf("expected 0 users, got %d", len(response.Data))
	}
}

func TestAdminHandler_ListUsers_DatabaseError(t *testing.T) {
	mock := &mockUserRepository{
		listFn: func(ctx context.Context) ([]*model.User, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- CreateUser Tests ---

func TestAdminHandler_CreateUser_Success(t *testing.T) {
	mock := &mockUserRepository{
		emailExistsFn: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		createFn: func(ctx context.Context, user *model.User) error {
			// Simulate setting timestamps
			user.CreatedAt = time.Now()
			user.UpdatedAt = time.Now()
			return nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	body := `{"email": "newuser@example.com", "password": "password123", "display_name": "New User", "is_admin": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			IsAdmin     bool   `json:"is_admin"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Email != "newuser@example.com" {
		t.Errorf("expected email newuser@example.com, got %s", response.Data.Email)
	}
	if response.Data.DisplayName != "New User" {
		t.Errorf("expected display_name New User, got %s", response.Data.DisplayName)
	}
}

func TestAdminHandler_CreateUser_EmailExists(t *testing.T) {
	mock := &mockUserRepository{
		emailExistsFn: func(ctx context.Context, email string) (bool, error) {
			return true, nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	body := `{"email": "existing@example.com", "password": "password123", "display_name": "Existing User", "is_admin": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestAdminHandler_CreateUser_ValidationErrors(t *testing.T) {
	mock := &mockUserRepository{}
	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

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
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withAdminRequestContext(req)
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

func TestAdminHandler_CreateUser_InvalidJSON(t *testing.T) {
	mock := &mockUserRepository{}
	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// --- GetUser Tests ---

func TestAdminHandler_GetUser_Success(t *testing.T) {
	now := time.Now()
	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			if id == "user-123" {
				return &model.User{
					ID:          "user-123",
					Email:       "user@example.com",
					DisplayName: "Test User",
					IsAdmin:     false,
					CreatedAt:   now,
					UpdatedAt:   now,
				}, nil
			}
			return nil, repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/user-123", nil)
	req = withAdminRequestContext(req)
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
			IsAdmin     bool   `json:"is_admin"`
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
}

func TestAdminHandler_GetUser_NotFound(t *testing.T) {
	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return nil, repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/nonexistent", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// --- UpdateUser Tests ---

func TestAdminHandler_UpdateUser_Success(t *testing.T) {
	now := time.Now()
	existingUser := &model.User{
		ID:           "user-123",
		Email:        "user@example.com",
		PasswordHash: "hashedpassword",
		DisplayName:  "Old Name",
		IsAdmin:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			if id == "user-123" {
				return existingUser, nil
			}
			return nil, repository.ErrNotFound
		},
		emailExistsFn: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		getByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return nil, repository.ErrNotFound
		},
		updateFn: func(ctx context.Context, user *model.User) error {
			return nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	body := `{"display_name": "New Name", "is_admin": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/user-123", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			IsAdmin     bool   `json:"is_admin"`
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
	if !response.Data.IsAdmin {
		t.Error("expected is_admin to be true")
	}
}

func TestAdminHandler_UpdateUser_EmailChange(t *testing.T) {
	now := time.Now()
	existingUser := &model.User{
		ID:           "user-123",
		Email:        "old@example.com",
		PasswordHash: "hashedpassword",
		DisplayName:  "Test User",
		IsAdmin:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			if id == "user-123" {
				return existingUser, nil
			}
			return nil, repository.ErrNotFound
		},
		getByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return nil, repository.ErrNotFound
		},
		updateFn: func(ctx context.Context, user *model.User) error {
			return nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	body := `{"email": "new@example.com"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/user-123", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Email string `json:"email"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.Email != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", response.Data.Email)
	}
}

func TestAdminHandler_UpdateUser_EmailExists(t *testing.T) {
	now := time.Now()
	existingUser := &model.User{
		ID:           "user-123",
		Email:        "old@example.com",
		PasswordHash: "hashedpassword",
		DisplayName:  "Test User",
		IsAdmin:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	otherUser := &model.User{
		ID:           "user-456",
		Email:        "taken@example.com",
		PasswordHash: "hashedpassword",
		DisplayName:  "Other User",
		IsAdmin:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			if id == "user-123" {
				return existingUser, nil
			}
			return nil, repository.ErrNotFound
		},
		getByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			if email == "taken@example.com" {
				return otherUser, nil
			}
			return nil, repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	body := `{"email": "taken@example.com"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/user-123", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

func TestAdminHandler_UpdateUser_NotFound(t *testing.T) {
	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return nil, repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	body := `{"display_name": "New Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestAdminHandler_UpdateUser_PasswordChange(t *testing.T) {
	now := time.Now()
	existingUser := &model.User{
		ID:           "user-123",
		Email:        "user@example.com",
		PasswordHash: "oldhash",
		DisplayName:  "Test User",
		IsAdmin:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	var updatedUser *model.User
	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			if id == "user-123" {
				return existingUser, nil
			}
			return nil, repository.ErrNotFound
		},
		updateFn: func(ctx context.Context, user *model.User) error {
			updatedUser = user
			return nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	body := `{"password": "newpassword123"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/user-123", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	if updatedUser == nil {
		t.Fatal("expected user to be updated")
	}
	// Password should be hashed, not the original value
	if updatedUser.PasswordHash == "oldhash" || updatedUser.PasswordHash == "newpassword123" {
		t.Error("password should have been hashed")
	}
}

func TestAdminHandler_UpdateUser_ValidationErrors(t *testing.T) {
	now := time.Now()
	existingUser := &model.User{
		ID:           "user-123",
		Email:        "user@example.com",
		PasswordHash: "hashedpassword",
		DisplayName:  "Test User",
		IsAdmin:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			if id == "user-123" {
				return existingUser, nil
			}
			return nil, repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "invalid email format",
			body:      `{"email": "not-an-email"}`,
			wantField: "email",
		},
		{
			name:      "empty email",
			body:      `{"email": ""}`,
			wantField: "email",
		},
		{
			name:      "short password",
			body:      `{"password": "short"}`,
			wantField: "password",
		},
		{
			name:      "empty display_name",
			body:      `{"display_name": ""}`,
			wantField: "display_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/user-123", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withAdminRequestContext(req)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
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

// --- DeleteUser Tests ---

func TestAdminHandler_DeleteUser_Success(t *testing.T) {
	mock := &mockUserRepository{
		deleteFn: func(ctx context.Context, id string) error {
			if id == "user-123" {
				return nil
			}
			return repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/user-123", nil)
	req = withAdminRequestContext(req)
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

func TestAdminHandler_DeleteUser_NotFound(t *testing.T) {
	mock := &mockUserRepository{
		deleteFn: func(ctx context.Context, id string) error {
			return repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/nonexistent", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestAdminHandler_DeleteUser_DatabaseError(t *testing.T) {
	mock := &mockUserRepository{
		deleteFn: func(ctx context.Context, id string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/user-123", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- Response Format Tests ---

func TestAdminHandler_ResponseDoesNotIncludePasswordHash(t *testing.T) {
	now := time.Now()
	mock := &mockUserRepository{
		getByIDFn: func(ctx context.Context, id string) (*model.User, error) {
			return &model.User{
				ID:           "user-123",
				Email:        "user@example.com",
				PasswordHash: "secrethash",
				DisplayName:  "Test User",
				IsAdmin:      false,
				CreatedAt:    now,
				UpdatedAt:    now,
			}, nil
		},
	}

	h := handler.NewAdminHandler(mock)
	router := setupAdminRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/user-123", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check that password_hash is not in the response
	body := w.Body.String()
	if bytes.Contains([]byte(body), []byte("password_hash")) {
		t.Error("response should not contain password_hash")
	}
	if bytes.Contains([]byte(body), []byte("secrethash")) {
		t.Error("response should not contain the actual password hash value")
	}
}
