package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

// Share access token expiration (1 hour).
const shareAccessTokenExpiry = time.Hour

// ShareAccessClaims represents the JWT claims for share access tokens.
type ShareAccessClaims struct {
	ShareID string `json:"share_id"`
	jwt.RegisteredClaims
}

// PublicShareServiceInterface defines the interface for share service operations needed by PublicHandler.
type PublicShareServiceInterface interface {
	GetBySlug(ctx context.Context, slug string) (*model.Share, error)
	GetByID(ctx context.Context, id string) (*model.Share, error)
	VerifyPassword(ctx context.Context, id string, password string) bool
	ValidateAccess(ctx context.Context, share *model.Share) error
	IncrementViewCount(ctx context.Context, id string) error
	IncrementDownloadCount(ctx context.Context, id string) error
}

// PublicFileServiceInterface defines the interface for file service operations needed by PublicHandler.
type PublicFileServiceInterface interface {
	ListByShare(ctx context.Context, shareID string) ([]*model.File, error)
	GetByID(ctx context.Context, id string) (*model.File, error)
	GetContent(ctx context.Context, id string) (io.ReadCloser, *model.File, error)
	Upload(ctx context.Context, input service.UploadInput) (*model.File, error)
	InitiateDirectUpload(ctx context.Context, input service.DirectUploadInput) (*service.DirectUploadResponse, error)
	FinalizeDirectUpload(ctx context.Context, uploadID string) (*model.File, error)
	GetPresignedDownloadURL(ctx context.Context, fileID string, expiry time.Duration) (*service.DirectDownloadResponse, error)
}

// PublicHandler handles public share-related HTTP requests (no auth required).
type PublicHandler struct {
	shareService  PublicShareServiceInterface
	fileService   PublicFileServiceInterface
	jwtSecret     []byte
	maxFileSize   int64
	settingsRepo  SettingsRepositoryInterface
	secureCookies bool
	webhooks      WebhookEmitter
	directUpload  bool
	directExpiry  time.Duration
}

// PublicHandlerOption configures a PublicHandler.
type PublicHandlerOption func(*PublicHandler)

// WithPublicMaxFileSize sets the maximum file size for public uploads.
func WithPublicMaxFileSize(size int64) PublicHandlerOption {
	return func(h *PublicHandler) {
		h.maxFileSize = size
	}
}

// WithPublicSettingsRepo sets the settings repository for dynamic file restrictions on public uploads.
func WithPublicSettingsRepo(repo SettingsRepositoryInterface) PublicHandlerOption {
	return func(h *PublicHandler) {
		h.settingsRepo = repo
	}
}

// WithSecureCookies forces the Secure flag on all cookies set by PublicHandler.
func WithSecureCookies(secure bool) PublicHandlerOption {
	return func(h *PublicHandler) {
		h.secureCookies = secure
	}
}

// WithPublicWebhookEmitter sets the webhook emitter used for public-share events.
func WithPublicWebhookEmitter(emitter WebhookEmitter) PublicHandlerOption {
	return func(h *PublicHandler) {
		h.webhooks = emitter
	}
}

func WithPublicDirectTransfer(enabled bool, expiry time.Duration) PublicHandlerOption {
	return func(h *PublicHandler) {
		h.directUpload = enabled
		h.directExpiry = expiry
	}
}

// NewPublicHandler creates a new PublicHandler instance.
func NewPublicHandler(
	shareService PublicShareServiceInterface,
	fileService PublicFileServiceInterface,
	jwtSecret []byte,
	opts ...PublicHandlerOption,
) *PublicHandler {
	h := &PublicHandler{
		shareService: shareService,
		fileService:  fileService,
		jwtSecret:    jwtSecret,
		maxFileSize:  DefaultMaxFileSize,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// publicShareResponse represents a share in public API responses.
type publicShareResponse struct {
	ID             string  `json:"id"`
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	HasPassword    bool    `json:"has_password"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
	MaxDownloads   *int    `json:"max_downloads,omitempty"`
	DownloadCount  int     `json:"download_count"`
	MaxViews       *int    `json:"max_views,omitempty"`
	ViewCount      int     `json:"view_count"`
	IsReverseShare bool    `json:"is_reverse_share"`
	CreatedAt      string  `json:"created_at"`
}

// publicFileResponse represents a file in public API responses.
type publicFileResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// shareDetailsResponse combines share info with files.
type shareDetailsResponse struct {
	Share publicShareResponse  `json:"share"`
	Files []publicFileResponse `json:"files"`
}

// verifyPasswordRequest represents the request body for password verification.
type verifyPasswordRequest struct {
	Password string `json:"password"`
}

// verifyPasswordResponse represents the response for successful password verification.
type verifyPasswordResponse struct {
	Token string `json:"token"`
}

// ViewShare handles GET /s/{slug} - retrieves share details and files.
//
//	@Summary		View a public share
//	@Description	Returns share details and files. Requires X-Share-Token for password-protected shares.
//	@Tags			public
//	@Produce		json
//	@Param			slug	path		string	true	"Share slug"
//	@Success		200		{object}	APIResponse{data=shareDetailsResponse}
//	@Failure		401		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		410		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/s/{slug} [get]
func (h *PublicHandler) ViewShare(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		Error(w, http.StatusBadRequest, "slug is required")
		return
	}

	share, err := h.shareService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Validate share access (expiry, limits)
	if err := h.shareService.ValidateAccess(r.Context(), share); err != nil {
		h.handleAccessError(w, err)
		return
	}

	// Check password protection
	if share.HasPassword() {
		if err := h.validateShareToken(r, share.ID); err != nil {
			Error(w, http.StatusUnauthorized, "password verification required")
			return
		}
	}

	// Increment view count
	if err := h.shareService.IncrementViewCount(r.Context(), share.ID); err != nil {
		slog.Warn("failed to increment view count", "share_id", share.ID, "error", err)
	}

	// Get files for the share
	files, err := h.fileService.ListByShare(r.Context(), share.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve files")
		return
	}

	// Build response
	response := shareDetailsResponse{
		Share: h.toPublicShareResponse(share),
		Files: h.toPublicFileResponseList(files),
	}

	if h.webhooks != nil && share.CreatorID != nil && *share.CreatorID != "" {
		creatorID := *share.CreatorID
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := h.webhooks.Emit(ctx, service.WebhookEvent{
				Type:      "share.viewed",
				CreatorID: creatorID,
				Resource:  share.ID,
				Data: map[string]interface{}{
					"share_id": share.ID,
					"slug":     share.Slug,
				},
			}); err != nil {
				slog.Warn("failed to emit webhook", "event_type", "share.viewed", "share_id", share.ID, "error", err)
			}
		}()
	}

	Success(w, http.StatusOK, response)
}

// VerifyPassword handles POST /s/{slug}/verify - verifies share password.
//
//	@Summary		Verify share password
//	@Description	Returns a share access token (1-hour expiry) for password-protected shares.
//	@Tags			public
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string					true	"Share slug"
//	@Param			body	body		verifyPasswordRequest	true	"Password"
//	@Success		200		{object}	APIResponse{data=verifyPasswordResponse}
//	@Failure		400		{object}	APIResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		410		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/s/{slug}/verify [post]
func (h *PublicHandler) VerifyPassword(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		Error(w, http.StatusBadRequest, "slug is required")
		return
	}

	share, err := h.shareService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Validate share access (expiry, limits)
	if err := h.shareService.ValidateAccess(r.Context(), share); err != nil {
		h.handleAccessError(w, err)
		return
	}

	// Check if share has a password
	if !share.HasPassword() {
		Error(w, http.StatusBadRequest, "share does not require password")
		return
	}

	// Parse request
	var req verifyPasswordRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate password
	if req.Password == "" {
		ValidationError(w, map[string]string{"password": "password is required"})
		return
	}

	// Verify password
	if !h.shareService.VerifyPassword(r.Context(), share.ID, req.Password) {
		Error(w, http.StatusUnauthorized, "invalid password")
		return
	}

	// Generate access token
	token, err := h.generateShareAccessToken(share.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	// Set an HttpOnly, path-scoped cookie so browser-initiated downloads (window.open,
	// window.location.href) are authenticated without exposing the token in the URL.
	http.SetCookie(w, &http.Cookie{
		Name:     "share_token",
		Value:    token,
		Path:     "/s/" + slug,
		MaxAge:   int(shareAccessTokenExpiry.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || r.TLS != nil,
	})

	w.Header().Set("Cache-Control", "no-store")
	Success(w, http.StatusOK, verifyPasswordResponse{Token: token})
}

// DownloadFile handles GET /s/{slug}/files/{fileId} - downloads a file.
//
//	@Summary		Download a file
//	@Description	Downloads a file from a public share. Use X-Share-Token header or the share_token cookie for password-protected shares.
//	@Tags			public
//	@Produce		octet-stream
//	@Param			slug	path		string	true	"Share slug"
//	@Param			fileId	path		string	true	"File ID (UUID)"
//	@Success		200		{file}		binary
//	@Failure		401		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		410		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/s/{slug}/files/{fileId} [get]
func (h *PublicHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "attachment")
}

// GetDownloadURL handles GET /s/{slug}/files/{fileId}/url.
//
//	@Summary		Get direct download URL
//	@Description	Returns a short-lived signed download URL for a public share file after normal access checks succeed.
//	@Tags			public
//	@Produce		json
//	@Param			slug	path		string	true	"Share slug"
//	@Param			fileId	path		string	true	"File ID (UUID)"
//	@Success		200		{object}	APIResponse{data=directDownloadURLResponse}
//	@Failure		401		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		409		{object}	APIResponse
//	@Failure		410		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/s/{slug}/files/{fileId}/url [get]
func (h *PublicHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	if !h.directUpload {
		Error(w, http.StatusConflict, "direct transfer is not enabled")
		return
	}

	slug := chi.URLParam(r, "slug")
	fileID := chi.URLParam(r, "fileId")
	if slug == "" {
		Error(w, http.StatusBadRequest, "slug is required")
		return
	}
	if fileID == "" {
		Error(w, http.StatusBadRequest, "file ID is required")
		return
	}

	share, file, ok := h.loadShareAndFileForDownload(w, r, slug, fileID)
	if !ok {
		return
	}

	if err := h.shareService.IncrementDownloadCount(r.Context(), share.ID); err != nil {
		slog.Warn("failed to increment download count", "share_id", share.ID, "error", err)
	}
	h.emitShareDownloadedWebhook(share, file)

	result, err := h.fileService.GetPresignedDownloadURL(r.Context(), fileID, h.directExpiry)
	if err != nil {
		if errors.Is(err, service.ErrDirectTransferUnsupported) {
			Error(w, http.StatusConflict, "direct transfer is not supported by the configured storage")
			return
		}
		if errors.Is(err, service.ErrFileNotFound) {
			Error(w, http.StatusNotFound, "file not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to generate download url")
		return
	}

	Success(w, http.StatusOK, directDownloadURLResponse{
		URL:       result.URL.URL,
		Method:    result.URL.Method,
		Headers:   result.URL.Headers,
		ExpiresAt: result.URL.ExpiresAt.Format(time.RFC3339),
	})
}

// PreviewFile handles GET /s/{slug}/files/{fileId}/preview - previews a file.
//
//	@Summary		Preview a file
//	@Description	Previews a file inline. Use X-Share-Token header or the share_token cookie for password-protected shares.
//	@Tags			public
//	@Produce		octet-stream
//	@Param			slug	path		string	true	"Share slug"
//	@Param			fileId	path		string	true	"File ID (UUID)"
//	@Success		200		{file}		binary
//	@Failure		401		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		410		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/s/{slug}/files/{fileId}/preview [get]
func (h *PublicHandler) PreviewFile(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "inline")
}

// serveFile handles file serving for both download and preview.
func (h *PublicHandler) serveFile(w http.ResponseWriter, r *http.Request, disposition string) {
	slug := chi.URLParam(r, "slug")
	fileID := chi.URLParam(r, "fileId")

	if slug == "" {
		Error(w, http.StatusBadRequest, "slug is required")
		return
	}
	if fileID == "" {
		Error(w, http.StatusBadRequest, "file ID is required")
		return
	}

	share, file, ok := h.loadShareAndFileForDownload(w, r, slug, fileID)
	if !ok {
		return
	}

	// Increment download count
	if err := h.shareService.IncrementDownloadCount(r.Context(), share.ID); err != nil {
		slog.Warn("failed to increment download count", "share_id", share.ID, "error", err)
	}

	h.emitShareDownloadedWebhook(share, file)

	// Get file content
	content, _, err := h.fileService.GetContent(r.Context(), fileID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve file content")
		return
	}
	defer func() { _ = content.Close() }()

	// Only non-scriptable MIME types are served inline. For scriptable types that could
	// execute code on the app origin, always force an attachment disposition.
	if disposition == "inline" && isScriptableMimeType(file.MimeType) {
		disposition = "attachment"
	}

	// Set headers
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": file.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")

	// Stream content
	if _, err := io.Copy(w, content); err != nil {
		// Response already started, can't send error
		return
	}
}

func (h *PublicHandler) loadShareAndFileForDownload(w http.ResponseWriter, r *http.Request, slug, fileID string) (*model.Share, *model.File, bool) {
	share, err := h.shareService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return nil, nil, false
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return nil, nil, false
	}

	if err := h.shareService.ValidateAccess(r.Context(), share); err != nil {
		h.handleAccessError(w, err)
		return nil, nil, false
	}

	if share.HasPassword() {
		if err := h.validateShareToken(r, share.ID); err != nil {
			Error(w, http.StatusUnauthorized, "password verification required")
			return nil, nil, false
		}
	}

	file, err := h.fileService.GetByID(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, service.ErrFileNotFound) {
			Error(w, http.StatusNotFound, "file not found")
			return nil, nil, false
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve file")
		return nil, nil, false
	}

	if file.ShareID != share.ID {
		Error(w, http.StatusNotFound, "file not found")
		return nil, nil, false
	}

	return share, file, true
}

func (h *PublicHandler) emitShareDownloadedWebhook(share *model.Share, file *model.File) {
	if h.webhooks == nil || share.CreatorID == nil || *share.CreatorID == "" {
		return
	}

	creatorID := *share.CreatorID
	go func(shareID, fileID, fileName string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.webhooks.Emit(ctx, service.WebhookEvent{
			Type:      "share.downloaded",
			CreatorID: creatorID,
			Resource:  shareID,
			Data: map[string]interface{}{
				"share_id": shareID,
				"file_id":  fileID,
				"name":     fileName,
			},
		}); err != nil {
			slog.Warn("failed to emit webhook", "event_type", "share.downloaded", "share_id", shareID, "file_id", fileID, "error", err)
		}
	}(share.ID, file.ID, file.Name)
}

// isScriptableMimeType returns true for MIME types that can execute scripts or
// render active content when served inline in a browser, making them unsafe to
// deliver as inline previews from the application origin.
func isScriptableMimeType(mimeType string) bool {
	switch mimeType {
	case "text/html",
		"application/xhtml+xml",
		"image/svg+xml",
		"application/javascript",
		"text/javascript",
		"text/css",
		"application/xml":
		return true
	}
	return false
}

// UploadToReverseShare handles POST /s/{slug}/upload - uploads files to a reverse share.
//
//	@Summary		Upload to a reverse share
//	@Description	Uploads files to a public reverse share. No authentication required.
//	@Tags			public
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			slug	path		string	true	"Share slug"
//	@Param			files	formData	file	true	"Files to upload"
//	@Success		201		{object}	APIResponse{data=[]publicFileResponse}
//	@Failure		400		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		410		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/s/{slug}/upload [post]
func (h *PublicHandler) UploadToReverseShare(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		Error(w, http.StatusBadRequest, "slug is required")
		return
	}

	// Get share
	share, err := h.shareService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Validate share access
	if err := h.shareService.ValidateAccess(r.Context(), share); err != nil {
		h.handleAccessError(w, err)
		return
	}

	// Verify share is a reverse share
	if !share.IsReverseShare {
		Error(w, http.StatusForbidden, "share does not accept uploads")
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
	uploadedFiles := make([]publicFileResponse, 0, len(files))

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

		// Upload file (no uploader ID for public uploads)
		input := service.UploadInput{
			ShareID:    share.ID,
			UploaderID: "", // Anonymous upload
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

		uploadedFiles = append(uploadedFiles, publicFileResponse{
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
				Resource:  share.ID,
				Data: map[string]interface{}{
					"share_id": share.ID,
					"count":    len(uploadedFiles),
					"files":    uploaded,
				},
			}); err != nil {
				slog.Warn("failed to emit webhook", "event_type", "file.upload.completed", "share_id", share.ID, "error", err)
			}
		}()
	}

	Success(w, http.StatusCreated, uploadedFiles)
}

// InitiateReverseShareUpload handles POST /s/{slug}/upload/initiate.
//
//	@Summary		Initiate reverse-share direct upload
//	@Description	Creates a pending upload and returns a short-lived signed upload URL for a reverse share.
//	@Tags			public
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string						true	"Share slug"
//	@Param			body	body		directUploadInitiateRequest	true	"Upload metadata"
//	@Success		200		{object}	APIResponse{data=directUploadInitiateResponse}
//	@Failure		400		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		409		{object}	APIResponse
//	@Failure		410		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/s/{slug}/upload/initiate [post]
func (h *PublicHandler) InitiateReverseShareUpload(w http.ResponseWriter, r *http.Request) {
	if !h.directUpload {
		Error(w, http.StatusConflict, "direct transfer is not enabled")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		Error(w, http.StatusBadRequest, "slug is required")
		return
	}

	share, err := h.shareService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}
	if err := h.shareService.ValidateAccess(r.Context(), share); err != nil {
		h.handleAccessError(w, err)
		return
	}
	if !share.IsReverseShare {
		Error(w, http.StatusForbidden, "share does not accept uploads")
		return
	}

	var req directUploadInitiateRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	validationErrors := map[string]string{}
	if req.Filename == "" {
		validationErrors["filename"] = "filename is required"
	}
	if req.Size < 0 {
		validationErrors["size"] = "size must be zero or greater"
	}
	if len(validationErrors) > 0 {
		ValidationError(w, validationErrors)
		return
	}

	effectiveMaxSize, blockedExtensions := loadEffectiveRestrictions(r.Context(), h.settingsRepo, h.maxFileSize)
	if IsExtensionBlocked(req.Filename, blockedExtensions) {
		Error(w, http.StatusBadRequest, "file extension is not allowed")
		return
	}
	if req.Size > effectiveMaxSize {
		Error(w, http.StatusBadRequest, "file exceeds maximum size limit")
		return
	}

	upload, err := h.fileService.InitiateDirectUpload(r.Context(), service.DirectUploadInput{
		ShareID:  share.ID,
		Filename: req.Filename,
		Size:     req.Size,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDirectTransferUnsupported):
			Error(w, http.StatusConflict, "direct transfer is not supported by the configured storage")
		case errors.Is(err, service.ErrInvalidFilename), errors.Is(err, storage.ErrInvalidKey):
			Error(w, http.StatusBadRequest, "invalid filename")
		default:
			Error(w, http.StatusInternalServerError, "failed to initiate upload")
		}
		return
	}

	finalizeToken, err := generateDirectUploadFinalizeToken(h.jwtSecret, directUploadFinalizeClaims{
		UploadID:   upload.UploadID,
		ShareID:    upload.ShareID,
		Public:     true,
		StorageKey: upload.StorageKey,
	}, upload.Upload.ExpiresAt)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to initiate upload")
		return
	}

	Success(w, http.StatusOK, directUploadInitiateResponse{
		UploadID:      upload.UploadID,
		FileID:        upload.FileID,
		Filename:      upload.Filename,
		Size:          upload.Size,
		MimeType:      upload.MimeType,
		URL:           upload.Upload.URL,
		Method:        upload.Upload.Method,
		Headers:       upload.Upload.Headers,
		ExpiresAt:     upload.Upload.ExpiresAt.Format(time.RFC3339),
		FinalizeToken: finalizeToken,
	})
}

// FinalizeReverseShareUpload handles POST /s/{slug}/upload/{uploadId}/finalize.
//
//	@Summary		Finalize reverse-share direct upload
//	@Description	Validates a reverse-share direct upload and creates the final file metadata record.
//	@Tags			public
//	@Accept			json
//	@Produce		json
//	@Param			slug		path		string						true	"Share slug"
//	@Param			uploadId	path		string						true	"Upload ID"
//	@Param			body		body		directUploadFinalizeRequest	true	"Finalize token"
//	@Success		201			{object}	APIResponse{data=publicFileResponse}
//	@Failure		400			{object}	APIResponse
//	@Failure		401			{object}	APIResponse
//	@Failure		403			{object}	APIResponse
//	@Failure		404			{object}	APIResponse
//	@Failure		409			{object}	APIResponse
//	@Failure		410			{object}	APIResponse
//	@Failure		500			{object}	APIResponse
//	@Router			/s/{slug}/upload/{uploadId}/finalize [post]
func (h *PublicHandler) FinalizeReverseShareUpload(w http.ResponseWriter, r *http.Request) {
	if !h.directUpload {
		Error(w, http.StatusConflict, "direct transfer is not enabled")
		return
	}

	slug := chi.URLParam(r, "slug")
	uploadID := chi.URLParam(r, "uploadId")
	if slug == "" {
		Error(w, http.StatusBadRequest, "slug is required")
		return
	}
	if uploadID == "" {
		Error(w, http.StatusBadRequest, "upload ID is required")
		return
	}

	share, err := h.shareService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}
	if err := h.shareService.ValidateAccess(r.Context(), share); err != nil {
		h.handleAccessError(w, err)
		return
	}
	if !share.IsReverseShare {
		Error(w, http.StatusForbidden, "share does not accept uploads")
		return
	}

	var req directUploadFinalizeRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims, err := validateDirectUploadFinalizeToken(h.jwtSecret, req.Token)
	if err != nil || !claims.Public || claims.UploadID != uploadID || claims.ShareID != share.ID {
		Error(w, http.StatusUnauthorized, "invalid finalize token")
		return
	}

	file, err := h.fileService.FinalizeDirectUpload(r.Context(), uploadID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDirectTransferUnsupported):
			Error(w, http.StatusConflict, "direct transfer is not supported by the configured storage")
		case errors.Is(err, service.ErrUploadNotFound):
			Error(w, http.StatusNotFound, "upload not found")
		case errors.Is(err, service.ErrUploadExpired):
			Error(w, http.StatusGone, "upload expired")
		case errors.Is(err, service.ErrUploadAlreadyFinalized):
			Error(w, http.StatusConflict, "upload already finalized")
		case errors.Is(err, service.ErrIntegrityCheckFailed):
			Error(w, http.StatusBadRequest, "uploaded object failed validation")
		default:
			Error(w, http.StatusInternalServerError, "failed to finalize upload")
		}
		return
	}

	if h.webhooks != nil && share.CreatorID != nil && *share.CreatorID != "" {
		creatorID := *share.CreatorID
		go func(file *model.File) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := h.webhooks.Emit(ctx, service.WebhookEvent{
				Type:      "file.upload.completed",
				CreatorID: creatorID,
				Resource:  share.ID,
				Data: map[string]interface{}{
					"share_id": share.ID,
					"count":    1,
					"files": []map[string]interface{}{
						{
							"id":        file.ID,
							"name":      file.Name,
							"size":      file.Size,
							"mime_type": file.MimeType,
						},
					},
				},
			}); err != nil {
				slog.Warn("failed to emit webhook", "event_type", "file.upload.completed", "share_id", share.ID, "error", err)
			}
		}(file)
	}

	Success(w, http.StatusCreated, publicFileResponse{
		ID:       file.ID,
		Name:     file.Name,
		Size:     file.Size,
		MimeType: file.MimeType,
	})
}

// generateShareAccessToken creates a JWT for accessing password-protected shares.
func (h *PublicHandler) generateShareAccessToken(shareID string) (string, error) {
	now := time.Now()
	claims := &ShareAccessClaims{
		ShareID: shareID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(shareAccessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}

// validateShareToken validates the share access token from header or cookie.
// It checks the X-Share-Token header first, then falls back to the path-scoped
// share_token cookie set by VerifyPassword. The ?token= query parameter is no
// longer accepted to prevent token leakage via browser history and referrer headers.
func (h *PublicHandler) validateShareToken(r *http.Request, expectedShareID string) error {
	tokenStr := r.Header.Get("X-Share-Token")
	if tokenStr == "" {
		// Fall back to the path-scoped cookie set during password verification.
		// The browser only sends this cookie for requests under /s/{slug}/, so
		// it is automatically scoped to the correct share.
		if cookie, err := r.Cookie("share_token"); err == nil {
			tokenStr = cookie.Value
		}
	}
	if tokenStr == "" {
		return errors.New("share token required")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &ShareAccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return h.jwtSecret, nil
	})

	if err != nil {
		return errors.New("invalid token")
	}

	claims, ok := token.Claims.(*ShareAccessClaims)
	if !ok || !token.Valid {
		return errors.New("invalid token claims")
	}

	if claims.ShareID != expectedShareID {
		return errors.New("token does not match share")
	}

	return nil
}

// handleAccessError maps access validation errors to HTTP responses.
func (h *PublicHandler) handleAccessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrShareExpired):
		Error(w, http.StatusGone, "share has expired")
	case errors.Is(err, service.ErrDownloadLimit):
		Error(w, http.StatusGone, "download limit reached")
	case errors.Is(err, service.ErrViewLimit):
		Error(w, http.StatusGone, "view limit reached")
	default:
		Error(w, http.StatusInternalServerError, "access validation failed")
	}
}

// toPublicShareResponse converts a model.Share to publicShareResponse.
func (h *PublicHandler) toPublicShareResponse(share *model.Share) publicShareResponse {
	resp := publicShareResponse{
		ID:             share.ID,
		Slug:           share.Slug,
		Name:           share.Name,
		Description:    share.Description,
		HasPassword:    share.HasPassword(),
		DownloadCount:  share.DownloadCount,
		ViewCount:      share.ViewCount,
		IsReverseShare: share.IsReverseShare,
		CreatedAt:      share.CreatedAt.Format(time.RFC3339),
	}

	if share.ExpiresAt != nil {
		formatted := share.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &formatted
	}

	if share.MaxDownloads != nil {
		resp.MaxDownloads = share.MaxDownloads
	}

	if share.MaxViews != nil {
		resp.MaxViews = share.MaxViews
	}

	return resp
}

// toPublicFileResponseList converts a slice of model.File to publicFileResponse slice.
func (h *PublicHandler) toPublicFileResponseList(files []*model.File) []publicFileResponse {
	result := make([]publicFileResponse, len(files))
	for i, file := range files {
		result[i] = publicFileResponse{
			ID:       file.ID,
			Name:     file.Name,
			Size:     file.Size,
			MimeType: file.MimeType,
		}
	}
	return result
}
