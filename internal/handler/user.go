package handler

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// UserServiceInterface defines the interface for user service operations.
// Using an interface allows for easier testing with mocks.
type UserServiceInterface interface {
	GetUser(ctx context.Context, userID string) (*model.User, error)
	UpdateProfile(ctx context.Context, userID, displayName, email string) (*model.User, error)
	UpdatePassword(ctx context.Context, userID, oldPassword, newPassword string) error
}

// UserHandler handles user profile-related HTTP requests.
type UserHandler struct {
	authService UserServiceInterface
}

// NewUserHandler creates a new UserHandler instance.
func NewUserHandler(authService UserServiceInterface) *UserHandler {
	return &UserHandler{
		authService: authService,
	}
}

// updateProfileRequest represents the request body for updating user profile.
type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Email       *string `json:"email"`
}

// updatePasswordRequest represents the request body for changing password.
type updatePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// profileResponse represents the user profile in API responses (without sensitive fields).
type profileResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
	OIDCLinked  bool   `json:"oidc_linked"`
	HasPassword bool   `json:"has_password"`
}

// Email validation regex pattern for user handler.
var userEmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// GetProfile handles GET /api/v1/me - get current user profile.
//
//	@Summary	Get current user profile
//	@Tags		user
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	APIResponse{data=profileResponse}
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/me [get]
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.authService.GetUser(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	Success(w, http.StatusOK, profileResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		OIDCLinked:  user.OIDCSubject != "",
		HasPassword: user.PasswordHash != "",
	})
}

// UpdateProfile handles PATCH /api/v1/me - update user profile.
//
//	@Summary	Update current user profile
//	@Tags		user
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		updateProfileRequest	true	"Fields to update"
//	@Success	200		{object}	APIResponse{data=profileResponse}
//	@Failure	400		{object}	ValidationErrorResponse
//	@Failure	401		{object}	APIResponse
//	@Failure	409		{object}	APIResponse
//	@Failure	500		{object}	APIResponse
//	@Router		/api/v1/me [patch]
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req updateProfileRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if at least one field is provided
	if req.DisplayName == nil && req.Email == nil {
		Error(w, http.StatusBadRequest, "at least one field (display_name or email) is required")
		return
	}

	// Validate input
	fieldErrors := h.validateUpdateProfileRequest(req)
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	// Extract values for service call
	displayName := ""
	email := ""
	if req.DisplayName != nil {
		displayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Email != nil {
		email = strings.TrimSpace(*req.Email)
	}

	user, err := h.authService.UpdateProfile(r.Context(), userID, displayName, email)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	Success(w, http.StatusOK, profileResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		OIDCLinked:  user.OIDCSubject != "",
		HasPassword: user.PasswordHash != "",
	})
}

// UpdatePassword handles PUT /api/v1/me/password - change user password.
//
//	@Summary	Change password
//	@Tags		user
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		updatePasswordRequest	true	"Old and new password"
//	@Success	200		{object}	APIResponse
//	@Failure	400		{object}	ValidationErrorResponse
//	@Failure	401		{object}	APIResponse
//	@Failure	500		{object}	APIResponse
//	@Router		/api/v1/me/password [put]
func (h *UserHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req updatePasswordRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	fieldErrors := h.validateUpdatePasswordRequest(req)
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	err := h.authService.UpdatePassword(r.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	Success(w, http.StatusOK, nil)
}

// validateUpdateProfileRequest validates the update profile request fields.
func (h *UserHandler) validateUpdateProfileRequest(req updateProfileRequest) map[string]string {
	errs := make(map[string]string)

	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			errs["email"] = "email cannot be empty"
		} else if !userEmailRegex.MatchString(email) {
			errs["email"] = "invalid email format"
		}
	}

	if req.DisplayName != nil {
		displayName := strings.TrimSpace(*req.DisplayName)
		if displayName == "" {
			errs["display_name"] = "display_name cannot be empty"
		}
	}

	return errs
}

// validateUpdatePasswordRequest validates the update password request fields.
func (h *UserHandler) validateUpdatePasswordRequest(req updatePasswordRequest) map[string]string {
	errs := make(map[string]string)

	if req.OldPassword == "" {
		errs["old_password"] = "old_password is required"
	}

	if req.NewPassword == "" {
		errs["new_password"] = "new_password is required"
	} else if len(req.NewPassword) < 8 {
		errs["new_password"] = "new_password must be at least 8 characters"
	}

	return errs
}

// handleServiceError maps service errors to HTTP responses.
func (h *UserHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		Error(w, http.StatusNotFound, "user not found")
	case errors.Is(err, service.ErrEmailExists):
		Error(w, http.StatusConflict, "email already exists")
	case errors.Is(err, service.ErrInvalidCredentials):
		Error(w, http.StatusUnauthorized, "invalid credentials")
	default:
		Error(w, http.StatusInternalServerError, "internal server error")
	}
}
