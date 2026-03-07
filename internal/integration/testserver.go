//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/handler"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

const testJWTSecret = "integration-test-jwt-secret"

// TestServer wraps an httptest.Server backed by the full Enlace stack.
type TestServer struct {
	URL    string
	Client *http.Client
	server *httptest.Server
}

// NewTestServer creates a fully wired Enlace server using a temporary SQLite
// database and local file storage. The server is automatically cleaned up when
// the test finishes.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	tmpDir := t.TempDir()

	db, err := database.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	sqlDB := db.DB()

	userRepo := repository.NewUserRepository(sqlDB)
	shareRepo := repository.NewShareRepository(sqlDB)
	fileRepo := repository.NewFileRepository(sqlDB)
	totpRepo := repository.NewTOTPRepository(sqlDB)
	settingsRepo := repository.NewSettingsRepository(sqlDB)
	apiKeyRepo := repository.NewAPIKeyRepository(sqlDB)
	webhookRepo := repository.NewWebhookRepository(sqlDB)
	recipientRepo := repository.NewRecipientRepository(sqlDB)

	store, err := storage.NewLocalStorage(filepath.Join(tmpDir, "uploads"))
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	jwtSecret := []byte(testJWTSecret)
	authService := service.NewAuthService(userRepo, jwtSecret)
	shareService := service.NewShareService(shareRepo, fileRepo, store)
	fileService := service.NewFileService(fileRepo, shareRepo, store, nil, 0)
	emailService := service.NewEmailService(service.SMTPConfig{}, recipientRepo, "http://localhost")
	totpService := service.NewTOTPService(totpRepo, userRepo, jwtSecret)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo)
	webhookService := service.NewWebhookService(webhookRepo, jwtSecret, nil)

	router := handler.NewRouter(handler.RouterConfig{
		AuthService:    authService,
		ShareService:   shareService,
		FileService:    fileService,
		EmailService:   emailService,
		APIKeyService:  apiKeyService,
		WebhookService: webhookService,
		UserRepo:       userRepo,
		ShareRepo:      shareRepo,
		FileRepo:       fileRepo,
		Storage:        store,
		SettingsRepo:   settingsRepo,
		JWTSecret:      testJWTSecret,
		BaseURL:        "http://localhost",
		FrontendFS:     nil,
		TOTPService:    totpService,
		Require2FA:     false,
	})

	srv := httptest.NewServer(router)

	t.Cleanup(func() {
		srv.Close()
		_ = db.Close()
	})

	return &TestServer{
		URL:    srv.URL,
		Client: &http.Client{Timeout: 10 * time.Second},
		server: srv,
	}
}

// PostJSON sends a POST request with a JSON body to the given path.
func (ts *TestServer) PostJSON(t *testing.T, path string, body any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	resp, err := ts.Client.Post(ts.URL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}

	return resp
}

// GetWithToken sends a GET request with a Bearer token to the given path.
func (ts *TestServer) GetWithToken(t *testing.T, path, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ts.Client.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}

	return resp
}

// PostJSONWithToken sends a POST request with a JSON body and Bearer token.
func (ts *TestServer) PostJSONWithToken(t *testing.T, path string, body any, token string) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ts.Client.Do(req)
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}

	return resp
}

// PutJSONWithToken sends a PUT request with a JSON body and Bearer token.
func (ts *TestServer) PutJSONWithToken(t *testing.T, path string, body any, token string) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, ts.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ts.Client.Do(req)
	if err != nil {
		t.Fatalf("PUT %s failed: %v", path, err)
	}

	return resp
}
