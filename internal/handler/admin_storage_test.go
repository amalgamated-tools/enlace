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
)

// mockSettingsRepository implements handler.SettingsRepositoryInterface for testing.
type mockSettingsRepository struct {
	getFn            func(ctx context.Context, key string) (string, error)
	setFn            func(ctx context.Context, key, value string) error
	getMultipleFn    func(ctx context.Context, keys []string) (map[string]string, error)
	setMultipleFn    func(ctx context.Context, settings map[string]string) error
	deleteMultipleFn func(ctx context.Context, keys []string) error
}

func (m *mockSettingsRepository) Get(ctx context.Context, key string) (string, error) {
	if m.getFn != nil {
		return m.getFn(ctx, key)
	}
	return "", errors.New("not implemented")
}

func (m *mockSettingsRepository) Set(ctx context.Context, key, value string) error {
	if m.setFn != nil {
		return m.setFn(ctx, key, value)
	}
	return errors.New("not implemented")
}

func (m *mockSettingsRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if m.getMultipleFn != nil {
		return m.getMultipleFn(ctx, keys)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSettingsRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	if m.setMultipleFn != nil {
		return m.setMultipleFn(ctx, settings)
	}
	return errors.New("not implemented")
}

func (m *mockSettingsRepository) DeleteMultiple(ctx context.Context, keys []string) error {
	if m.deleteMultipleFn != nil {
		return m.deleteMultipleFn(ctx, keys)
	}
	return errors.New("not implemented")
}

// setupStorageRouter creates a router with storage config routes for testing.
func setupStorageRouter(h *handler.StorageConfigHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/admin/storage", func(r chi.Router) {
		r.Get("/", h.GetStorageConfig)
		r.Put("/", h.UpdateStorageConfig)
		r.Delete("/", h.DeleteStorageConfig)
	})
	return r
}

// --- GetStorageConfig Tests ---

func TestStorageConfigHandler_GetStorageConfig_Success(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"storage_type":  "s3",
				"s3_bucket":     "my-bucket",
				"s3_region":     "us-east-1",
				"s3_secret_key": "supersecret",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			StorageType    string `json:"storage_type"`
			S3Bucket       string `json:"s3_bucket"`
			S3Region       string `json:"s3_region"`
			S3SecretKeySet bool   `json:"s3_secret_key_set"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.StorageType != "s3" {
		t.Errorf("expected storage_type 's3', got '%s'", response.Data.StorageType)
	}
	if response.Data.S3Bucket != "my-bucket" {
		t.Errorf("expected s3_bucket 'my-bucket', got '%s'", response.Data.S3Bucket)
	}
	if !response.Data.S3SecretKeySet {
		t.Error("expected s3_secret_key_set to be true")
	}
}

func TestStorageConfigHandler_GetStorageConfig_SecretKeyNotReturned(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"storage_type":  "s3",
				"s3_secret_key": "supersecret",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check that the actual secret key value is not in the response body
	body := w.Body.String()
	if bytes.Contains([]byte(body), []byte("supersecret")) {
		t.Error("response should not contain the actual secret key value")
	}
}

func TestStorageConfigHandler_GetStorageConfig_Empty(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			StorageType    string `json:"storage_type"`
			S3SecretKeySet bool   `json:"s3_secret_key_set"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.StorageType != "" {
		t.Errorf("expected empty storage_type, got '%s'", response.Data.StorageType)
	}
	if response.Data.S3SecretKeySet {
		t.Error("expected s3_secret_key_set to be false")
	}
}

func TestStorageConfigHandler_GetStorageConfig_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- UpdateStorageConfig Tests ---

func TestStorageConfigHandler_UpdateStorageConfig_Success(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"storage_type": "s3",
				"s3_bucket":    "new-bucket",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	body := `{"storage_type": "s3", "s3_bucket": "new-bucket"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	if savedSettings == nil {
		t.Fatal("expected settings to be saved")
	}
	if savedSettings["storage_type"] != "s3" {
		t.Errorf("expected storage_type 's3', got '%s'", savedSettings["storage_type"])
	}
	if savedSettings["s3_bucket"] != "new-bucket" {
		t.Errorf("expected s3_bucket 'new-bucket', got '%s'", savedSettings["s3_bucket"])
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_InvalidStorageType(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	body := `{"storage_type": "azure"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response struct {
		Success bool              `json:"success"`
		Fields  map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success to be false")
	}
	if _, ok := response.Fields["storage_type"]; !ok {
		t.Error("expected field error for storage_type")
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_EmptyBody(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_InvalidJSON(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	body := `{"storage_type": "local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_SecretKeyNotReturned(t *testing.T) {
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"storage_type":  "s3",
				"s3_secret_key": "mysecret",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	body := `{"storage_type": "s3", "s3_secret_key": "mysecret"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	respBody := w.Body.String()
	if bytes.Contains([]byte(respBody), []byte("mysecret")) {
		t.Error("response should not contain the actual secret key value")
	}

	var response struct {
		Data struct {
			S3SecretKeySet bool `json:"s3_secret_key_set"`
		} `json:"data"`
	}
	if err := json.NewDecoder(bytes.NewBufferString(respBody)).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Data.S3SecretKeySet {
		t.Error("expected s3_secret_key_set to be true")
	}
}

// --- DeleteStorageConfig Tests ---

func TestStorageConfigHandler_DeleteStorageConfig_Success(t *testing.T) {
	var deletedKeys []string
	mock := &mockSettingsRepository{
		deleteMultipleFn: func(ctx context.Context, keys []string) error {
			deletedKeys = keys
			return nil
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if deletedKeys == nil {
		t.Fatal("expected keys to be deleted")
	}
	if len(deletedKeys) != 8 {
		t.Errorf("expected 8 keys to be deleted, got %d", len(deletedKeys))
	}
}

func TestStorageConfigHandler_DeleteStorageConfig_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		deleteMultipleFn: func(ctx context.Context, keys []string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewStorageConfigHandler(mock)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
