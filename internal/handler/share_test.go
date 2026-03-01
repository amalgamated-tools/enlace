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
	"github.com/amalgamated-tools/sharer/internal/service"
)

// mockShareService implements ShareServiceInterface for testing.
type mockShareService struct {
	createFn        func(ctx context.Context, input service.CreateShareInput) (*model.Share, error)
	getByIDFn       func(ctx context.Context, id string) (*model.Share, error)
	updateFn        func(ctx context.Context, id string, input service.UpdateShareInput) (*model.Share, error)
	deleteFn        func(ctx context.Context, id string) error
	listByCreatorFn func(ctx context.Context, creatorID string) ([]*model.Share, error)
}

func (m *mockShareService) Create(ctx context.Context, input service.CreateShareInput) (*model.Share, error) {
	if m.createFn != nil {
		return m.createFn(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockShareService) GetByID(ctx context.Context, id string) (*model.Share, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockShareService) Update(ctx context.Context, id string, input service.UpdateShareInput) (*model.Share, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockShareService) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errors.New("not implemented")
}

func (m *mockShareService) ListByCreator(ctx context.Context, creatorID string) ([]*model.Share, error) {
	if m.listByCreatorFn != nil {
		return m.listByCreatorFn(ctx, creatorID)
	}
	return nil, errors.New("not implemented")
}

// withUserContext adds a user ID to the request context.
func withUserContext(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

// setupShareRouter creates a router with share routes for testing.
func setupShareRouter(h *handler.ShareHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/shares", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
	})
	return r
}

// newTestShare creates a test share with default values.
func newTestShare(id string, creatorID string) *model.Share {
	creator := creatorID
	now := time.Now()
	return &model.Share{
		ID:             id,
		CreatorID:      &creator,
		Slug:           "test-slug",
		Name:           "Test Share",
		Description:    "A test share",
		PasswordHash:   nil,
		ExpiresAt:      nil,
		MaxDownloads:   nil,
		DownloadCount:  0,
		MaxViews:       nil,
		ViewCount:      0,
		IsReverseShare: false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestShareHandler_List_Success(t *testing.T) {
	userID := "user-123"
	shares := []*model.Share{
		newTestShare("share-1", userID),
		newTestShare("share-2", userID),
	}
	shares[1].Slug = "another-slug"

	mockShare := &mockShareService{
		listByCreatorFn: func(ctx context.Context, creatorID string) ([]*model.Share, error) {
			if creatorID != userID {
				t.Errorf("expected creatorID %s, got %s", userID, creatorID)
			}
			return shares, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if len(response.Data) != 2 {
		t.Errorf("expected 2 shares, got %d", len(response.Data))
	}
}

func TestShareHandler_List_EmptyList(t *testing.T) {
	userID := "user-123"

	mockShare := &mockShareService{
		listByCreatorFn: func(ctx context.Context, creatorID string) ([]*model.Share, error) {
			return []*model.Share{}, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	req = withUserContext(req, userID)
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
		t.Errorf("expected 0 shares, got %d", len(response.Data))
	}
}

func TestShareHandler_List_Unauthenticated(t *testing.T) {
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	// No user context
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestShareHandler_List_InternalError(t *testing.T) {
	userID := "user-123"

	mockShare := &mockShareService{
		listByCreatorFn: func(ctx context.Context, creatorID string) ([]*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestShareHandler_Create_Success(t *testing.T) {
	userID := "user-123"
	share := newTestShare("share-123", userID)

	mockShare := &mockShareService{
		createFn: func(ctx context.Context, input service.CreateShareInput) (*model.Share, error) {
			if input.CreatorID != userID {
				t.Errorf("expected creatorID %s, got %s", userID, input.CreatorID)
			}
			if input.Name != "My Share" {
				t.Errorf("expected name 'My Share', got %s", input.Name)
			}
			share.Name = input.Name
			share.Description = input.Description
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "My Share", "description": "A test share"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.ID != "share-123" {
		t.Errorf("expected ID share-123, got %s", response.Data.ID)
	}
	if response.Data.Name != "My Share" {
		t.Errorf("expected name 'My Share', got %s", response.Data.Name)
	}
}

func TestShareHandler_Create_WithAllFields(t *testing.T) {
	userID := "user-123"
	share := newTestShare("share-123", userID)
	maxDownloads := 10
	maxViews := 100
	share.MaxDownloads = &maxDownloads
	share.MaxViews = &maxViews

	mockShare := &mockShareService{
		createFn: func(ctx context.Context, input service.CreateShareInput) (*model.Share, error) {
			if input.Slug != "custom-slug" {
				t.Errorf("expected slug 'custom-slug', got %s", input.Slug)
			}
			if input.Password == nil || *input.Password != "secret" {
				t.Error("expected password to be 'secret'")
			}
			if input.MaxDownloads == nil || *input.MaxDownloads != 10 {
				t.Error("expected max_downloads to be 10")
			}
			if input.MaxViews == nil || *input.MaxViews != 100 {
				t.Error("expected max_views to be 100")
			}
			if !input.IsReverseShare {
				t.Error("expected is_reverse_share to be true")
			}
			share.Slug = input.Slug
			share.IsReverseShare = input.IsReverseShare
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{
		"name": "My Share",
		"description": "A test share",
		"slug": "custom-slug",
		"password": "secret",
		"expires_at": "2024-12-31T23:59:59Z",
		"max_downloads": 10,
		"max_views": 100,
		"is_reverse_share": true
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestShareHandler_Create_ValidationErrors(t *testing.T) {
	userID := "user-123"
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "missing name",
			body:      `{"description": "test"}`,
			wantField: "name",
		},
		{
			name:      "empty name",
			body:      `{"name": ""}`,
			wantField: "name",
		},
		{
			name:      "slug too short",
			body:      `{"name": "Test", "slug": "ab"}`,
			wantField: "slug",
		},
		{
			name:      "slug invalid chars",
			body:      `{"name": "Test", "slug": "Invalid_Slug!"}`,
			wantField: "slug",
		},
		{
			name:      "negative max_downloads",
			body:      `{"name": "Test", "max_downloads": -1}`,
			wantField: "max_downloads",
		},
		{
			name:      "negative max_views",
			body:      `{"name": "Test", "max_views": -1}`,
			wantField: "max_views",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withUserContext(req, userID)
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

func TestShareHandler_Create_InvalidExpiresAt(t *testing.T) {
	userID := "user-123"
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test", "expires_at": "invalid-date"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := response.Fields["expires_at"]; !ok {
		t.Errorf("expected field error for expires_at, got fields: %v", response.Fields)
	}
}

func TestShareHandler_Create_SlugExists(t *testing.T) {
	userID := "user-123"

	mockShare := &mockShareService{
		createFn: func(ctx context.Context, input service.CreateShareInput) (*model.Share, error) {
			return nil, service.ErrSlugExists
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test", "slug": "existing-slug"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestShareHandler_Create_Unauthenticated(t *testing.T) {
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No user context
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestShareHandler_Create_InvalidJSON(t *testing.T) {
	userID := "user-123"
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestShareHandler_Get_Success(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			if id != shareID {
				t.Errorf("expected id %s, got %s", shareID, id)
			}
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.ID != shareID {
		t.Errorf("expected ID %s, got %s", shareID, response.Data.ID)
	}
}

func TestShareHandler_Get_NotFound(t *testing.T) {
	userID := "user-123"
	shareID := "nonexistent"

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestShareHandler_Get_NotOwner(t *testing.T) {
	userID := "user-123"
	otherUserID := "user-456"
	shareID := "share-123"
	share := newTestShare(shareID, otherUserID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 instead of 403 for info hiding
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestShareHandler_Get_Unauthenticated(t *testing.T) {
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares/share-123", nil)
	// No user context
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestShareHandler_Update_Success(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)
	updatedShare := newTestShare(shareID, userID)
	updatedShare.Name = "Updated Name"
	updatedShare.Description = "Updated description"

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
		updateFn: func(ctx context.Context, id string, input service.UpdateShareInput) (*model.Share, error) {
			if id != shareID {
				t.Errorf("expected id %s, got %s", shareID, id)
			}
			if input.Name == nil || *input.Name != "Updated Name" {
				t.Error("expected name to be 'Updated Name'")
			}
			return updatedShare, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Updated Name", "description": "Updated description"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+shareID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %s", response.Data.Name)
	}
}

func TestShareHandler_Update_ClearPassword(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	var capturedInput service.UpdateShareInput
	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
		updateFn: func(ctx context.Context, id string, input service.UpdateShareInput) (*model.Share, error) {
			capturedInput = input
			updatedShare := newTestShare(shareID, userID)
			return updatedShare, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"clear_password": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+shareID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !capturedInput.ClearPassword {
		t.Error("expected ClearPassword to be true")
	}
}

func TestShareHandler_Update_NotFound(t *testing.T) {
	userID := "user-123"
	shareID := "nonexistent"

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+shareID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestShareHandler_Update_NotOwner(t *testing.T) {
	userID := "user-123"
	otherUserID := "user-456"
	shareID := "share-123"
	share := newTestShare(shareID, otherUserID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+shareID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 instead of 403 for info hiding
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestShareHandler_Update_ValidationErrors(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "empty name",
			body:      `{"name": ""}`,
			wantField: "name",
		},
		{
			name:      "negative max_downloads",
			body:      `{"max_downloads": -1}`,
			wantField: "max_downloads",
		},
		{
			name:      "negative max_views",
			body:      `{"max_views": -1}`,
			wantField: "max_views",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+shareID, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withUserContext(req, userID)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}

			var response struct {
				Fields map[string]string `json:"fields"`
			}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if _, ok := response.Fields[tt.wantField]; !ok {
				t.Errorf("expected field error for %s, got fields: %v", tt.wantField, response.Fields)
			}
		})
	}
}

func TestShareHandler_Update_Unauthenticated(t *testing.T) {
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/share-123", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No user context
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestShareHandler_Delete_Success(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
		deleteFn: func(ctx context.Context, id string) error {
			if id != shareID {
				t.Errorf("expected id %s, got %s", shareID, id)
			}
			return nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
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

func TestShareHandler_Delete_NotFound(t *testing.T) {
	userID := "user-123"
	shareID := "nonexistent"

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestShareHandler_Delete_NotOwner(t *testing.T) {
	userID := "user-123"
	otherUserID := "user-456"
	shareID := "share-123"
	share := newTestShare(shareID, otherUserID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 instead of 403 for info hiding
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestShareHandler_Delete_Unauthenticated(t *testing.T) {
	mockShare := &mockShareService{}
	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/share-123", nil)
	// No user context
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestShareHandler_Delete_ServiceError(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
		deleteFn: func(ctx context.Context, id string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestShareHandler_Get_NilCreatorID(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)
	share.CreatorID = nil // Set to nil

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 for shares with nil creator (info hiding)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestShareHandler_Get_InternalError(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestShareHandler_Create_InternalError(t *testing.T) {
	userID := "user-123"

	mockShare := &mockShareService{
		createFn: func(ctx context.Context, input service.CreateShareInput) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestShareHandler_Update_InvalidJSON(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+shareID, bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestShareHandler_Update_InternalError(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
		updateFn: func(ctx context.Context, id string, input service.UpdateShareInput) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	body := `{"name": "Test"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+shareID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestShareHandler_Delete_InternalErrorOnGet(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"

	mockShare := &mockShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewShareHandler(mockShare, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+shareID, nil)
	req = withUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
