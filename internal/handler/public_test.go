package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/amalgamated-tools/enlace/internal/handler"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// Test JWT secret.
var testJWTSecret = []byte("test-secret-key-for-testing-only")

// mockPublicShareService implements PublicShareServiceInterface for testing.
type mockPublicShareService struct {
	getBySlugFn              func(ctx context.Context, slug string) (*model.Share, error)
	getByIDFn                func(ctx context.Context, id string) (*model.Share, error)
	verifyPasswordFn         func(ctx context.Context, id string, password string) bool
	validateAccessFn         func(ctx context.Context, share *model.Share) error
	incrementViewCountFn     func(ctx context.Context, id string) error
	incrementDownloadCountFn func(ctx context.Context, id string) error
}

func (m *mockPublicShareService) GetBySlug(ctx context.Context, slug string) (*model.Share, error) {
	if m.getBySlugFn != nil {
		return m.getBySlugFn(ctx, slug)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPublicShareService) GetByID(ctx context.Context, id string) (*model.Share, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPublicShareService) VerifyPassword(ctx context.Context, id string, password string) bool {
	if m.verifyPasswordFn != nil {
		return m.verifyPasswordFn(ctx, id, password)
	}
	return false
}

func (m *mockPublicShareService) ValidateAccess(ctx context.Context, share *model.Share) error {
	if m.validateAccessFn != nil {
		return m.validateAccessFn(ctx, share)
	}
	return nil
}

func (m *mockPublicShareService) IncrementViewCount(ctx context.Context, id string) error {
	if m.incrementViewCountFn != nil {
		return m.incrementViewCountFn(ctx, id)
	}
	return nil
}

func (m *mockPublicShareService) IncrementDownloadCount(ctx context.Context, id string) error {
	if m.incrementDownloadCountFn != nil {
		return m.incrementDownloadCountFn(ctx, id)
	}
	return nil
}

// mockPublicFileService implements PublicFileServiceInterface for testing.
type mockPublicFileService struct {
	listByShareFn func(ctx context.Context, shareID string) ([]*model.File, error)
	getByIDFn     func(ctx context.Context, id string) (*model.File, error)
	getContentFn  func(ctx context.Context, id string) (io.ReadCloser, *model.File, error)
	uploadFn      func(ctx context.Context, input service.UploadInput) (*model.File, error)
}

func (m *mockPublicFileService) ListByShare(ctx context.Context, shareID string) ([]*model.File, error) {
	if m.listByShareFn != nil {
		return m.listByShareFn(ctx, shareID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPublicFileService) GetByID(ctx context.Context, id string) (*model.File, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPublicFileService) GetContent(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
	if m.getContentFn != nil {
		return m.getContentFn(ctx, id)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockPublicFileService) Upload(ctx context.Context, input service.UploadInput) (*model.File, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, input)
	}
	return nil, errors.New("not implemented")
}

// setupPublicRouter creates a router with public routes for testing.
func setupPublicRouter(h *handler.PublicHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/s/{slug}", func(r chi.Router) {
		r.Get("/", h.ViewShare)
		r.Post("/verify", h.VerifyPassword)
		r.Get("/files/{fileId}", h.DownloadFile)
		r.Get("/files/{fileId}/preview", h.PreviewFile)
		r.Post("/upload", h.UploadToReverseShare)
	})
	return r
}

// newPublicTestShare creates a test share for public tests.
func newPublicTestShare(id, slug string) *model.Share {
	creatorID := "creator-123"
	now := time.Now()
	return &model.Share{
		ID:             id,
		CreatorID:      &creatorID,
		Slug:           slug,
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

// newPublicTestFile creates a test file for public handler tests.
func newPublicTestFile(id, shareID, name string) *model.File {
	return &model.File{
		ID:         id,
		ShareID:    shareID,
		UploaderID: nil,
		Name:       name,
		Size:       1024,
		MimeType:   "text/plain",
		StorageKey: shareID + "/" + id + "/" + name,
		CreatedAt:  time.Now(),
	}
}

// generateTestShareToken creates a valid share access token for testing.
func generateTestShareToken(shareID string) string {
	now := time.Now()
	claims := &handler.ShareAccessClaims{
		ShareID: shareID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(testJWTSecret)
	return tokenStr
}

// generateExpiredShareToken creates an expired share access token for testing.
func generateExpiredShareToken(shareID string) string {
	now := time.Now()
	claims := &handler.ShareAccessClaims{
		ShareID: shareID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(testJWTSecret)
	return tokenStr
}

// TestPublicHandler_ViewShare_Success tests viewing a public share.
func TestPublicHandler_ViewShare_Success(t *testing.T) {
	shareID := "share-123"
	slug := "test-share"
	share := newPublicTestShare(shareID, slug)
	files := []*model.File{
		newPublicTestFile("file-1", shareID, "test.txt"),
		newPublicTestFile("file-2", shareID, "data.csv"),
	}

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			if s != slug {
				t.Errorf("expected slug %s, got %s", slug, s)
			}
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementViewCountFn: func(ctx context.Context, id string) error {
			if id != shareID {
				t.Errorf("expected share ID %s, got %s", shareID, id)
			}
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		listByShareFn: func(ctx context.Context, sID string) ([]*model.File, error) {
			if sID != shareID {
				t.Errorf("expected share ID %s, got %s", shareID, sID)
			}
			return files, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/"+slug, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Share struct {
				ID   string `json:"id"`
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"share"`
			Files []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Share.ID != shareID {
		t.Errorf("expected share ID %s, got %s", shareID, response.Data.Share.ID)
	}
	if len(response.Data.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(response.Data.Files))
	}
}

// TestPublicHandler_ViewShare_NotFound tests viewing a non-existent share.
func TestPublicHandler_ViewShare_NotFound(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestPublicHandler_ViewShare_Expired tests viewing an expired share.
func TestPublicHandler_ViewShare_Expired(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return service.ErrShareExpired
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d", http.StatusGone, w.Code)
	}
}

// TestPublicHandler_ViewShare_DownloadLimitReached tests viewing a share with download limit reached.
func TestPublicHandler_ViewShare_DownloadLimitReached(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return service.ErrDownloadLimit
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d", http.StatusGone, w.Code)
	}
}

// TestPublicHandler_ViewShare_ViewLimitReached tests viewing a share with view limit reached.
func TestPublicHandler_ViewShare_ViewLimitReached(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return service.ErrViewLimit
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d", http.StatusGone, w.Code)
	}
}

// TestPublicHandler_ViewShare_PasswordProtected_WithToken tests viewing password-protected share with valid token.
func TestPublicHandler_ViewShare_PasswordProtected_WithToken(t *testing.T) {
	shareID := "share-123"
	slug := "test-share"
	share := newPublicTestShare(shareID, slug)
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementViewCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		listByShareFn: func(ctx context.Context, sID string) ([]*model.File, error) {
			return []*model.File{}, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/"+slug, nil)
	req.Header.Set("X-Share-Token", generateTestShareToken(shareID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestPublicHandler_ViewShare_PasswordProtected_WithoutToken tests viewing password-protected share without token.
func TestPublicHandler_ViewShare_PasswordProtected_WithoutToken(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestPublicHandler_ViewShare_PasswordProtected_ExpiredToken tests viewing with expired token.
func TestPublicHandler_ViewShare_PasswordProtected_ExpiredToken(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	req.Header.Set("X-Share-Token", generateExpiredShareToken(shareID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestPublicHandler_ViewShare_PasswordProtected_WrongShareToken tests viewing with token for different share.
func TestPublicHandler_ViewShare_PasswordProtected_WrongShareToken(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	req.Header.Set("X-Share-Token", generateTestShareToken("different-share-id"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestPublicHandler_VerifyPassword_Success tests successful password verification.
func TestPublicHandler_VerifyPassword_Success(t *testing.T) {
	shareID := "share-123"
	slug := "test-share"
	share := newPublicTestShare(shareID, slug)
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		verifyPasswordFn: func(ctx context.Context, id string, password string) bool {
			return id == shareID && password == "correct-password"
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := `{"password": "correct-password"}`
	req := httptest.NewRequest(http.MethodPost, "/s/"+slug+"/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if response.Data.Token == "" {
		t.Error("expected token to be non-empty")
	}

	// An HttpOnly, path-scoped cookie must be set so browser downloads are authenticated
	// without passing the token as a query parameter.
	var foundShareTokenCookie bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "share_token" {
			foundShareTokenCookie = true
			if !c.HttpOnly {
				t.Error("expected share_token cookie to be HttpOnly")
			}
			if c.Value == "" {
				t.Error("expected share_token cookie to be non-empty")
			}
			if c.Path != "/s/"+slug {
				t.Errorf("expected share_token cookie path /s/%s, got %s", slug, c.Path)
			}
		}
	}
	if !foundShareTokenCookie {
		t.Error("expected share_token cookie to be set")
	}
}

// TestPublicHandler_DownloadFile_PasswordProtected_WithCookie tests that the share_token cookie authenticates downloads.
func TestPublicHandler_DownloadFile_PasswordProtected_WithCookie(t *testing.T) {
	shareID := "share-cookie-123"
	fileID := "file-cookie-456"
	slug := "cookie-share"
	share := newPublicTestShare(shareID, slug)
	passwordHash := "hash"
	share.PasswordHash = &passwordHash
	file := newPublicTestFile(fileID, shareID, "test.txt")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementDownloadCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
		getContentFn: func(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
			return io.NopCloser(strings.NewReader("content")), file, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	// Authenticate via the share_token cookie (as set by VerifyPassword).
	req := httptest.NewRequest(http.MethodGet, "/s/"+slug+"/files/"+fileID, nil)
	req.AddCookie(&http.Cookie{Name: "share_token", Value: generateTestShareToken(shareID)})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d with cookie token, got %d", http.StatusOK, w.Code)
	}
}

// TestPublicHandler_VerifyPassword_WrongPassword tests incorrect password.
func TestPublicHandler_VerifyPassword_WrongPassword(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		verifyPasswordFn: func(ctx context.Context, id string, password string) bool {
			return false
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := `{"password": "wrong-password"}`
	req := httptest.NewRequest(http.MethodPost, "/s/test-share/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestPublicHandler_VerifyPassword_NoPassword tests verifying password for share without password.
func TestPublicHandler_VerifyPassword_NoPassword(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	// No password set

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := `{"password": "any-password"}`
	req := httptest.NewRequest(http.MethodPost, "/s/test-share/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestPublicHandler_VerifyPassword_EmptyPassword tests empty password.
func TestPublicHandler_VerifyPassword_EmptyPassword(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := `{"password": ""}`
	req := httptest.NewRequest(http.MethodPost, "/s/test-share/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestPublicHandler_VerifyPassword_InvalidJSON tests invalid JSON body.
func TestPublicHandler_VerifyPassword_InvalidJSON(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/verify", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestPublicHandler_VerifyPassword_ShareNotFound tests verifying password for non-existent share.
func TestPublicHandler_VerifyPassword_ShareNotFound(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := `{"password": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/s/nonexistent/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestPublicHandler_DownloadFile_Success tests successful file download.
func TestPublicHandler_DownloadFile_Success(t *testing.T) {
	shareID := "share-123"
	fileID := "file-456"
	slug := "test-share"
	share := newPublicTestShare(shareID, slug)
	file := newPublicTestFile(fileID, shareID, "test.txt")
	fileContent := "Hello, World!"

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementDownloadCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			if id == fileID {
				return file, nil
			}
			return nil, service.ErrFileNotFound
		},
		getContentFn: func(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
			return io.NopCloser(strings.NewReader(fileContent)), file, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/"+slug+"/files/"+fileID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check headers
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("expected content-type text/plain, got %s", contentType)
	}

	disposition := w.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment") {
		t.Errorf("expected attachment disposition, got %s", disposition)
	}

	// Check body
	if w.Body.String() != fileContent {
		t.Errorf("expected body %s, got %s", fileContent, w.Body.String())
	}
}

// TestPublicHandler_PreviewFile_Success tests successful file preview.
func TestPublicHandler_PreviewFile_Success(t *testing.T) {
	shareID := "share-123"
	fileID := "file-456"
	slug := "test-share"
	share := newPublicTestShare(shareID, slug)
	file := newPublicTestFile(fileID, shareID, "test.txt")
	fileContent := "Hello, World!"

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementDownloadCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
		getContentFn: func(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
			return io.NopCloser(strings.NewReader(fileContent)), file, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/"+slug+"/files/"+fileID+"/preview", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	disposition := w.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "inline") {
		t.Errorf("expected inline disposition, got %s", disposition)
	}
}

// TestPublicHandler_DownloadFile_FileNotFound tests downloading non-existent file.
func TestPublicHandler_DownloadFile_FileNotFound(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return nil, service.ErrFileNotFound
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share/files/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestPublicHandler_DownloadFile_FileFromDifferentShare tests downloading file from different share.
func TestPublicHandler_DownloadFile_FileFromDifferentShare(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	file := newPublicTestFile("file-456", "different-share-id", "test.txt")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share/files/file-456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestPublicHandler_DownloadFile_PasswordProtected tests downloading from password-protected share.
func TestPublicHandler_DownloadFile_PasswordProtected(t *testing.T) {
	shareID := "share-123"
	fileID := "file-456"
	share := newPublicTestShare(shareID, "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash
	file := newPublicTestFile(fileID, shareID, "test.txt")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementDownloadCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
		getContentFn: func(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
			return io.NopCloser(strings.NewReader("content")), file, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	// Without token - should fail
	req := httptest.NewRequest(http.MethodGet, "/s/test-share/files/"+fileID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d without token, got %d", http.StatusUnauthorized, w.Code)
	}

	// With valid token in header - should succeed
	req = httptest.NewRequest(http.MethodGet, "/s/test-share/files/"+fileID, nil)
	req.Header.Set("X-Share-Token", generateTestShareToken(shareID))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d with token, got %d", http.StatusOK, w.Code)
	}

	// Query-param token must NOT be accepted (prevents leakage via URL history/referrer).
	req = httptest.NewRequest(http.MethodGet, "/s/test-share/files/"+fileID+"?token="+generateTestShareToken(shareID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d when token is in query param, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestPublicHandler_DownloadFile_ReferrerPolicyHeader tests that download responses include Referrer-Policy.
func TestPublicHandler_DownloadFile_ReferrerPolicyHeader(t *testing.T) {
	shareID := "share-123"
	fileID := "file-456"
	slug := "test-share"
	share := newPublicTestShare(shareID, slug)
	file := newPublicTestFile(fileID, shareID, "test.txt")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementDownloadCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
		getContentFn: func(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
			return io.NopCloser(strings.NewReader("content")), file, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/"+slug+"/files/"+fileID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if rp := w.Header().Get("Referrer-Policy"); rp != "no-referrer" {
		t.Errorf("expected Referrer-Policy: no-referrer, got %q", rp)
	}
}

// TestPublicHandler_UploadToReverseShare_Success tests successful upload to reverse share.
func TestPublicHandler_UploadToReverseShare_Success(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			if input.ShareID != shareID {
				t.Errorf("expected share ID %s, got %s", shareID, input.ShareID)
			}
			if input.UploaderID != "" {
				t.Errorf("expected empty uploader ID, got %s", input.UploaderID)
			}
			return newPublicTestFile("new-file-id", shareID, input.Filename), nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "upload.txt")
	_, _ = part.Write([]byte("file content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if len(response.Data) != 1 {
		t.Errorf("expected 1 file, got %d", len(response.Data))
	}
}

// TestPublicHandler_UploadToReverseShare_NotReverseShare tests upload to regular share.
func TestPublicHandler_UploadToReverseShare_NotReverseShare(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	share.IsReverseShare = false

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "upload.txt")
	_, _ = part.Write([]byte("file content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_NoFiles tests upload with no files.
func TestPublicHandler_UploadToReverseShare_NoFiles(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	// Create empty multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_FileTooLarge tests upload of oversized file.
func TestPublicHandler_UploadToReverseShare_FileTooLarge(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	// Set a very small max file size for testing
	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret, handler.WithPublicMaxFileSize(10))
	router := setupPublicRouter(h)

	// Create multipart form with large content
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "large.txt")
	_, _ = part.Write([]byte("this content is too large for the limit"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_ShareNotFound tests upload to non-existent share.
func TestPublicHandler_UploadToReverseShare_ShareNotFound(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "upload.txt")
	_, _ = part.Write([]byte("content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/nonexistent/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_ShareExpired tests upload to expired share.
func TestPublicHandler_UploadToReverseShare_ShareExpired(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return service.ErrShareExpired
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "upload.txt")
	_, _ = part.Write([]byte("content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d", http.StatusGone, w.Code)
	}
}

// TestPublicHandler_ViewShare_InternalError tests internal error handling.
func TestPublicHandler_ViewShare_InternalError(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_ViewShare_FileListError tests error when listing files fails.
func TestPublicHandler_ViewShare_FileListError(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementViewCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		listByShareFn: func(ctx context.Context, shareID string) ([]*model.File, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_DownloadFile_GetContentError tests error when getting file content fails.
func TestPublicHandler_DownloadFile_GetContentError(t *testing.T) {
	shareID := "share-123"
	fileID := "file-456"
	share := newPublicTestShare(shareID, "test-share")
	file := newPublicTestFile(fileID, shareID, "test.txt")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementDownloadCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
		getContentFn: func(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
			return nil, nil, errors.New("storage error")
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share/files/"+fileID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_UploadError tests error during file upload.
func TestPublicHandler_UploadToReverseShare_UploadError(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			return nil, errors.New("storage error")
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "upload.txt")
	_, _ = part.Write([]byte("file content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_ViewShare_WithExpiresAt tests share response includes expires_at.
func TestPublicHandler_ViewShare_WithExpiresAt(t *testing.T) {
	shareID := "share-123"
	slug := "test-share"
	share := newPublicTestShare(shareID, slug)
	expiresAt := time.Now().Add(24 * time.Hour)
	share.ExpiresAt = &expiresAt
	maxDownloads := 10
	share.MaxDownloads = &maxDownloads
	maxViews := 100
	share.MaxViews = &maxViews

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, s string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
		incrementViewCountFn: func(ctx context.Context, id string) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		listByShareFn: func(ctx context.Context, sID string) ([]*model.File, error) {
			return []*model.File{}, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/"+slug, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Share struct {
				ExpiresAt    *string `json:"expires_at"`
				MaxDownloads *int    `json:"max_downloads"`
				MaxViews     *int    `json:"max_views"`
			} `json:"share"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.Share.ExpiresAt == nil {
		t.Error("expected expires_at to be present")
	}
	if response.Data.Share.MaxDownloads == nil || *response.Data.Share.MaxDownloads != 10 {
		t.Error("expected max_downloads to be 10")
	}
	if response.Data.Share.MaxViews == nil || *response.Data.Share.MaxViews != 100 {
		t.Error("expected max_views to be 100")
	}
}

// TestPublicHandler_VerifyPassword_InternalError tests internal error during password verification.
func TestPublicHandler_VerifyPassword_InternalError(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := `{"password": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/s/test-share/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_VerifyPassword_ShareExpired tests password verification for expired share.
func TestPublicHandler_VerifyPassword_ShareExpired(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return service.ErrShareExpired
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := `{"password": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/s/test-share/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d", http.StatusGone, w.Code)
	}
}

// TestPublicHandler_DownloadFile_ShareNotFound tests downloading from non-existent share.
func TestPublicHandler_DownloadFile_ShareNotFound(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/nonexistent/files/file-123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestPublicHandler_DownloadFile_ShareInternalError tests internal error when getting share.
func TestPublicHandler_DownloadFile_ShareInternalError(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share/files/file-123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_DownloadFile_FileInternalError tests internal error when getting file.
func TestPublicHandler_DownloadFile_FileInternalError(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share/files/file-123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_InternalError tests internal error when getting share.
func TestPublicHandler_UploadToReverseShare_InternalError(t *testing.T) {
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "upload.txt")
	_, _ = part.Write([]byte("content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_InvalidMultipart tests invalid multipart form.
func TestPublicHandler_UploadToReverseShare_InvalidMultipart(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	// Send invalid multipart form
	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", bytes.NewBufferString("not a multipart form"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestPublicHandler_UploadToReverseShare_MultipleFiles tests uploading multiple files.
func TestPublicHandler_UploadToReverseShare_MultipleFiles(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	share.IsReverseShare = true

	uploadCount := 0
	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			uploadCount++
			return newPublicTestFile("file-"+string(rune('0'+uploadCount)), shareID, input.Filename), nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret)
	router := setupPublicRouter(h)

	// Create multipart form with multiple files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for i := 0; i < 3; i++ {
		part, _ := writer.CreateFormFile("files", "file"+string(rune('1'+i))+".txt")
		_, _ = part.Write([]byte("content " + string(rune('1'+i))))
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 3 {
		t.Errorf("expected 3 files, got %d", len(response.Data))
	}
}

// TestPublicHandler_DownloadFile_ShareExpired tests downloading from expired share.
func TestPublicHandler_DownloadFile_ShareExpired(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return service.ErrShareExpired
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share/files/file-123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d", http.StatusGone, w.Code)
	}
}

// TestPublicHandler_ViewShare_InvalidToken tests viewing with invalid token format.
func TestPublicHandler_ViewShare_InvalidToken(t *testing.T) {
	share := newPublicTestShare("share-123", "test-share")
	passwordHash := "hash"
	share.PasswordHash = &passwordHash

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret)
	router := setupPublicRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/s/test-share", nil)
	req.Header.Set("X-Share-Token", "invalid-token-format")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestPublicHandler_UploadToReverseShare_BlockedExtension(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockSettings := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"blocked_extensions": ".exe,.bat,.sh",
			}, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret, handler.WithPublicSettingsRepo(mockSettings))
	router := setupPublicRouter(h)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "script.sh")
	_, _ = part.Write([]byte("#!/bin/bash"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "file extension is not allowed" {
		t.Errorf("expected 'file extension is not allowed', got %q", response.Error)
	}
}

func TestPublicHandler_UploadToReverseShare_AdminMaxFileSizeOverride(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockSettings := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"max_file_size": "10",
			}, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, nil, testJWTSecret, handler.WithPublicSettingsRepo(mockSettings))
	router := setupPublicRouter(h)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "large.txt")
	_, _ = part.Write([]byte("this content is more than 10 bytes"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "file exceeds maximum size limit" {
		t.Errorf("expected 'file exceeds maximum size limit', got %q", response.Error)
	}
}

func TestPublicHandler_UploadToReverseShare_SettingsRepoError_FallsBackToDefault(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			return newPublicTestFile("new-file-id", shareID, input.Filename), nil
		},
	}

	mockSettings := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret, handler.WithPublicSettingsRepo(mockSettings))
	router := setupPublicRouter(h)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "small.txt")
	_, _ = part.Write([]byte("ok"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should still succeed using default max size
	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
}

func TestPublicHandler_UploadToReverseShare_AllowedExtensionPasses(t *testing.T) {
	shareID := "share-123"
	share := newPublicTestShare(shareID, "test-share")
	share.IsReverseShare = true

	mockShare := &mockPublicShareService{
		getBySlugFn: func(ctx context.Context, slug string) (*model.Share, error) {
			return share, nil
		},
		validateAccessFn: func(ctx context.Context, s *model.Share) error {
			return nil
		},
	}

	mockFile := &mockPublicFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			return newPublicTestFile("new-file-id", shareID, input.Filename), nil
		},
	}

	mockSettings := &mockSettingsRepository{
		getMultipleFn: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"blocked_extensions": ".exe,.bat",
			}, nil
		},
	}

	h := handler.NewPublicHandler(mockShare, mockFile, testJWTSecret, handler.WithPublicSettingsRepo(mockSettings))
	router := setupPublicRouter(h)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("files", "document.pdf")
	_, _ = part.Write([]byte("pdf content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/s/test-share/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
}
