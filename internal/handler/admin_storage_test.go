package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/enlace/internal/handler"
)

var storageTestJWTSecret = []byte("test-jwt-secret-for-storage-tests")

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
		r.Post("/test", h.TestStorageConnection)
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
				"s3_secret_key": "enc:someencryptedvalue",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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
				"s3_secret_key": "enc:someencryptedvalue",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check that the encrypted value is not in the response body
	body := w.Body.String()
	if bytes.Contains([]byte(body), []byte("someencryptedvalue")) {
		t.Error("response should not contain the encrypted secret key value")
	}
}

func TestStorageConfigHandler_GetStorageConfig_Empty(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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
			// Called twice: once for validation (before save) and once for response (after save)
			return map[string]string{
				"storage_type":  "s3",
				"s3_bucket":     "new-bucket",
				"s3_access_key": "AKIA123",
				"s3_secret_key": "enc:encrypted",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	body := `{"storage_type": "s3", "s3_bucket": "new-bucket", "s3_access_key": "AKIA123", "s3_secret_key": "mysecret"}`
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

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			// Validation read returns existing complete config
			return map[string]string{
				"storage_local_path": "/data/uploads",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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

func TestStorageConfigHandler_UpdateStorageConfig_SecretKeyEncrypted(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"storage_type":  "s3",
				"s3_bucket":     "bucket",
				"s3_access_key": "AKIA123",
				"s3_secret_key": "enc:encrypted",
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	body := `{"storage_type": "s3", "s3_bucket": "bucket", "s3_access_key": "AKIA123", "s3_secret_key": "mysecretkey"}`
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

	// The saved secret key should be encrypted (not plaintext)
	savedSecret := savedSettings["s3_secret_key"]
	if savedSecret == "mysecretkey" {
		t.Error("expected secret key to be encrypted, but got plaintext")
	}
	if !strings.HasPrefix(savedSecret, "enc:") {
		t.Errorf("expected encrypted secret to have 'enc:' prefix, got %q", savedSecret)
	}

	// The response should not contain the plaintext secret
	respBody := w.Body.String()
	if bytes.Contains([]byte(respBody), []byte("mysecretkey")) {
		t.Error("response should not contain the plaintext secret key value")
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

func TestStorageConfigHandler_UpdateStorageConfig_S3MissingRequiredFields(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			// No existing settings in DB
			return map[string]string{}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	// Setting storage_type=s3 without bucket, access_key, secret_key
	body := `{"storage_type": "s3"}`
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
	if _, ok := response.Fields["s3_bucket"]; !ok {
		t.Error("expected field error for s3_bucket")
	}
	if _, ok := response.Fields["s3_access_key"]; !ok {
		t.Error("expected field error for s3_access_key")
	}
	if _, ok := response.Fields["s3_secret_key"]; !ok {
		t.Error("expected field error for s3_secret_key")
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_S3ValidWithExistingDBSettings(t *testing.T) {
	// Existing DB settings already have the required S3 fields
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"storage_type":  "s3",
				"s3_bucket":     "existing-bucket",
				"s3_access_key": "AKIA_EXISTING",
				"s3_secret_key": "enc:existingencrypted",
			}, nil
		},
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			return nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	// Only updating the bucket; access_key and secret_key already exist in DB
	body := `{"s3_bucket": "updated-bucket"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_LocalMissingPath(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	body := `{"storage_type": "local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := response.Fields["storage_local_path"]; !ok {
		t.Error("expected field error for storage_local_path")
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

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
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

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/storage", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- TestStorageConnection Tests ---

func TestStorageConfigHandler_TestStorageConnection_MissingFields(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/storage/test", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["s3_bucket"]; !ok {
		t.Error("expected field error for s3_bucket")
	}
	if _, ok := response.Fields["s3_access_key"]; !ok {
		t.Error("expected field error for s3_access_key")
	}
	if _, ok := response.Fields["s3_secret_key"]; !ok {
		t.Error("expected field error for s3_secret_key")
	}
}

func TestStorageConfigHandler_TestStorageConnection_InvalidJSON(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/storage/test", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestStorageConfigHandler_TestStorageConnection_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	body := `{"s3_bucket": "test-bucket", "s3_access_key": "AKIA123", "s3_secret_key": "secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/storage/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestStorageConfigHandler_TestStorageConnection_MergesDBSettings(t *testing.T) {
	// DB has access_key and secret_key already; request only provides bucket
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"s3_access_key": "AKIA_FROM_DB",
				"s3_secret_key": "plaintext-secret", // unencrypted legacy value
			}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	// Only provide bucket — access_key and secret_key come from DB
	body := `{"s3_bucket": "test-bucket", "s3_endpoint": "http://localhost:19000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/storage/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// We expect 422 (connection failed to fake endpoint) rather than 400 (missing fields),
	// proving the merge worked.
	if w.Code == http.StatusBadRequest {
		t.Errorf("expected merge to fill missing fields, but got 400: %s", w.Body.String())
	}
}

func TestStorageConfigHandler_TestStorageConnection_InvalidEndpoint(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewStorageConfigHandler(mock, storageTestJWTSecret)
	router := setupStorageRouter(h)

	body := `{"s3_bucket": "test-bucket", "s3_access_key": "AKIA123", "s3_secret_key": "secret", "s3_endpoint": "http://localhost:19000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/storage/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should get 422 because connection to fake endpoint fails
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status %d, got %d: %s", http.StatusUnprocessableEntity, w.Code, w.Body.String())
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
	if !strings.Contains(response.Error, "S3 connection failed") {
		t.Errorf("expected error to mention S3 connection failure, got %q", response.Error)
	}
}
