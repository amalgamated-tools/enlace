package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// ShareServiceInterface defines the interface for share service operations.
type ShareServiceInterface interface {
	Create(ctx context.Context, input service.CreateShareInput) (*model.Share, error)
	GetByID(ctx context.Context, id string) (*model.Share, error)
	Update(ctx context.Context, id string, input service.UpdateShareInput) (*model.Share, error)
	Delete(ctx context.Context, id string) error
	ListByCreator(ctx context.Context, creatorID string) ([]*model.Share, error)
}

// FileServiceInterface defines the interface for file service operations needed by ShareHandler.
type FileServiceInterface interface {
	ListByShare(ctx context.Context, shareID string) ([]*model.File, error)
}

// EmailServiceInterface defines the interface for email notification operations.
type EmailServiceInterface interface {
	IsConfigured() bool
	SendShareNotification(ctx context.Context, share *model.Share, recipients []string) error
	ListRecipients(ctx context.Context, shareID string) ([]*model.ShareRecipient, error)
}

// ShareHandler handles share-related HTTP requests.
type ShareHandler struct {
	shareService ShareServiceInterface
	fileService  FileServiceInterface
	emailService EmailServiceInterface
}

// NewShareHandler creates a new ShareHandler instance.
func NewShareHandler(shareService ShareServiceInterface, fileService FileServiceInterface, emailService EmailServiceInterface) *ShareHandler {
	return &ShareHandler{
		shareService: shareService,
		fileService:  fileService,
		emailService: emailService,
	}
}

// createShareRequest represents the request body for creating a share.
type createShareRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Slug           string   `json:"slug"`
	Password       *string  `json:"password"`
	ExpiresAt      *string  `json:"expires_at"`
	MaxDownloads   *int     `json:"max_downloads"`
	MaxViews       *int     `json:"max_views"`
	IsReverseShare bool     `json:"is_reverse_share"`
	Recipients     []string `json:"recipients"`
}

// updateShareRequest represents the request body for updating a share.
type updateShareRequest struct {
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	Password       *string `json:"password"`
	ClearPassword  *bool   `json:"clear_password"`
	ExpiresAt      *string `json:"expires_at"`
	ClearExpiry    *bool   `json:"clear_expiry"`
	MaxDownloads   *int    `json:"max_downloads"`
	MaxViews       *int    `json:"max_views"`
	IsReverseShare *bool   `json:"is_reverse_share"`
}

// shareResponse represents a share in API responses.
type shareResponse struct {
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
	UpdatedAt      string  `json:"updated_at"`
}

// List handles GET /api/v1/shares - lists all shares for the authenticated user.
//
//	@Summary	List shares
//	@Tags		shares
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	APIResponse{data=[]shareResponse}
//	@Failure	401	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/shares [get]
func (h *ShareHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	shares, err := h.shareService.ListByCreator(r.Context(), userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve shares")
		return
	}

	// Convert to response format
	response := make([]shareResponse, len(shares))
	for i, share := range shares {
		response[i] = h.toShareResponse(share)
	}

	Success(w, http.StatusOK, response)
}

// Create handles POST /api/v1/shares - creates a new share.
//
//	@Summary	Create a share
//	@Tags		shares
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		createShareRequest	true	"Share details"
//	@Success	201		{object}	APIResponse{data=shareResponse}
//	@Failure	400		{object}	ValidationErrorResponse
//	@Failure	401		{object}	APIResponse
//	@Failure	409		{object}	APIResponse
//	@Failure	500		{object}	APIResponse
//	@Router		/api/v1/shares [post]
func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createShareRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	fieldErrors := h.validateCreateRequest(req)
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	// Validate recipient emails if provided
	var validRecipients []string
	for _, email := range req.Recipients {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		if !validateEmail(email) {
			ValidationError(w, map[string]string{"recipients": "invalid email address: " + email})
			return
		}
		validRecipients = append(validRecipients, email)
	}

	// Parse expiry time if provided
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			ValidationError(w, map[string]string{"expires_at": "invalid date format, use RFC3339"})
			return
		}
		expiresAt = &parsed
	}

	// Create share input
	input := service.CreateShareInput{
		CreatorID:      userID,
		Name:           strings.TrimSpace(req.Name),
		Description:    strings.TrimSpace(req.Description),
		Slug:           strings.TrimSpace(req.Slug),
		Password:       req.Password,
		ExpiresAt:      expiresAt,
		MaxDownloads:   req.MaxDownloads,
		MaxViews:       req.MaxViews,
		IsReverseShare: req.IsReverseShare,
	}

	share, err := h.shareService.Create(r.Context(), input)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Send email notifications in background (non-blocking).
	// Use WithoutCancel so the goroutine isn't canceled when the handler returns,
	// while still carrying request-scoped values (e.g. trace IDs) for logging.
	if len(validRecipients) > 0 && h.emailService != nil && h.emailService.IsConfigured() {
		bgCtx := context.WithoutCancel(r.Context())
		go func() {
			ctx, cancel := context.WithTimeout(bgCtx, 30*time.Second)
			defer cancel()

			if err := h.emailService.SendShareNotification(ctx, share, validRecipients); err != nil {
				slog.ErrorContext(ctx, "failed to send share notifications", slog.Any("error", err))
			}
		}()
	}

	Success(w, http.StatusCreated, h.toShareResponse(share))
}

// Get handles GET /api/v1/shares/{id} - retrieves a specific share.
//
//	@Summary	Get a share
//	@Tags		shares
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Share ID (UUID)"
//	@Success	200	{object}	APIResponse{data=shareResponse}
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/shares/{id} [get]
func (h *ShareHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	share, err := h.shareService.GetByID(r.Context(), shareID)
	if err != nil {
		// Return 404 for both not found and unauthorized (info hiding)
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

	Success(w, http.StatusOK, h.toShareResponse(share))
}

// Update handles PATCH /api/v1/shares/{id} - updates an existing share.
//
//	@Summary	Update a share
//	@Tags		shares
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string				true	"Share ID (UUID)"
//	@Param		body	body		updateShareRequest	true	"Fields to update"
//	@Success	200		{object}	APIResponse{data=shareResponse}
//	@Failure	400		{object}	ValidationErrorResponse
//	@Failure	401		{object}	APIResponse
//	@Failure	404		{object}	APIResponse
//	@Failure	500		{object}	APIResponse
//	@Router		/api/v1/shares/{id} [patch]
func (h *ShareHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	// First verify ownership
	existingShare, err := h.shareService.GetByID(r.Context(), shareID)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Check ownership - return 404 for unauthorized (info hiding)
	if existingShare.CreatorID == nil || *existingShare.CreatorID != userID {
		Error(w, http.StatusNotFound, "share not found")
		return
	}

	var req updateShareRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	fieldErrors := h.validateUpdateRequest(req)
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	// Parse expiry time if provided
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			ValidationError(w, map[string]string{"expires_at": "invalid date format, use RFC3339"})
			return
		}
		expiresAt = &parsed
	}

	// Build update input
	input := service.UpdateShareInput{
		Name:           req.Name,
		Description:    req.Description,
		Password:       req.Password,
		ExpiresAt:      expiresAt,
		MaxDownloads:   req.MaxDownloads,
		MaxViews:       req.MaxViews,
		IsReverseShare: req.IsReverseShare,
	}

	// Handle clear flags
	if req.ClearPassword != nil && *req.ClearPassword {
		input.ClearPassword = true
	}
	if req.ClearExpiry != nil && *req.ClearExpiry {
		input.ClearExpiry = true
	}

	share, err := h.shareService.Update(r.Context(), shareID, input)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	Success(w, http.StatusOK, h.toShareResponse(share))
}

// Delete handles DELETE /api/v1/shares/{id} - deletes a share.
//
//	@Summary	Delete a share
//	@Tags		shares
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Share ID (UUID)"
//	@Success	200	{object}	APIResponse
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/shares/{id} [delete]
func (h *ShareHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	// First verify ownership
	existingShare, err := h.shareService.GetByID(r.Context(), shareID)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Check ownership - return 404 for unauthorized (info hiding)
	if existingShare.CreatorID == nil || *existingShare.CreatorID != userID {
		Error(w, http.StatusNotFound, "share not found")
		return
	}

	if err := h.shareService.Delete(r.Context(), shareID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	Success(w, http.StatusOK, nil)
}

// validateCreateRequest validates the create share request fields.
func (h *ShareHandler) validateCreateRequest(req createShareRequest) map[string]string {
	errs := make(map[string]string)

	name := strings.TrimSpace(req.Name)
	if name == "" {
		errs["name"] = "name is required"
	} else if len(name) > 255 {
		errs["name"] = "name must be 255 characters or less"
	}

	slug := strings.TrimSpace(req.Slug)
	if slug != "" {
		if len(slug) < 3 {
			errs["slug"] = "slug must be at least 3 characters"
		} else if len(slug) > 50 {
			errs["slug"] = "slug must be 50 characters or less"
		} else if !isValidSlug(slug) {
			errs["slug"] = "slug must contain only lowercase letters, numbers, and hyphens"
		}
	}

	if req.MaxDownloads != nil && *req.MaxDownloads < 0 {
		errs["max_downloads"] = "max_downloads must be non-negative"
	}

	if req.MaxViews != nil && *req.MaxViews < 0 {
		errs["max_views"] = "max_views must be non-negative"
	}

	return errs
}

// validateUpdateRequest validates the update share request fields.
func (h *ShareHandler) validateUpdateRequest(req updateShareRequest) map[string]string {
	errs := make(map[string]string)

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			errs["name"] = "name cannot be empty"
		} else if len(name) > 255 {
			errs["name"] = "name must be 255 characters or less"
		}
	}

	if req.MaxDownloads != nil && *req.MaxDownloads < 0 {
		errs["max_downloads"] = "max_downloads must be non-negative"
	}

	if req.MaxViews != nil && *req.MaxViews < 0 {
		errs["max_views"] = "max_views must be non-negative"
	}

	return errs
}

// isValidSlug checks if a string is a valid URL slug.
func isValidSlug(s string) bool {
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	// Cannot start or end with hyphen
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	return true
}

// toShareResponse converts a model.Share to shareResponse.
func (h *ShareHandler) toShareResponse(share *model.Share) shareResponse {
	resp := shareResponse{
		ID:             share.ID,
		Slug:           share.Slug,
		Name:           share.Name,
		Description:    share.Description,
		HasPassword:    share.HasPassword(),
		DownloadCount:  share.DownloadCount,
		ViewCount:      share.ViewCount,
		IsReverseShare: share.IsReverseShare,
		CreatedAt:      share.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      share.UpdatedAt.Format(time.RFC3339),
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

// handleServiceError maps service errors to HTTP responses.
func (h *ShareHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrShareNotFound):
		Error(w, http.StatusNotFound, "share not found")
	case errors.Is(err, service.ErrSlugExists):
		Error(w, http.StatusConflict, "slug already exists")
	default:
		Error(w, http.StatusInternalServerError, "internal server error")
	}
}

// sendNotificationRequest represents the request body for sending share notifications.
type sendNotificationRequest struct {
	Recipients []string `json:"recipients" example:"user@example.com"`
}

// recipientResponse represents a share recipient in API responses.
type recipientResponse struct {
	ID     string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email  string `json:"email" example:"user@example.com"`
	SentAt string `json:"sent_at" example:"2024-01-01T00:00:00Z"`
}

// validateEmail checks that an email address is a bare valid address using
// net/mail.ParseAddress. It rejects display-name forms like "Name <user@example.com>"
// and addresses containing CRLF characters that could enable header injection.
func validateEmail(email string) bool {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return false
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return false
	}
	// Only accept bare addresses, not display names or comments like "Name <user@example.com>".
	if addr.Name != "" {
		return false
	}
	// Ensure the parsed address exactly matches the original (trimmed) input.
	return addr.Address == trimmed
}

// SendNotification handles POST /api/v1/shares/{id}/notify - sends email notifications for a share.
//
//	@Summary	Send share notification emails
//	@Tags		shares
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string					true	"Share ID (UUID)"
//	@Param		body	body		sendNotificationRequest	true	"Recipient emails"
//	@Success	200		{object}	APIResponse
//	@Failure	400		{object}	ValidationErrorResponse
//	@Failure	401		{object}	APIResponse
//	@Failure	404		{object}	APIResponse
//	@Failure	500		{object}	APIResponse
//	@Router		/api/v1/shares/{id}/notify [post]
func (h *ShareHandler) SendNotification(w http.ResponseWriter, r *http.Request) {
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

	share, err := h.shareService.GetByID(r.Context(), shareID)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Check ownership
	if share.CreatorID == nil || *share.CreatorID != userID {
		Error(w, http.StatusNotFound, "share not found")
		return
	}

	var req sendNotificationRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Filter and validate recipients
	var validRecipients []string
	for _, email := range req.Recipients {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		if !validateEmail(email) {
			ValidationError(w, map[string]string{"recipients": "invalid email address: " + email})
			return
		}
		validRecipients = append(validRecipients, email)
	}

	if len(validRecipients) == 0 {
		ValidationError(w, map[string]string{"recipients": "at least one valid email address is required"})
		return
	}

	if h.emailService == nil || !h.emailService.IsConfigured() {
		Error(w, http.StatusInternalServerError, "email notifications are not configured")
		return
	}

	if err := h.emailService.SendShareNotification(r.Context(), share, validRecipients); err != nil {
		Error(w, http.StatusInternalServerError, "failed to send notifications")
		return
	}

	Success(w, http.StatusOK, map[string]string{"message": "notifications sent"})
}

// ListRecipients handles GET /api/v1/shares/{id}/recipients - lists notification recipients for a share.
//
//	@Summary	List share notification recipients
//	@Tags		shares
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Share ID (UUID)"
//	@Success	200	{object}	APIResponse{data=[]recipientResponse}
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/shares/{id}/recipients [get]
func (h *ShareHandler) ListRecipients(w http.ResponseWriter, r *http.Request) {
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

	share, err := h.shareService.GetByID(r.Context(), shareID)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			Error(w, http.StatusNotFound, "share not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve share")
		return
	}

	// Check ownership
	if share.CreatorID == nil || *share.CreatorID != userID {
		Error(w, http.StatusNotFound, "share not found")
		return
	}

	if h.emailService == nil {
		Success(w, http.StatusOK, []recipientResponse{})
		return
	}

	recipients, err := h.emailService.ListRecipients(r.Context(), shareID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve recipients")
		return
	}

	response := make([]recipientResponse, len(recipients))
	for i, rec := range recipients {
		response[i] = recipientResponse{
			ID:     rec.ID,
			Email:  rec.Email,
			SentAt: rec.SentAt.Format(time.RFC3339),
		}
	}

	Success(w, http.StatusOK, response)
}
