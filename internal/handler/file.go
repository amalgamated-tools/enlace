package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// Default limits for file uploads.
const (
	// DefaultMaxFileSize is the default maximum size per file (100MB).
	DefaultMaxFileSize = 100 << 20 // 100MB

	// DefaultMaxMemory is the maximum memory used during multipart parsing.
	// Files exceeding this are stored in temporary files.
	DefaultMaxMemory = 32 << 20 // 32MB
)

// FileHandlerShareService defines the interface for share service operations needed by FileHandler.
type FileHandlerShareService interface {
	GetByID(ctx context.Context, id string) (*model.Share, error)
}

// FileHandlerFileService defines the interface for file service operations needed by FileHandler.
type FileHandlerFileService interface {
	Upload(ctx context.Context, input service.UploadInput) (*model.File, error)
	GetByID(ctx context.Context, id string) (*model.File, error)
	Delete(ctx context.Context, id string) error
	ListByShare(ctx context.Context, shareID string) ([]*model.File, error)
}

// FileHandler handles file-related HTTP requests.
type FileHandler struct {
	fileService  FileHandlerFileService
	shareService FileHandlerShareService
	maxFileSize  int64
}

// FileHandlerOption configures a FileHandler.
type FileHandlerOption func(*FileHandler)

// WithMaxFileSize sets the maximum file size for uploads.
func WithMaxFileSize(size int64) FileHandlerOption {
	return func(h *FileHandler) {
		h.maxFileSize = size
	}
}

// NewFileHandler creates a new FileHandler instance.
func NewFileHandler(fileService FileHandlerFileService, shareService FileHandlerShareService, opts ...FileHandlerOption) *FileHandler {
	h := &FileHandler{
		fileService:  fileService,
		shareService: shareService,
		maxFileSize:  DefaultMaxFileSize,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// fileResponse represents a file in API responses.
type fileResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// Upload handles POST /api/v1/shares/{shareId}/files - uploads files to a share.
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	shareID := chi.URLParam(r, "id")
	if shareID == "" {
		Error(w, http.StatusBadRequest, "share ID is required")
		return
	}

	// Verify share exists and user owns it
	share, err := h.shareService.GetByID(r.Context(), shareID)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Check ownership - return 404 for unauthorized (info hiding)
	if share.CreatorID == nil || *share.CreatorID != userID {
		Error(w, http.StatusNotFound, "share not found")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(DefaultMaxMemory); err != nil {
		Error(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	// Get files from form
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		Error(w, http.StatusBadRequest, "no files provided")
		return
	}

	// Upload each file
	uploadedFiles := make([]fileResponse, 0, len(files))
	for _, fileHeader := range files {
		// Check file size
		if fileHeader.Size > h.maxFileSize {
			Error(w, http.StatusBadRequest, "file exceeds maximum size limit")
			return
		}

		// Open file
		file, err := fileHeader.Open()
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to read uploaded file")
			return
		}

		// Upload file
		input := service.UploadInput{
			ShareID:    shareID,
			UploaderID: userID,
			Filename:   fileHeader.Filename,
			Content:    file,
			Size:       fileHeader.Size,
		}

		uploadedFile, err := h.fileService.Upload(r.Context(), input)
		// Close file after upload attempt
		_ = file.Close()

		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to upload file")
			return
		}

		uploadedFiles = append(uploadedFiles, fileResponse{
			ID:       uploadedFile.ID,
			Name:     uploadedFile.Name,
			Size:     uploadedFile.Size,
			MimeType: uploadedFile.MimeType,
		})
	}

	Success(w, http.StatusCreated, uploadedFiles)
}

// ListByShare handles GET /api/v1/shares/{id}/files - lists files for a share.
func (h *FileHandler) ListByShare(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	shareID := chi.URLParam(r, "id")
	if shareID == "" {
		Error(w, http.StatusBadRequest, "share ID is required")
		return
	}

	// Verify share exists and user owns it
	share, err := h.shareService.GetByID(r.Context(), shareID)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Check ownership - return 404 for unauthorized (info hiding)
	if share.CreatorID == nil || *share.CreatorID != userID {
		Error(w, http.StatusNotFound, "share not found")
		return
	}

	files, err := h.fileService.ListByShare(r.Context(), shareID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve files")
		return
	}

	response := make([]fileResponse, len(files))
	for i, f := range files {
		response[i] = fileResponse{
			ID:       f.ID,
			Name:     f.Name,
			Size:     f.Size,
			MimeType: f.MimeType,
		}
	}

	Success(w, http.StatusOK, response)
}

// Delete handles DELETE /api/v1/files/{id} - deletes a file.
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	fileID := chi.URLParam(r, "id")
	if fileID == "" {
		Error(w, http.StatusBadRequest, "file ID is required")
		return
	}

	// Get file to find its share
	file, err := h.fileService.GetByID(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, service.ErrFileNotFound) {
			Error(w, http.StatusNotFound, "file not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve file")
		return
	}

	// Get share to verify ownership
	share, err := h.shareService.GetByID(r.Context(), file.ShareID)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			// Share doesn't exist but file does - data inconsistency
			Error(w, http.StatusNotFound, "file not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Check ownership - return 404 for unauthorized (info hiding)
	if share.CreatorID == nil || *share.CreatorID != userID {
		Error(w, http.StatusNotFound, "file not found")
		return
	}

	// Delete file
	if err := h.fileService.Delete(r.Context(), fileID); err != nil {
		if errors.Is(err, service.ErrFileNotFound) {
			Error(w, http.StatusNotFound, "file not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	Success(w, http.StatusOK, nil)
}
