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

var smtpTestJWTSecret = []byte("test-jwt-secret-for-smtp-tests")

// setupSMTPRouter creates a router with SMTP config routes for testing.
func setupSMTPRouter(h *handler.SMTPConfigHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/admin/smtp", func(r chi.Router) {
		r.Get("/", h.GetSMTPConfig)
		r.Put("/", h.UpdateSMTPConfig)
		r.Delete("/", h.DeleteSMTPConfig)
	})
	return r
}

// --- GetSMTPConfig Tests ---

func TestSMTPConfigHandler_GetSMTPConfig_Success(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"smtp_host":       "smtp.example.com",
				"smtp_port":       "465",
				"smtp_user":       "user@example.com",
				"smtp_pass":       "enc:someencryptedvalue",
				"smtp_from":       "noreply@example.com",
				"smtp_tls_policy": "mandatory",
			}, nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/smtp", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Host      string `json:"smtp_host"`
			Port      string `json:"smtp_port"`
			User      string `json:"smtp_user"`
			PassSet   bool   `json:"smtp_pass_set"`
			From      string `json:"smtp_from"`
			TLSPolicy string `json:"smtp_tls_policy"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Host != "smtp.example.com" {
		t.Errorf("expected smtp_host 'smtp.example.com', got '%s'", response.Data.Host)
	}
	if response.Data.Port != "465" {
		t.Errorf("expected smtp_port '465', got '%s'", response.Data.Port)
	}
	if !response.Data.PassSet {
		t.Error("expected smtp_pass_set to be true")
	}
	if response.Data.TLSPolicy != "mandatory" {
		t.Errorf("expected smtp_tls_policy 'mandatory', got '%s'", response.Data.TLSPolicy)
	}
}

func TestSMTPConfigHandler_GetSMTPConfig_PasswordNotReturned(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_pass": "enc:someencryptedvalue",
			}, nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/smtp", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.String()
	if bytes.Contains([]byte(body), []byte("someencryptedvalue")) {
		t.Error("response should not contain the encrypted password value")
	}
}

func TestSMTPConfigHandler_GetSMTPConfig_Empty(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/smtp", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Host    string `json:"smtp_host"`
			PassSet bool   `json:"smtp_pass_set"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Host != "" {
		t.Errorf("expected empty smtp_host, got '%s'", response.Data.Host)
	}
	if response.Data.PassSet {
		t.Error("expected smtp_pass_set to be false")
	}
}

func TestSMTPConfigHandler_GetSMTPConfig_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/smtp", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// --- UpdateSMTPConfig Tests ---

func TestSMTPConfigHandler_UpdateSMTPConfig_Success(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"smtp_host":       "smtp.example.com",
				"smtp_port":       "587",
				"smtp_from":       "noreply@example.com",
				"smtp_tls_policy": "opportunistic",
				"smtp_pass":       "enc:encrypted",
			}, nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{"smtp_host": "smtp.example.com", "smtp_port": "587", "smtp_from": "noreply@example.com", "smtp_tls_policy": "opportunistic"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
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
	if savedSettings["smtp_host"] != "smtp.example.com" {
		t.Errorf("expected smtp_host 'smtp.example.com', got '%s'", savedSettings["smtp_host"])
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_InvalidTLSPolicy(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{"smtp_host": "smtp.example.com", "smtp_tls_policy": "starttls"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["smtp_tls_policy"]; !ok {
		t.Error("expected field error for smtp_tls_policy")
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_InvalidPort(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{"smtp_host": "smtp.example.com", "smtp_port": "abc"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["smtp_port"]; !ok {
		t.Error("expected field error for smtp_port")
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_PortOutOfRange(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{"smtp_host": "smtp.example.com", "smtp_port": "99999"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["smtp_port"]; !ok {
		t.Error("expected field error for smtp_port")
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_EmptyBody(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_InvalidJSON(t *testing.T) {
	mock := &mockSettingsRepository{}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_PasswordEncrypted(t *testing.T) {
	var savedSettings map[string]string
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			savedSettings = settings
			return nil
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_from": "noreply@example.com",
				"smtp_pass": "enc:encrypted",
			}, nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{"smtp_host": "smtp.example.com", "smtp_from": "noreply@example.com", "smtp_pass": "mysecretpassword"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
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

	savedPass := savedSettings["smtp_pass"]
	if savedPass == "mysecretpassword" {
		t.Error("expected password to be encrypted, but got plaintext")
	}
	if !strings.HasPrefix(savedPass, "enc:") {
		t.Errorf("expected encrypted password to have 'enc:' prefix, got %q", savedPass)
	}

	respBody := w.Body.String()
	if bytes.Contains([]byte(respBody), []byte("mysecretpassword")) {
		t.Error("response should not contain the plaintext password value")
	}

	var response struct {
		Data struct {
			PassSet bool `json:"smtp_pass_set"`
		} `json:"data"`
	}
	if err := json.NewDecoder(bytes.NewBufferString(respBody)).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Data.PassSet {
		t.Error("expected smtp_pass_set to be true")
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_MissingFromWhenHostSet(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{"smtp_host": "smtp.example.com"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["smtp_from"]; !ok {
		t.Error("expected field error for smtp_from")
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			return errors.New("database error")
		},
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"smtp_from": "noreply@example.com",
			}, nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	body := `{"smtp_host": "smtp.example.com"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestSMTPConfigHandler_UpdateSMTPConfig_PartialUpdateWithExistingDB(t *testing.T) {
	mock := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_from": "noreply@example.com",
				"smtp_port": "587",
			}, nil
		},
		setMultipleFn: func(ctx context.Context, settings map[string]string) error {
			return nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	// Only updating the port; host and from already exist in DB
	body := `{"smtp_port": "465"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// --- DeleteSMTPConfig Tests ---

func TestSMTPConfigHandler_DeleteSMTPConfig_Success(t *testing.T) {
	var deletedKeys []string
	mock := &mockSettingsRepository{
		deleteMultipleFn: func(ctx context.Context, keys []string) error {
			deletedKeys = keys
			return nil
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/smtp", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if deletedKeys == nil {
		t.Fatal("expected keys to be deleted")
	}
	if len(deletedKeys) != 6 {
		t.Errorf("expected 6 keys to be deleted, got %d", len(deletedKeys))
	}
}

func TestSMTPConfigHandler_DeleteSMTPConfig_DatabaseError(t *testing.T) {
	mock := &mockSettingsRepository{
		deleteMultipleFn: func(ctx context.Context, keys []string) error {
			return errors.New("database error")
		},
	}

	h := handler.NewSMTPConfigHandler(mock, smtpTestJWTSecret)
	router := setupSMTPRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/smtp", nil)
	req = withAdminRequestContext(req)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
