package handler

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

const bcryptCost = 12

// UserRepositoryInterface defines the interface for user repository operations.
type UserRepositoryInterface interface {
	List(ctx context.Context) ([]*model.User, error)
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id string) error
	EmailExists(ctx context.Context, email string) (bool, error)
}

// AdminHandler handles admin-related HTTP requests.
type AdminHandler struct {
	userRepo UserRepositoryInterface
}

// NewAdminHandler creates a new AdminHandler instance.
func NewAdminHandler(userRepo UserRepositoryInterface) *AdminHandler {
	return &AdminHandler{
		userRepo: userRepo,
	}
}

// createUserRequest represents the request body for creating a user.
type createUserRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
}

// updateUserRequest represents the request body for updating a user.
type updateUserRequest struct {
	Email       *string `json:"email"`
	Password    *string `json:"password"`
	DisplayName *string `json:"display_name"`
	IsAdmin     *bool   `json:"is_admin"`
}

// adminUserResponse represents a user in admin API responses (without password_hash).
type adminUserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Email validation regex pattern for admin handler.
var adminEmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// ListUsers handles GET /api/v1/admin/users - lists all users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.List(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve users")
		return
	}

	// Convert to response format
	response := make([]adminUserResponse, len(users))
	for i, user := range users {
		response[i] = h.toUserResponse(user)
	}

	Success(w, http.StatusOK, response)
}

// CreateUser handles POST /api/v1/admin/users - creates a new user.
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
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

	// Check if email already exists
	exists, err := h.userRepo.EmailExists(r.Context(), strings.TrimSpace(req.Email))
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to check email availability")
		return
	}
	if exists {
		Error(w, http.StatusConflict, "email already exists")
		return
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	// Create user
	user := &model.User{
		ID:           uuid.NewString(),
		Email:        strings.TrimSpace(req.Email),
		PasswordHash: string(passwordHash),
		DisplayName:  strings.TrimSpace(req.DisplayName),
		IsAdmin:      req.IsAdmin,
	}

	if err := h.userRepo.Create(r.Context(), user); err != nil {
		Error(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	Success(w, http.StatusCreated, h.toUserResponse(user))
}

// GetUser handles GET /api/v1/admin/users/{id} - retrieves a specific user.
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		Error(w, http.StatusBadRequest, "user ID is required")
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			Error(w, http.StatusNotFound, "user not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve user")
		return
	}

	Success(w, http.StatusOK, h.toUserResponse(user))
}

// UpdateUser handles PATCH /api/v1/admin/users/{id} - updates an existing user.
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		Error(w, http.StatusBadRequest, "user ID is required")
		return
	}

	// Get existing user
	existingUser, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			Error(w, http.StatusNotFound, "user not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to retrieve user")
		return
	}

	var req updateUserRequest
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

	// Check if email change conflicts with existing users
	if req.Email != nil {
		newEmail := strings.TrimSpace(*req.Email)
		if newEmail != existingUser.Email {
			otherUser, err := h.userRepo.GetByEmail(r.Context(), newEmail)
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				Error(w, http.StatusInternalServerError, "failed to check email availability")
				return
			}
			if otherUser != nil && otherUser.ID != existingUser.ID {
				Error(w, http.StatusConflict, "email already exists")
				return
			}
		}
	}

	// Build updated user (immutability: create new object)
	updatedUser := &model.User{
		ID:           existingUser.ID,
		Email:        existingUser.Email,
		PasswordHash: existingUser.PasswordHash,
		DisplayName:  existingUser.DisplayName,
		IsAdmin:      existingUser.IsAdmin,
		CreatedAt:    existingUser.CreatedAt,
		UpdatedAt:    existingUser.UpdatedAt,
	}

	// Apply updates
	if req.Email != nil {
		updatedUser.Email = strings.TrimSpace(*req.Email)
	}
	if req.DisplayName != nil {
		updatedUser.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.IsAdmin != nil {
		updatedUser.IsAdmin = *req.IsAdmin
	}
	if req.Password != nil {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcryptCost)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to process password")
			return
		}
		updatedUser.PasswordHash = string(passwordHash)
	}

	if err := h.userRepo.Update(r.Context(), updatedUser); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			Error(w, http.StatusNotFound, "user not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	Success(w, http.StatusOK, h.toUserResponse(updatedUser))
}

// DeleteUser handles DELETE /api/v1/admin/users/{id} - deletes a user.
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		Error(w, http.StatusBadRequest, "user ID is required")
		return
	}

	if err := h.userRepo.Delete(r.Context(), userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			Error(w, http.StatusNotFound, "user not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	Success(w, http.StatusOK, nil)
}

// validateCreateRequest validates the create user request fields.
func (h *AdminHandler) validateCreateRequest(req createUserRequest) map[string]string {
	errs := make(map[string]string)

	email := strings.TrimSpace(req.Email)
	if email == "" {
		errs["email"] = "email is required"
	} else if !adminEmailRegex.MatchString(email) {
		errs["email"] = "invalid email format"
	}

	if req.Password == "" {
		errs["password"] = "password is required"
	} else if len(req.Password) < 8 {
		errs["password"] = "password must be at least 8 characters"
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		errs["display_name"] = "display_name is required"
	}

	return errs
}

// validateUpdateRequest validates the update user request fields.
func (h *AdminHandler) validateUpdateRequest(req updateUserRequest) map[string]string {
	errs := make(map[string]string)

	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			errs["email"] = "email cannot be empty"
		} else if !adminEmailRegex.MatchString(email) {
			errs["email"] = "invalid email format"
		}
	}

	if req.Password != nil {
		if len(*req.Password) < 8 {
			errs["password"] = "password must be at least 8 characters"
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

// toUserResponse converts a model.User to adminUserResponse.
func (h *AdminHandler) toUserResponse(user *model.User) adminUserResponse {
	return adminUserResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		CreatedAt:   user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   user.UpdatedAt.Format(time.RFC3339),
	}
}
