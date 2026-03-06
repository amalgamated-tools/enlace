package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
	"github.com/amalgamated-tools/enlace/internal/storage"
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
	settingsRepo SettingsRepositoryInterface
	webhooks     WebhookEmitter
}

// FileHandlerOption configures a FileHandler.
type FileHandlerOption func(*FileHandler)

// WithMaxFileSize sets the maximum file size for uploads.
func WithMaxFileSize(size int64) FileHandlerOption {
	return func(h *FileHandler) {
		h.maxFileSize = size
	}
}

// WithSettingsRepo sets the settings repository for dynamic file restrictions.
func WithSettingsRepo(repo SettingsRepositoryInterface) FileHandlerOption {
	return func(h *FileHandler) {
		h.settingsRepo = repo
	}
}

// WithFileWebhookEmitter sets the webhook emitter used for file events.
func WithFileWebhookEmitter(emitter WebhookEmitter) FileHandlerOption {
	return func(h *FileHandler) {
		h.webhooks = emitter
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

// Upload handles POST /api/v1/shares/{id}/files - uploads files to a share.
//
//	@Summary		Upload files to a share
//	@Description	Uploads one or more files to a share using multipart/form-data. Only the share owner may upload files.
//	@Tags			files
//	@Accept		multipart/form-data
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string	true	"Share ID (UUID)"
//	@Param		files	formData	file	true	"Files to upload"
//	@Success	201		{object}	APIResponse{data=[]fileResponse}
//	@Failure	400		{object}	APIResponse
//	@Failure	401		{object}	APIResponse
//	@Failure	404		{object}	APIResponse
//	@Failure	500		{object}	APIResponse
//	@Router		/api/v1/shares/{id}/files [post]
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

	// Read admin-configured file restrictions dynamically
	effectiveMaxSize, blockedExtensions := loadEffectiveRestrictions(r.Context(), h.settingsRepo, h.maxFileSize)

	for _, fileHeader := range files {
		// Check blocked extension
		if IsExtensionBlocked(fileHeader.Filename, blockedExtensions) {
			Error(w, http.StatusBadRequest, "file extension is not allowed")
			return
		}

		// Check file size
		if fileHeader.Size > effectiveMaxSize {
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
			if errors.Is(err, service.ErrInvalidFilename) || errors.Is(err, storage.ErrInvalidKey) {
				Error(w, http.StatusBadRequest, "invalid filename")
				return
			}
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

	if h.webhooks != nil && share.CreatorID != nil && *share.CreatorID != "" {
		creatorID := *share.CreatorID
		uploaded := make([]map[string]interface{}, 0, len(uploadedFiles))
		for _, item := range uploadedFiles {
			uploaded = append(uploaded, map[string]interface{}{
				"id":        item.ID,
				"name":      item.Name,
				"size":      item.Size,
				"mime_type": item.MimeType,
			})
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := h.webhooks.Emit(ctx, service.WebhookEvent{
				Type:      "file.upload.completed",
				CreatorID: creatorID,
				ActorID:   userID,
				Resource:  shareID,
				Data: map[string]interface{}{
					"share_id": shareID,
					"count":    len(uploadedFiles),
					"files":    uploaded,
				},
			}); err != nil {
				slog.Warn("failed to emit webhook", "event_type", "file.upload.completed", "share_id", shareID, "error", err)
			}
		}()
	}

	Success(w, http.StatusCreated, uploadedFiles)
}

// ListByShare handles GET /api/v1/shares/{id}/files - lists files for a share.
//
//	@Summary		List files in a share
//	@Description	Returns all files attached to a share owned by the current user.
//	@Tags			files
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Share ID (UUID)"
//	@Success	200	{object}	APIResponse{data=[]fileResponse}
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/shares/{id}/files [get]
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
//
//	@Summary		Delete a file
//	@Description	Permanently deletes a file. Only the owner of the share containing this file may delete it.
//	@Tags			files
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"File ID (UUID)"
//	@Success	200	{object}	APIResponse
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/files/{id} [delete]
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
