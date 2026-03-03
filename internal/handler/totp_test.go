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
	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// mockTOTPService implements handler.TOTPServiceInterface for testing.
type mockTOTPService struct {
	beginSetupFn              func(ctx context.Context, userID string) (string, string, string, error)
	confirmSetupFn            func(ctx context.Context, userID, code string) ([]string, error)
	verifyFn                  func(ctx context.Context, userID, code string) error
	verifyRecoveryCodeFn      func(ctx context.Context, userID, code string) error
	disableFn                 func(ctx context.Context, userID string) error
	regenerateRecoveryCodesFn func(ctx context.Context, userID string) ([]string, error)
	getStatusFn               func(ctx context.Context, userID string) (bool, error)
	isOIDCUserFn              func(ctx context.Context, userID string) (bool, error)
	generatePendingTokenFn    func(userID string, isAdmin bool) (string, error)
	validatePendingTokenFn    func(tokenStr string) (*service.Claims, error)
}

func (m *mockTOTPService) BeginSetup(ctx context.Context, userID string) (string, string, string, error) {
	if m.beginSetupFn != nil {
		return m.beginSetupFn(ctx, userID)
	}
	return "", "", "", errors.New("not implemented")
}

func (m *mockTOTPService) ConfirmSetup(ctx context.Context, userID, code string) ([]string, error) {
	if m.confirmSetupFn != nil {
		return m.confirmSetupFn(ctx, userID, code)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTOTPService) Verify(ctx context.Context, userID, code string) error {
	if m.verifyFn != nil {
		return m.verifyFn(ctx, userID, code)
	}
	return errors.New("not implemented")
}

func (m *mockTOTPService) VerifyRecoveryCode(ctx context.Context, userID, code string) error {
	if m.verifyRecoveryCodeFn != nil {
		return m.verifyRecoveryCodeFn(ctx, userID, code)
	}
	return errors.New("not implemented")
}

func (m *mockTOTPService) Disable(ctx context.Context, userID string) error {
	if m.disableFn != nil {
		return m.disableFn(ctx, userID)
	}
	return errors.New("not implemented")
}

func (m *mockTOTPService) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	if m.regenerateRecoveryCodesFn != nil {
		return m.regenerateRecoveryCodesFn(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTOTPService) GetStatus(ctx context.Context, userID string) (bool, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(ctx, userID)
	}
	return false, errors.New("not implemented")
}

func (m *mockTOTPService) IsOIDCUser(ctx context.Context, userID string) (bool, error) {
	if m.isOIDCUserFn != nil {
		return m.isOIDCUserFn(ctx, userID)
	}
	return false, nil
}

func (m *mockTOTPService) GeneratePendingToken(userID string, isAdmin bool) (string, error) {
	if m.generatePendingTokenFn != nil {
		return m.generatePendingTokenFn(userID, isAdmin)
	}
	return "", errors.New("not implemented")
}

func (m *mockTOTPService) ValidatePendingToken(tokenStr string) (*service.Claims, error) {
	if m.validatePendingTokenFn != nil {
		return m.validatePendingTokenFn(tokenStr)
	}
	return nil, errors.New("not implemented")
}

// mockPasswordVerifier implements handler.PasswordVerifier for testing.
type mockPasswordVerifier struct {
	verifyPasswordFn func(ctx context.Context, userID, password string) error
}

func (m *mockPasswordVerifier) VerifyPassword(ctx context.Context, userID, password string) error {
	if m.verifyPasswordFn != nil {
		return m.verifyPasswordFn(ctx, userID, password)
	}
	return errors.New("not implemented")
}

// setupTOTPRouter creates a chi router with TOTP routes for testing.
// Authenticated routes under /me/2fa inject a test user ID into the context.
func setupTOTPRouter(h *handler.TOTPHandler) *chi.Mux {
	r := chi.NewRouter()
	// Inject test user ID for authenticated routes
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.UserIDKey, "test-user-123")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/me/2fa", func(r chi.Router) {
		r.Get("/status", h.GetStatus)
		r.Post("/setup", h.BeginSetup)
		r.Post("/confirm", h.ConfirmSetup)
		r.Post("/disable", h.Disable)
		r.Post("/recovery-codes", h.RegenerateRecoveryCodes)
	})
	r.Route("/auth/2fa", func(r chi.Router) {
		r.Post("/verify", h.Verify)
		r.Post("/recovery", h.Recovery)
	})
	return r
}

// --- Authenticated endpoints: /me/2fa ---

func TestTOTPHandler_GetStatus_Success(t *testing.T) {
	totpMock := &mockTOTPService{
		getStatusFn: func(ctx context.Context, userID string) (bool, error) {
			return false, nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/me/2fa/status", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Enabled    bool `json:"enabled"`
			Require2FA bool `json:"require_2fa"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Enabled {
		t.Error("expected enabled to be false")
	}
	if response.Data.Require2FA {
		t.Error("expected require_2fa to be false")
	}
}

func TestTOTPHandler_GetStatus_Enabled(t *testing.T) {
	totpMock := &mockTOTPService{
		getStatusFn: func(ctx context.Context, userID string) (bool, error) {
			return true, nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, true)
	router := setupTOTPRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/me/2fa/status", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Enabled    bool `json:"enabled"`
			Require2FA bool `json:"require_2fa"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if !response.Data.Enabled {
		t.Error("expected enabled to be true")
	}
	if !response.Data.Require2FA {
		t.Error("expected require_2fa to be true")
	}
}

func TestTOTPHandler_BeginSetup_Success(t *testing.T) {
	totpMock := &mockTOTPService{
		beginSetupFn: func(ctx context.Context, userID string) (string, string, string, error) {
			return "JBSWY3DPEHPK3PXP", "base64-qr-data", "otpauth://totp/Enlace:user@example.com?secret=JBSWY3DPEHPK3PXP", nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/me/2fa/setup", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Secret          string `json:"secret"`
			QRCode          string `json:"qr_code"`
			ProvisioningURI string `json:"provisioning_uri"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("expected secret JBSWY3DPEHPK3PXP, got %s", response.Data.Secret)
	}
	if response.Data.QRCode != "base64-qr-data" {
		t.Errorf("expected qr_code base64-qr-data, got %s", response.Data.QRCode)
	}
	if response.Data.ProvisioningURI != "otpauth://totp/Enlace:user@example.com?secret=JBSWY3DPEHPK3PXP" {
		t.Errorf("expected provisioning_uri otpauth://totp/Enlace:user@example.com?secret=JBSWY3DPEHPK3PXP, got %s", response.Data.ProvisioningURI)
	}
}

func TestTOTPHandler_BeginSetup_AlreadyEnabled(t *testing.T) {
	totpMock := &mockTOTPService{
		beginSetupFn: func(ctx context.Context, userID string) (string, string, string, error) {
			return "", "", "", service.ErrTOTPAlreadyEnabled
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/me/2fa/setup", nil)
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

func TestTOTPHandler_ConfirmSetup_Success(t *testing.T) {
	totpMock := &mockTOTPService{
		confirmSetupFn: func(ctx context.Context, userID, code string) ([]string, error) {
			return []string{"code1", "code2", "code3", "code4", "code5"}, nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{"code": "123456"}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/confirm", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			RecoveryCodes []string `json:"recovery_codes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if len(response.Data.RecoveryCodes) != 5 {
		t.Errorf("expected 5 recovery codes, got %d", len(response.Data.RecoveryCodes))
	}
}

func TestTOTPHandler_ConfirmSetup_MissingCode(t *testing.T) {
	totpMock := &mockTOTPService{}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/confirm", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["code"]; !ok {
		t.Errorf("expected field error for code, got fields: %v", response.Fields)
	}
}

func TestTOTPHandler_ConfirmSetup_InvalidCode(t *testing.T) {
	totpMock := &mockTOTPService{
		confirmSetupFn: func(ctx context.Context, userID, code string) ([]string, error) {
			return nil, service.ErrInvalidTOTPCode
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{"code": "000000"}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/confirm", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestTOTPHandler_Disable_Success(t *testing.T) {
	totpMock := &mockTOTPService{
		disableFn: func(ctx context.Context, userID string) error {
			return nil
		},
	}
	pwMock := &mockPasswordVerifier{
		verifyPasswordFn: func(ctx context.Context, userID, password string) error {
			return nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, pwMock, false)
	router := setupTOTPRouter(h)

	body := `{"password": "correct-password"}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/disable", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestTOTPHandler_Disable_MissingPassword(t *testing.T) {
	totpMock := &mockTOTPService{}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/disable", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["password"]; !ok {
		t.Errorf("expected field error for password, got fields: %v", response.Fields)
	}
}

func TestTOTPHandler_Disable_WrongPassword(t *testing.T) {
	totpMock := &mockTOTPService{}
	pwMock := &mockPasswordVerifier{
		verifyPasswordFn: func(ctx context.Context, userID, password string) error {
			return service.ErrInvalidCredentials
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, pwMock, false)
	router := setupTOTPRouter(h)

	body := `{"password": "wrong-password"}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/disable", bytes.NewBufferString(body))
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

func TestTOTPHandler_RegenerateRecoveryCodes_Success(t *testing.T) {
	totpMock := &mockTOTPService{
		regenerateRecoveryCodesFn: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"new1", "new2", "new3", "new4", "new5"}, nil
		},
	}
	pwMock := &mockPasswordVerifier{
		verifyPasswordFn: func(ctx context.Context, userID, password string) error {
			return nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, pwMock, false)
	router := setupTOTPRouter(h)

	body := `{"password": "correct-password"}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/recovery-codes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			RecoveryCodes []string `json:"recovery_codes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if len(response.Data.RecoveryCodes) != 5 {
		t.Errorf("expected 5 recovery codes, got %d", len(response.Data.RecoveryCodes))
	}
}

func TestTOTPHandler_RegenerateRecoveryCodes_MissingPassword(t *testing.T) {
	totpMock := &mockTOTPService{}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/recovery-codes", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["password"]; !ok {
		t.Errorf("expected field error for password, got fields: %v", response.Fields)
	}
}

func TestTOTPHandler_RegenerateRecoveryCodes_WrongPassword(t *testing.T) {
	totpMock := &mockTOTPService{}
	pwMock := &mockPasswordVerifier{
		verifyPasswordFn: func(ctx context.Context, userID, password string) error {
			return service.ErrInvalidCredentials
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, pwMock, false)
	router := setupTOTPRouter(h)

	body := `{"password": "wrong-password"}`
	req := httptest.NewRequest(http.MethodPost, "/me/2fa/recovery-codes", bytes.NewBufferString(body))
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

// --- Public endpoints: /auth/2fa ---

func TestTOTPHandler_Verify_Success(t *testing.T) {
	totpMock := &mockTOTPService{
		validatePendingTokenFn: func(tokenStr string) (*service.Claims, error) {
			return &service.Claims{UserID: "test-user-123", IsAdmin: false, TFA: true}, nil
		},
		verifyFn: func(ctx context.Context, userID, code string) error {
			return nil
		},
	}
	authTokenMock := &mockAuthTokenService{
		generateTokensFn: func(userID string, isAdmin bool) (*handler.TokenPair, error) {
			return &handler.TokenPair{AccessToken: "access-token", RefreshToken: "refresh-token"}, nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, authTokenMock, nil, false)
	router := setupTOTPRouter(h)

	body := `{"pending_token": "pending-token-123", "code": "123456"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/verify", bytes.NewBufferString(body))
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
	if response.Data.AccessToken != "access-token" {
		t.Errorf("expected access_token access-token, got %s", response.Data.AccessToken)
	}
	if response.Data.RefreshToken != "refresh-token" {
		t.Errorf("expected refresh_token refresh-token, got %s", response.Data.RefreshToken)
	}
}

func TestTOTPHandler_Verify_MissingFields(t *testing.T) {
	totpMock := &mockTOTPService{}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/verify", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["pending_token"]; !ok {
		t.Errorf("expected field error for pending_token, got fields: %v", response.Fields)
	}
	if _, ok := response.Fields["code"]; !ok {
		t.Errorf("expected field error for code, got fields: %v", response.Fields)
	}
}

func TestTOTPHandler_Verify_InvalidPendingToken(t *testing.T) {
	totpMock := &mockTOTPService{
		validatePendingTokenFn: func(tokenStr string) (*service.Claims, error) {
			return nil, errors.New("invalid token")
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{"pending_token": "bad-token", "code": "123456"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/verify", bytes.NewBufferString(body))
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

func TestTOTPHandler_Verify_InvalidCode(t *testing.T) {
	totpMock := &mockTOTPService{
		validatePendingTokenFn: func(tokenStr string) (*service.Claims, error) {
			return &service.Claims{UserID: "test-user-123", IsAdmin: false, TFA: true}, nil
		},
		verifyFn: func(ctx context.Context, userID, code string) error {
			return service.ErrInvalidTOTPCode
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{"pending_token": "pending-token-123", "code": "000000"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/verify", bytes.NewBufferString(body))
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

func TestTOTPHandler_Recovery_Success(t *testing.T) {
	totpMock := &mockTOTPService{
		validatePendingTokenFn: func(tokenStr string) (*service.Claims, error) {
			return &service.Claims{UserID: "test-user-123", IsAdmin: false, TFA: true}, nil
		},
		verifyRecoveryCodeFn: func(ctx context.Context, userID, code string) error {
			return nil
		},
	}
	authTokenMock := &mockAuthTokenService{
		generateTokensFn: func(userID string, isAdmin bool) (*handler.TokenPair, error) {
			return &handler.TokenPair{AccessToken: "access-token", RefreshToken: "refresh-token"}, nil
		},
	}

	h := handler.NewTOTPHandler(totpMock, authTokenMock, nil, false)
	router := setupTOTPRouter(h)

	body := `{"pending_token": "pending-token-123", "recovery_code": "recovery-code-1"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/recovery", bytes.NewBufferString(body))
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
	if response.Data.AccessToken != "access-token" {
		t.Errorf("expected access_token access-token, got %s", response.Data.AccessToken)
	}
	if response.Data.RefreshToken != "refresh-token" {
		t.Errorf("expected refresh_token refresh-token, got %s", response.Data.RefreshToken)
	}
}

func TestTOTPHandler_Recovery_MissingFields(t *testing.T) {
	totpMock := &mockTOTPService{}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/recovery", bytes.NewBufferString(body))
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
	if _, ok := response.Fields["pending_token"]; !ok {
		t.Errorf("expected field error for pending_token, got fields: %v", response.Fields)
	}
	if _, ok := response.Fields["recovery_code"]; !ok {
		t.Errorf("expected field error for recovery_code, got fields: %v", response.Fields)
	}
}

func TestTOTPHandler_Recovery_InvalidPendingToken(t *testing.T) {
	totpMock := &mockTOTPService{
		validatePendingTokenFn: func(tokenStr string) (*service.Claims, error) {
			return nil, errors.New("invalid token")
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{"pending_token": "bad-token", "recovery_code": "recovery-code-1"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/recovery", bytes.NewBufferString(body))
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

func TestTOTPHandler_Recovery_InvalidCode(t *testing.T) {
	totpMock := &mockTOTPService{
		validatePendingTokenFn: func(tokenStr string) (*service.Claims, error) {
			return &service.Claims{UserID: "test-user-123", IsAdmin: false, TFA: true}, nil
		},
		verifyRecoveryCodeFn: func(ctx context.Context, userID, code string) error {
			return service.ErrInvalidRecoveryCode
		},
	}

	h := handler.NewTOTPHandler(totpMock, nil, nil, false)
	router := setupTOTPRouter(h)

	body := `{"pending_token": "pending-token-123", "recovery_code": "bad-code"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/recovery", bytes.NewBufferString(body))
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
