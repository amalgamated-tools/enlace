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

// setupFileRestrictionsRouter creates a router with file restriction config routes for testing.
func setupFileRestrictionsRouter(h *handler.FileRestrictionsHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/admin/files", func(r chi.Router) {
		r.Get("/", h.GetFileRestrictions)
		r.Put("/", h.UpdateFileRestrictions)
		r.Delete("/", h.DeleteFileRestrictions)
	})
	return r
}

// --- GetFileRestrictions Tests ---

func TestFileRestrictionsHandler_GetFileRestrictions_Success(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"max_file_size":      "52428800",
				"blocked_extensions": ".exe,.bat,.sh",
			}, nil
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/files", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			MaxFileSize       *int64   `json:"max_file_size"`
			BlockedExtensions []string `json:"blocked_extensions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.MaxFileSize == nil || *response.Data.MaxFileSize != 52428800 {
		t.Errorf("expected max_file_size 52428800, got %v", response.Data.MaxFileSize)
	}
	if len(response.Data.BlockedExtensions) != 3 {
		t.Errorf("expected 3 blocked extensions, got %d", len(response.Data.BlockedExtensions))
	}
	if response.Data.BlockedExtensions[0] != ".exe" {
		t.Errorf("expected first extension '.exe', got '%s'", response.Data.BlockedExtensions[0])
	}
}

func TestFileRestrictionsHandler_GetFileRestrictions_Empty(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/files", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			MaxFileSize       *int64   `json:"max_file_size"`
			BlockedExtensions []string `json:"blocked_extensions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.MaxFileSize != nil {
		t.Errorf("expected nil max_file_size, got %v", *response.Data.MaxFileSize)
	}
	if len(response.Data.BlockedExtensions) != 0 {
		t.Errorf("expected empty blocked_extensions, got %v", response.Data.BlockedExtensions)
	}
}

func TestFileRestrictionsHandler_GetFileRestrictions_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/files", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- UpdateFileRestrictions Tests ---

func TestFileRestrictionsHandler_UpdateFileRestrictions_MaxFileSizeOnly(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"max_file_size": "52428800",
			}, nil
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	body := `{"max_file_size": 52428800}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
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
	if savedSettings["max_file_size"] != "52428800" {
		t.Errorf("expected max_file_size '52428800', got '%s'", savedSettings["max_file_size"])
	}
	if _, ok := savedSettings["blocked_extensions"]; ok {
		t.Error("expected blocked_extensions to not be saved")
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_BlockedExtensionsOnly(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"blocked_extensions": ".exe,.bat,.sh",
			}, nil
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	body := `{"blocked_extensions": ".exe,.bat,.sh"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
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
	if savedSettings["blocked_extensions"] != ".exe,.bat,.sh" {
		t.Errorf("expected blocked_extensions '.exe,.bat,.sh', got '%s'", savedSettings["blocked_extensions"])
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_Both(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"max_file_size":      "10485760",
				"blocked_extensions": ".exe,.bat",
			}, nil
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	body := `{"max_file_size": 10485760, "blocked_extensions": ".exe,.bat"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
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
	if savedSettings["max_file_size"] != "10485760" {
		t.Errorf("expected max_file_size '10485760', got '%s'", savedSettings["max_file_size"])
	}
	if savedSettings["blocked_extensions"] != ".exe,.bat" {
		t.Errorf("expected blocked_extensions '.exe,.bat', got '%s'", savedSettings["blocked_extensions"])
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_InvalidMaxFileSize(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	body := `{"max_file_size": -1}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["max_file_size"]; !ok {
		t.Error("expected field error for max_file_size")
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_ZeroMaxFileSize(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	body := `{"max_file_size": 0}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_EmptyBody(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_InvalidJSON(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	body := `{"max_file_size": 1024}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestFileRestrictionsHandler_UpdateFileRestrictions_ExtensionsNormalized(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			if savedSettings != nil {
				return savedSettings, nil
			}
			return map[string]string{}, nil
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	// Send mixed case with spaces and missing dots
	body := `{"blocked_extensions": " .EXE , Bat , .SH "}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/files", bytes.NewBufferString(body))
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
	expected := ".exe,.bat,.sh"
	if savedSettings["blocked_extensions"] != expected {
		t.Errorf("expected normalized extensions '%s', got '%s'", expected, savedSettings["blocked_extensions"])
	}
}

// --- DeleteFileRestrictions Tests ---

func TestFileRestrictionsHandler_DeleteFileRestrictions_Success(t *testing.T) {
	var deletedKeys []string
	mock := &mockSettingsRepository{
		deleteMultipleFn: func(ctx context.Context, keys []string) error {
			deletedKeys = keys
			return nil
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/files", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if deletedKeys == nil {
		t.Fatal("expected keys to be deleted")
	}
	if len(deletedKeys) != 2 {
		t.Errorf("expected 2 keys to be deleted, got %d", len(deletedKeys))
	}
}

func TestFileRestrictionsHandler_DeleteFileRestrictions_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		deleteMultipleFn: func(ctx context.Context, keys []string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewFileRestrictionsHandler(mock)
	router := setupFileRestrictionsRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/files", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- Helper function tests ---

func TestIsExtensionBlocked(t *testing.T) {
	blocked := []string{".exe", ".bat", ".sh"}

	tests := []struct {
		filename string
		expected bool
	}{
		{"malware.exe", true},
		{"script.bat", true},
		{"run.sh", true},
		{"MALWARE.EXE", true},
		{"document.pdf", false},
		{"image.png", false},
		{"noextension", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := handler.IsExtensionBlocked(tt.filename, blocked)
			if result != tt.expected {
				t.Errorf("IsExtensionBlocked(%q, %v) = %v, want %v", tt.filename, blocked, result, tt.expected)
			}
		})
	}
}

func TestIsExtensionBlocked_EmptyBlockedList(t *testing.T) {
	if handler.IsExtensionBlocked("test.exe", nil) {
		t.Error("expected false for nil blocked list")
	}
	if handler.IsExtensionBlocked("test.exe", []string{}) {
		t.Error("expected false for empty blocked list")
	}
}
