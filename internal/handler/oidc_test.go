package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amalgamated-tools/sharer/internal/handler"
	"github.com/amalgamated-tools/sharer/internal/middleware"
)

func TestOIDCHandler_Config_Disabled(t *testing.T) {
	h := handler.NewOIDCHandler(nil, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/config", nil)
	rr := httptest.NewRecorder()

	h.Config(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"enabled":false`) {
		t.Errorf("expected enabled:false in response, got %s", body)
	}
}

func TestOIDCHandler_Config_Enabled(t *testing.T) {
	mockOIDC := &mockOIDCService{
		isEnabledFn: func() bool { return true },
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/config", nil)
	rr := httptest.NewRecorder()

	h.Config(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Enabled bool `json:"enabled"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if !response.Data.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestOIDCHandler_Login_Disabled(t *testing.T) {
	h := handler.NewOIDCHandler(nil, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestOIDCHandler_Login_Success(t *testing.T) {
	mockOIDC := &mockOIDCService{
		generateStateFn: func() (string, error) { return "test-state", nil },
		getAuthURLFn:    func(state, codeVerifier string) string { return "https://provider.example.com/auth?state=" + state },
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "https://provider.example.com/auth") {
		t.Errorf("expected redirect to provider, got %s", location)
	}

	// Check that state cookie was set
	cookies := rr.Result().Cookies()
	var foundStateCookie bool
	for _, c := range cookies {
		if c.Name == "oidc_state" {
			foundStateCookie = true
			if c.Value != "test-state" {
				t.Errorf("expected state cookie value test-state, got %s", c.Value)
			}
			if !c.HttpOnly {
				t.Error("expected HttpOnly to be true")
			}
		}
	}
	if !foundStateCookie {
		t.Error("expected oidc_state cookie to be set")
	}
}

func TestOIDCHandler_Callback_Disabled(t *testing.T) {
	h := handler.NewOIDCHandler(nil, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback", nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error, got %s", location)
	}
}

func TestOIDCHandler_Callback_MissingStateCookie(t *testing.T) {
	mockOIDC := &mockOIDCService{}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=test-state&code=auth-code", nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error, got %s", location)
	}
}

func TestOIDCHandler_Callback_StateMismatch(t *testing.T) {
	mockOIDC := &mockOIDCService{}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=wrong-state&code=auth-code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "correct-state"})
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error for state mismatch, got %s", location)
	}
}

func TestOIDCHandler_Link_Disabled(t *testing.T) {
	h := handler.NewOIDCHandler(nil, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/oidc/link", nil)
	rr := httptest.NewRecorder()

	h.Link(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestOIDCHandler_Link_Unauthorized(t *testing.T) {
	mockOIDC := &mockOIDCService{}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/oidc/link", nil)
	rr := httptest.NewRecorder()

	h.Link(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestOIDCHandler_Link_Success(t *testing.T) {
	mockOIDC := &mockOIDCService{
		generateStateFn: func() (string, error) { return "link-state", nil },
		getLinkAuthURLFn: func(state, codeVerifier string) string {
			return "https://provider.example.com/auth?state=" + state + "&prompt=consent"
		},
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/oidc/link", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Link(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	cookies := rr.Result().Cookies()
	var foundStateCookie, foundLinkCookie bool
	for _, c := range cookies {
		if c.Name == "oidc_state" {
			foundStateCookie = true
		}
		if c.Name == "oidc_link" && c.Value == "user-123" {
			foundLinkCookie = true
		}
	}
	if !foundStateCookie {
		t.Error("expected oidc_state cookie to be set")
	}
	if !foundLinkCookie {
		t.Error("expected oidc_link cookie to be set with user ID")
	}
}

func TestOIDCHandler_Unlink_Disabled(t *testing.T) {
	h := handler.NewOIDCHandler(nil, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me/oidc", nil)
	rr := httptest.NewRecorder()

	h.Unlink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestOIDCHandler_Unlink_Unauthorized(t *testing.T) {
	mockOIDC := &mockOIDCService{}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me/oidc", nil)
	rr := httptest.NewRecorder()

	h.Unlink(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestOIDCHandler_Unlink_Success(t *testing.T) {
	mockOIDC := &mockOIDCService{
		unlinkOIDCFn: func(ctx context.Context, userID string) error { return nil },
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me/oidc", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Unlink(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestOIDCHandler_Unlink_Error(t *testing.T) {
	mockOIDC := &mockOIDCService{
		unlinkOIDCFn: func(ctx context.Context, userID string) error {
			return errUnlinkFailed
		},
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me/oidc", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Unlink(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestOIDCHandler_Callback_Success(t *testing.T) {
	mockOIDC := &mockOIDCService{
		exchangeCodeFn: func(ctx context.Context, code, codeVerifier string) (*handler.OIDCUserInfo, error) {
			return &handler.OIDCUserInfo{
				Subject:     "sub-123",
				Email:       "user@example.com",
				DisplayName: "Test User",
				Issuer:      "https://issuer.example.com",
			}, nil
		},
		findOrCreateFn: func(ctx context.Context, info *handler.OIDCUserInfo) (*handler.OIDCUser, error) {
			return &handler.OIDCUser{
				ID:      "user-123",
				IsAdmin: false,
			}, nil
		},
	}
	mockAuth := &mockAuthTokenService{
		generateTokensFn: func(userID string, isAdmin bool) (*handler.TokenPair, error) {
			return &handler.TokenPair{
				AccessToken:  "access-token-abc",
				RefreshToken: "refresh-token-xyz",
			}, nil
		},
	}
	h := handler.NewOIDCHandler(mockOIDC, mockAuth, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=valid-state&code=auth-code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "valid-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_verifier", Value: "test-verifier"})
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "token=access-token-abc") {
		t.Errorf("expected redirect with access token, got %s", location)
	}
	if !strings.Contains(location, "refresh=refresh-token-xyz") {
		t.Errorf("expected redirect with refresh token, got %s", location)
	}
}

func TestOIDCHandler_Callback_MissingCode(t *testing.T) {
	mockOIDC := &mockOIDCService{}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "valid-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_verifier", Value: "test-verifier"})
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error for missing code, got %s", location)
	}
}

func TestOIDCHandler_Callback_ProviderError(t *testing.T) {
	mockOIDC := &mockOIDCService{}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=valid-state&error=access_denied&error_description=User+cancelled", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "valid-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_verifier", Value: "test-verifier"})
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error, got %s", location)
	}
}

func TestOIDCHandler_LinkCallback_Success(t *testing.T) {
	mockOIDC := &mockOIDCService{
		exchangeCodeFn: func(ctx context.Context, code, codeVerifier string) (*handler.OIDCUserInfo, error) {
			return &handler.OIDCUserInfo{
				Subject:     "sub-123",
				Email:       "user@example.com",
				DisplayName: "Test User",
				Issuer:      "https://issuer.example.com",
			}, nil
		},
		linkOIDCFn: func(ctx context.Context, userID string, info *handler.OIDCUserInfo) error {
			return nil
		},
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/oidc/callback?state=link-state&code=auth-code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "link-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_verifier", Value: "test-verifier"})
	req.AddCookie(&http.Cookie{Name: "oidc_link", Value: "user-123"})
	rr := httptest.NewRecorder()

	h.LinkCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "oidc=linked") {
		t.Errorf("expected redirect to settings with linked param, got %s", location)
	}
}

func TestOIDCHandler_LinkCallback_Disabled(t *testing.T) {
	h := handler.NewOIDCHandler(nil, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/oidc/callback", nil)
	rr := httptest.NewRecorder()

	h.LinkCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error, got %s", location)
	}
}

func TestOIDCHandler_LinkCallback_MissingLinkCookie(t *testing.T) {
	mockOIDC := &mockOIDCService{}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/oidc/callback?state=link-state&code=auth-code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "link-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_verifier", Value: "test-verifier"})
	rr := httptest.NewRecorder()

	h.LinkCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect with error for missing link cookie, got %s", location)
	}
}

func TestOIDCHandler_Login_StateGenerationError(t *testing.T) {
	mockOIDC := &mockOIDCService{
		generateStateFn: func() (string, error) {
			return "", errStateGenFailed
		},
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestOIDCHandler_Link_StateGenerationError(t *testing.T) {
	mockOIDC := &mockOIDCService{
		generateStateFn: func() (string, error) {
			return "", errStateGenFailed
		},
	}
	h := handler.NewOIDCHandler(mockOIDC, nil, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/oidc/link", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Link(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// Error variables for testing.
var (
	errUnlinkFailed   = errTest("cannot unlink OIDC from account without password")
	errStateGenFailed = errTest("state generation failed")
)

type errTest string

func (e errTest) Error() string { return string(e) }

// mockAuthTokenService implements AuthTokenServiceInterface for testing.
type mockAuthTokenService struct {
	generateTokensFn func(userID string, isAdmin bool) (*handler.TokenPair, error)
}

func (m *mockAuthTokenService) GenerateTokensForUser(userID string, isAdmin bool) (*handler.TokenPair, error) {
	if m.generateTokensFn != nil {
		return m.generateTokensFn(userID, isAdmin)
	}
	return nil, nil
}

// mockOIDCService implements OIDCServiceInterface for testing.
type mockOIDCService struct {
	isEnabledFn           func() bool
	generateStateFn       func() (string, error)
	generateCodeVerifierFn func() (string, error)
	getAuthURLFn          func(state, codeVerifier string) string
	getLinkAuthURLFn      func(state, codeVerifier string) string
	exchangeCodeFn        func(ctx context.Context, code, codeVerifier string) (*handler.OIDCUserInfo, error)
	findOrCreateFn        func(ctx context.Context, info *handler.OIDCUserInfo) (*handler.OIDCUser, error)
	linkOIDCFn            func(ctx context.Context, userID string, info *handler.OIDCUserInfo) error
	unlinkOIDCFn          func(ctx context.Context, userID string) error
}

func (m *mockOIDCService) IsEnabled() bool {
	if m.isEnabledFn != nil {
		return m.isEnabledFn()
	}
	return true
}

func (m *mockOIDCService) GenerateState() (string, error) {
	if m.generateStateFn != nil {
		return m.generateStateFn()
	}
	return "default-state", nil
}

func (m *mockOIDCService) GenerateCodeVerifier() (string, error) {
	if m.generateCodeVerifierFn != nil {
		return m.generateCodeVerifierFn()
	}
	return "default-verifier", nil
}

func (m *mockOIDCService) GetAuthURL(state, codeVerifier string) string {
	if m.getAuthURLFn != nil {
		return m.getAuthURLFn(state, codeVerifier)
	}
	return "https://default-auth-url.com?state=" + state
}

func (m *mockOIDCService) GetLinkAuthURL(state, codeVerifier string) string {
	if m.getLinkAuthURLFn != nil {
		return m.getLinkAuthURLFn(state, codeVerifier)
	}
	return "https://default-link-url.com?state=" + state
}

func (m *mockOIDCService) ExchangeCode(ctx context.Context, code, codeVerifier string) (*handler.OIDCUserInfo, error) {
	if m.exchangeCodeFn != nil {
		return m.exchangeCodeFn(ctx, code, codeVerifier)
	}
	return nil, nil
}

func (m *mockOIDCService) FindOrCreateUser(ctx context.Context, info *handler.OIDCUserInfo) (*handler.OIDCUser, error) {
	if m.findOrCreateFn != nil {
		return m.findOrCreateFn(ctx, info)
	}
	return nil, nil
}

func (m *mockOIDCService) LinkOIDC(ctx context.Context, userID string, info *handler.OIDCUserInfo) error {
	if m.linkOIDCFn != nil {
		return m.linkOIDCFn(ctx, userID, info)
	}
	return nil
}

func (m *mockOIDCService) UnlinkOIDC(ctx context.Context, userID string) error {
	if m.unlinkOIDCFn != nil {
		return m.unlinkOIDCFn(ctx, userID)
	}
	return nil
}
