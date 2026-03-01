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
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/sharer/internal/handler"
	"github.com/amalgamated-tools/sharer/internal/middleware"
	"github.com/amalgamated-tools/sharer/internal/model"
	"github.com/amalgamated-tools/sharer/internal/service"
)

// mockFileHandlerFileService implements FileHandlerFileService for testing.
type mockFileHandlerFileService struct {
	uploadFn      func(ctx context.Context, input service.UploadInput) (*model.File, error)
	getByIDFn     func(ctx context.Context, id string) (*model.File, error)
	deleteFn      func(ctx context.Context, id string) error
	listByShareFn func(ctx context.Context, shareID string) ([]*model.File, error)
}

func (m *mockFileHandlerFileService) Upload(ctx context.Context, input service.UploadInput) (*model.File, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileHandlerFileService) GetByID(ctx context.Context, id string) (*model.File, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileHandlerFileService) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errors.New("not implemented")
}

func (m *mockFileHandlerFileService) ListByShare(ctx context.Context, shareID string) ([]*model.File, error) {
	if m.listByShareFn != nil {
		return m.listByShareFn(ctx, shareID)
	}
	return nil, errors.New("not implemented")
}

// mockFileHandlerShareService implements FileHandlerShareService for testing.
type mockFileHandlerShareService struct {
	getByIDFn func(ctx context.Context, id string) (*model.Share, error)
}

func (m *mockFileHandlerShareService) GetByID(ctx context.Context, id string) (*model.Share, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

// withFileUserContext adds a user ID to the request context.
func withFileUserContext(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

// setupFileRouter creates a router with file routes for testing.
func setupFileRouter(h *handler.FileHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/shares/{id}/files", h.ListByShare)
	r.Post("/api/v1/shares/{id}/files", h.Upload)
	r.Delete("/api/v1/files/{id}", h.Delete)
	return r
}

// newTestFile creates a test file with default values.
func newTestFile(id string, shareID string) *model.File {
	return &model.File{
		ID:         id,
		ShareID:    shareID,
		UploaderID: nil,
		Name:       "test-file.txt",
		Size:       1024,
		MimeType:   "text/plain",
		StorageKey: shareID + "/" + id + "/test-file.txt",
		CreatedAt:  time.Now(),
	}
}

// createMultipartRequest creates a multipart/form-data request with files.
func createMultipartRequest(t *testing.T, url string, files map[string][]byte) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for filename, content := range files {
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("failed to write file content: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestFileHandler_Upload_Success(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			if id != shareID {
				t.Errorf("expected shareID %s, got %s", shareID, id)
			}
			return share, nil
		},
	}

	uploadCalls := 0
	mockFileSvc := &mockFileHandlerFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			uploadCalls++
			if input.ShareID != shareID {
				t.Errorf("expected shareID %s, got %s", shareID, input.ShareID)
			}
			if input.UploaderID != userID {
				t.Errorf("expected uploaderID %s, got %s", userID, input.UploaderID)
			}

			// Read content to verify it's passed correctly
			content, err := io.ReadAll(input.Content)
			if err != nil {
				t.Fatalf("failed to read content: %v", err)
			}
			if len(content) == 0 {
				t.Error("expected non-empty content")
			}

			return &model.File{
				ID:       "file-" + input.Filename,
				ShareID:  input.ShareID,
				Name:     input.Filename,
				Size:     input.Size,
				MimeType: "application/octet-stream",
			}, nil
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"test1.txt": []byte("hello world"),
		"test2.pdf": []byte("pdf content here"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d, body: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	if uploadCalls != 2 {
		t.Errorf("expected 2 upload calls, got %d", uploadCalls)
	}

	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			MimeType string `json:"mime_type"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success to be true")
	}
	if len(response.Data) != 2 {
		t.Errorf("expected 2 files, got %d", len(response.Data))
	}
}

func TestFileHandler_Upload_SingleFile(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			return &model.File{
				ID:       "file-123",
				ShareID:  input.ShareID,
				Name:     input.Filename,
				Size:     input.Size,
				MimeType: "application/pdf",
			}, nil
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"document.pdf": []byte("pdf content"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
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

	if len(response.Data) != 1 {
		t.Errorf("expected 1 file, got %d", len(response.Data))
	}
	if response.Data[0].Name != "document.pdf" {
		t.Errorf("expected name 'document.pdf', got %s", response.Data[0].Name)
	}
}

func TestFileHandler_Upload_Unauthenticated(t *testing.T) {
	mockFileSvc := &mockFileHandlerFileService{}
	mockShareSvc := &mockFileHandlerShareService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"test.txt": []byte("content"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/share-123/files", files)
	// No user context
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestFileHandler_Upload_ShareNotFound(t *testing.T) {
	userID := "user-123"
	shareID := "nonexistent"

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	mockFileSvc := &mockFileHandlerFileService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"test.txt": []byte("content"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestFileHandler_Upload_NotOwner(t *testing.T) {
	userID := "user-123"
	otherUserID := "user-456"
	shareID := "share-123"
	share := newTestShare(shareID, otherUserID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"test.txt": []byte("content"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 for info hiding
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestFileHandler_Upload_NoFiles(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	// Create empty multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares/"+shareID+"/files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withFileUserContext(req, userID)
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
	if response.Error != "no files provided" {
		t.Errorf("expected error 'no files provided', got %s", response.Error)
	}
}

func TestFileHandler_Upload_FileTooLarge(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{}

	// Set max file size to 10 bytes
	h := handler.NewFileHandler(mockFileSvc, mockShareSvc, handler.WithMaxFileSize(10))
	router := setupFileRouter(h)

	// Create a file larger than 10 bytes
	files := map[string][]byte{
		"large.txt": []byte("this content is definitely more than 10 bytes"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != "file exceeds maximum size limit" {
		t.Errorf("expected error 'file exceeds maximum size limit', got %s", response.Error)
	}
}

func TestFileHandler_Upload_ServiceError(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		uploadFn: func(ctx context.Context, input service.UploadInput) (*model.File, error) {
			return nil, errors.New("storage error")
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"test.txt": []byte("content"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestFileHandler_Upload_ShareInternalError(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	mockFileSvc := &mockFileHandlerFileService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"test.txt": []byte("content"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestFileHandler_Upload_InvalidMultipartForm(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	// Send non-multipart request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares/"+shareID+"/files", bytes.NewBufferString("not multipart"))
	req.Header.Set("Content-Type", "application/json")
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestFileHandler_Upload_NilCreatorID(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	share := newTestShare(shareID, userID)
	share.CreatorID = nil // Set to nil

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	files := map[string][]byte{
		"test.txt": []byte("content"),
	}
	req := createMultipartRequest(t, "/api/v1/shares/"+shareID+"/files", files)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 for info hiding (nil creator means no ownership)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestFileHandler_Delete_Success(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	fileID := "file-123"
	share := newTestShare(shareID, userID)
	file := newTestFile(fileID, shareID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			if id != shareID {
				t.Errorf("expected shareID %s, got %s", shareID, id)
			}
			return share, nil
		},
	}

	deleteCalled := false
	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			if id != fileID {
				t.Errorf("expected fileID %s, got %s", fileID, id)
			}
			return file, nil
		},
		deleteFn: func(ctx context.Context, id string) error {
			deleteCalled = true
			if id != fileID {
				t.Errorf("expected fileID %s, got %s", fileID, id)
			}
			return nil
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !deleteCalled {
		t.Error("expected delete to be called")
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

func TestFileHandler_Delete_Unauthenticated(t *testing.T) {
	mockFileSvc := &mockFileHandlerFileService{}
	mockShareSvc := &mockFileHandlerShareService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/file-123", nil)
	// No user context
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestFileHandler_Delete_FileNotFound(t *testing.T) {
	userID := "user-123"
	fileID := "nonexistent"

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return nil, service.ErrFileNotFound
		},
	}

	mockShareSvc := &mockFileHandlerShareService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestFileHandler_Delete_NotOwner(t *testing.T) {
	userID := "user-123"
	otherUserID := "user-456"
	shareID := "share-123"
	fileID := "file-123"
	share := newTestShare(shareID, otherUserID)
	file := newTestFile(fileID, shareID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 for info hiding
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestFileHandler_Delete_ShareNotFound(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	fileID := "file-123"
	file := newTestFile(fileID, shareID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, service.ErrShareNotFound
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 (share not found implies file not accessible)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestFileHandler_Delete_FileServiceError(t *testing.T) {
	userID := "user-123"
	fileID := "file-123"

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return nil, errors.New("database error")
		},
	}

	mockShareSvc := &mockFileHandlerShareService{}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestFileHandler_Delete_ShareServiceError(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	fileID := "file-123"
	file := newTestFile(fileID, shareID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return nil, errors.New("database error")
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestFileHandler_Delete_DeleteError(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	fileID := "file-123"
	share := newTestShare(shareID, userID)
	file := newTestFile(fileID, shareID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
		deleteFn: func(ctx context.Context, id string) error {
			return errors.New("storage error")
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestFileHandler_Delete_NilCreatorID(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	fileID := "file-123"
	share := newTestShare(shareID, userID)
	share.CreatorID = nil // Set to nil
	file := newTestFile(fileID, shareID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 404 for info hiding (nil creator means no ownership)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestFileHandler_Delete_FileNotFoundOnDelete(t *testing.T) {
	userID := "user-123"
	shareID := "share-123"
	fileID := "file-123"
	share := newTestShare(shareID, userID)
	file := newTestFile(fileID, shareID)

	mockShareSvc := &mockFileHandlerShareService{
		getByIDFn: func(ctx context.Context, id string) (*model.Share, error) {
			return share, nil
		},
	}

	mockFileSvc := &mockFileHandlerFileService{
		getByIDFn: func(ctx context.Context, id string) (*model.File, error) {
			return file, nil
		},
		deleteFn: func(ctx context.Context, id string) error {
			return service.ErrFileNotFound
		},
	}

	h := handler.NewFileHandler(mockFileSvc, mockShareSvc)
	router := setupFileRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req = withFileUserContext(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestNewFileHandler_WithOptions(t *testing.T) {
	mockFileSvc := &mockFileHandlerFileService{}
	mockShareSvc := &mockFileHandlerShareService{}

	// Test that WithMaxFileSize option is applied
	customSize := int64(50 << 20) // 50MB
	h := handler.NewFileHandler(mockFileSvc, mockShareSvc, handler.WithMaxFileSize(customSize))

	if h == nil {
		t.Fatal("expected handler to be created")
	}
}
