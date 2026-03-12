package handler

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// AuthServiceInterface defines the interface for auth service operations.
// Using an interface allows for easier testing with mocks.
type AuthServiceInterface interface {
	Register(ctx context.Context, email, password, displayName string) (*model.User, error)
	Login(ctx context.Context, email, password string) (*service.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*service.TokenPair, error)
	ValidateToken(token string) (*service.Claims, error)
	GetUser(ctx context.Context, userID string) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
}

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	authService AuthServiceInterface
	totpService TOTPServiceInterface
	require2FA  bool
}

// NewAuthHandler creates a new AuthHandler instance.
func NewAuthHandler(authService AuthServiceInterface, totpService TOTPServiceInterface, require2FA bool) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		totpService: totpService,
		require2FA:  require2FA,
	}
}

// registerRequest represents the request body for user registration.
type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// loginRequest represents the request body for user login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest represents the request body for token refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// userResponse represents the user data in API responses (without sensitive fields).
type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// loginResponse represents the response data for successful login.
type loginResponse struct {
	AccessToken      string        `json:"access_token,omitempty"`
	RefreshToken     string        `json:"refresh_token,omitempty"`
	User             *userResponse `json:"user,omitempty"`
	Requires2FA      bool          `json:"requires_2fa,omitempty"`
	Requires2FASetup bool          `json:"requires_2fa_setup,omitempty"`
	PendingToken     string        `json:"pending_token,omitempty"`
}

// tokenResponse represents the response data for token refresh.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Email validation regex pattern.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Register handles user registration requests.
//
//	@Summary		Register a new user
//	@Description	Creates a new user account. Email must be a valid address format and password must be at least 8 characters.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerRequest	true	"Registration details"
//	@Success		201		{object}	APIResponse{data=userResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		409		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	fieldErrors := h.validateRegisterRequest(req)
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	// Register user
	user, err := h.authService.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Return success response
	Success(w, http.StatusCreated, userResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
	})
}

// Login handles user login requests.
//
//	@Summary		Login
//	@Description	Authenticates a user and returns JWT tokens. When 2FA is enabled, returns a pending_token and requires_2fa flag instead; use /auth/2fa/verify to complete login.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		loginRequest	true	"Login credentials"
//	@Success		200		{object}	APIResponse{data=loginResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	fieldErrors := h.validateLoginRequest(req)
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	// Authenticate user (verify password)
	tokens, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Get user details (required to check 2FA status)
	user, err := h.authService.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Check 2FA status (skip for OIDC-linked users — their IdP handles MFA)
	if h.totpService != nil && user.OIDCSubject == "" {
		has2FA, err := h.totpService.GetStatus(r.Context(), user.ID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if has2FA {
			// 2FA is enabled - return pending token instead of real tokens
			pendingToken, err := h.totpService.GeneratePendingToken(user.ID, user.IsAdmin)
			if err != nil {
				Error(w, http.StatusInternalServerError, "internal server error")
				return
			}
			Success(w, http.StatusOK, loginResponse{
				Requires2FA:  true,
				PendingToken: pendingToken,
			})
			return
		}

		// Check admin enforcement
		if h.require2FA {
			pendingToken, err := h.totpService.GeneratePendingToken(user.ID, user.IsAdmin)
			if err != nil {
				Error(w, http.StatusInternalServerError, "internal server error")
				return
			}
			Success(w, http.StatusOK, loginResponse{
				User: &userResponse{
					ID:          user.ID,
					Email:       user.Email,
					DisplayName: user.DisplayName,
				},
				Requires2FASetup: true,
				PendingToken:     pendingToken,
			})
			return
		}
	}

	Success(w, http.StatusOK, loginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User: &userResponse{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}

// Refresh handles token refresh requests.
//
//	@Summary		Refresh tokens
//	@Description	Exchanges a valid refresh token for a new access token and refresh token pair.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		refreshRequest	true	"Refresh token"
//	@Success		200		{object}	APIResponse{data=tokenResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	fieldErrors := h.validateRefreshRequest(req)
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	claims, err := h.authService.ValidateToken(req.RefreshToken)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	user, err := h.authService.GetUser(r.Context(), claims.UserID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	if h.totpService != nil && user.OIDCSubject == "" {
		has2FA, err := h.totpService.GetStatus(r.Context(), user.ID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if h.require2FA && !has2FA {
			Error(w, http.StatusForbidden, "2FA setup required")
			return
		}
		if has2FA && !claims.TFAVerified {
			Error(w, http.StatusUnauthorized, "2FA verification required")
			return
		}
	}

	// Refresh tokens
	tokens, err := h.authService.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	Success(w, http.StatusOK, tokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Logout handles user logout requests. JWT logout is client-side.
//
//	@Summary		Logout
//	@Description	Invalidates the current session. Since Enlace uses stateless JWTs, this endpoint always succeeds and the client is responsible for discarding the tokens.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	APIResponse
//	@Router			/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// JWT logout is client-side, just return success
	Success(w, http.StatusOK, nil)
}

// validateRegisterRequest validates the registration request fields.
func (h *AuthHandler) validateRegisterRequest(req registerRequest) map[string]string {
	errs := make(map[string]string)

	email := strings.TrimSpace(req.Email)
	if email == "" {
		errs["email"] = "email is required"
	} else if !emailRegex.MatchString(email) {
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

// validateLoginRequest validates the login request fields.
func (h *AuthHandler) validateLoginRequest(req loginRequest) map[string]string {
	errs := make(map[string]string)

	email := strings.TrimSpace(req.Email)
	if email == "" {
		errs["email"] = "email is required"
	} else if !emailRegex.MatchString(email) {
		errs["email"] = "invalid email format"
	}

	if req.Password == "" {
		errs["password"] = "password is required"
	}

	return errs
}

// validateRefreshRequest validates the refresh request fields.
func (h *AuthHandler) validateRefreshRequest(req refreshRequest) map[string]string {
	errs := make(map[string]string)

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		errs["refresh_token"] = "refresh_token is required"
	}

	return errs
}

// handleServiceError maps service errors to HTTP responses.
func (h *AuthHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrEmailExists):
		Error(w, http.StatusConflict, "email already exists")
	case errors.Is(err, service.ErrInvalidCredentials):
		Error(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, service.ErrInvalidToken):
		Error(w, http.StatusUnauthorized, "invalid or expired token")
	default:
		Error(w, http.StatusInternalServerError, "internal server error")
	}
}
