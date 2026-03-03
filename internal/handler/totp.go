package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// TOTPServiceInterface defines the interface for TOTP service operations.
type TOTPServiceInterface interface {
	BeginSetup(ctx context.Context, userID string) (secret, qrBase64, provisioningURI string, err error)
	ConfirmSetup(ctx context.Context, userID, code string) ([]string, error)
	Verify(ctx context.Context, userID, code string) error
	VerifyRecoveryCode(ctx context.Context, userID, code string) error
	Disable(ctx context.Context, userID string) error
	RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error)
	GetStatus(ctx context.Context, userID string) (bool, error)
	GeneratePendingToken(userID string, isAdmin bool) (string, error)
	ValidatePendingToken(tokenStr string) (*service.Claims, error)
}

// PasswordVerifier defines the interface for verifying user passwords.
type PasswordVerifier interface {
	VerifyPassword(ctx context.Context, userID, password string) error
}

// UserGetter retrieves user data for authorization checks.
type UserGetter interface {
	GetUser(ctx context.Context, userID string) (*model.User, error)
}

// TOTPHandler handles two-factor authentication HTTP requests.
type TOTPHandler struct {
	totpService      TOTPServiceInterface
	authTokenService AuthTokenServiceInterface
	passwordVerifier PasswordVerifier
	userGetter       UserGetter
	require2FA       bool
}

// NewTOTPHandler creates a new TOTPHandler instance.
func NewTOTPHandler(totpService TOTPServiceInterface, authTokenService AuthTokenServiceInterface, passwordVerifier PasswordVerifier, require2FA bool, userGetter UserGetter) *TOTPHandler {
	return &TOTPHandler{
		totpService:      totpService,
		authTokenService: authTokenService,
		passwordVerifier: passwordVerifier,
		userGetter:       userGetter,
		require2FA:       require2FA,
	}
}

// isOIDCUser checks whether the given user is linked to an OIDC provider.
// Returns false if no UserGetter is configured.
func (h *TOTPHandler) isOIDCUser(ctx context.Context, userID string) (bool, error) {
	if h.userGetter == nil {
		return false, nil
	}
	user, err := h.userGetter.GetUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.OIDCSubject != "", nil
}

// --- Request/Response types for Swagger ---

// totpStatusResponse represents the 2FA status for a user.
type totpStatusResponse struct {
	Enabled    bool `json:"enabled"`
	Require2FA bool `json:"require_2fa"`
}

// totpSetupResponse represents the data returned when beginning 2FA setup.
type totpSetupResponse struct {
	Secret          string `json:"secret"`
	QRCode          string `json:"qr_code"`
	ProvisioningURI string `json:"provisioning_uri"`
}

// totpConfirmRequest represents the request body for confirming 2FA setup.
type totpConfirmRequest struct {
	Code string `json:"code"`
}

// totpConfirmResponse represents the response after confirming 2FA setup.
type totpConfirmResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// totpDisableRequest represents the request body for disabling 2FA.
type totpDisableRequest struct {
	Password string `json:"password"`
}

// totpRegenerateRequest represents the request body for regenerating recovery codes.
type totpRegenerateRequest struct {
	Password string `json:"password"`
}

// totpVerifyRequest represents the request body for verifying a TOTP code during login.
type totpVerifyRequest struct {
	PendingToken string `json:"pending_token"`
	Code         string `json:"code"`
}

// totpRecoveryRequest represents the request body for using a recovery code during login.
type totpRecoveryRequest struct {
	PendingToken string `json:"pending_token"`
	RecoveryCode string `json:"recovery_code"`
}

// --- Authenticated setup endpoints (under /api/v1/me/2fa) ---

// GetStatus handles GET /api/v1/me/2fa/status - returns the user's 2FA status.
//
//	@Summary	Get 2FA status
//	@Tags		2fa
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	APIResponse{data=totpStatusResponse}
//	@Failure	401	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/me/2fa/status [get]
func (h *TOTPHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	enabled, err := h.totpService.GetStatus(r.Context(), userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	Success(w, http.StatusOK, totpStatusResponse{
		Enabled:    enabled,
		Require2FA: h.require2FA,
	})
}

// BeginSetup handles POST /api/v1/me/2fa/setup - initiates 2FA setup.
//
//	@Summary		Begin 2FA setup
//	@Description	Generates a TOTP secret and returns a QR code for scanning with an authenticator app.
//	@Tags			2fa
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse{data=totpSetupResponse}
//	@Failure		401	{object}	APIResponse
//	@Failure		409	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/me/2fa/setup [post]
func (h *TOTPHandler) BeginSetup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if isOIDC, err := h.isOIDCUser(r.Context(), userID); err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	} else if isOIDC {
		Error(w, http.StatusForbidden, "2FA is not available for SSO accounts")
		return
	}

	secret, qrBase64, provisioningURI, err := h.totpService.BeginSetup(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrTOTPAlreadyEnabled) {
			Error(w, http.StatusConflict, "2FA is already enabled")
			return
		}
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	Success(w, http.StatusOK, totpSetupResponse{
		Secret:          secret,
		QRCode:          qrBase64,
		ProvisioningURI: provisioningURI,
	})
}

// ConfirmSetup handles POST /api/v1/me/2fa/confirm - verifies TOTP code and enables 2FA.
//
//	@Summary		Confirm 2FA setup
//	@Description	Verifies a TOTP code from the authenticator app, enables 2FA, and returns recovery codes.
//	@Tags			2fa
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		totpConfirmRequest	true	"TOTP verification code"
//	@Success		200		{object}	APIResponse{data=totpConfirmResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		409		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/me/2fa/confirm [post]
func (h *TOTPHandler) ConfirmSetup(w http.ResponseWriter, r *http.Request) {
	var req totpConfirmRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" {
		ValidationError(w, map[string]string{"code": "code is required"})
		return
	}

	userID := middleware.GetUserID(r.Context())

	if isOIDC, err := h.isOIDCUser(r.Context(), userID); err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	} else if isOIDC {
		Error(w, http.StatusForbidden, "2FA is not available for SSO accounts")
		return
	}

	codes, err := h.totpService.ConfirmSetup(r.Context(), userID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTOTPAlreadyEnabled):
			Error(w, http.StatusConflict, "2FA is already enabled")
		case errors.Is(err, service.ErrTOTPNotSetup):
			Error(w, http.StatusBadRequest, "2FA setup not started; call /me/2fa/setup first")
		case errors.Is(err, service.ErrInvalidTOTPCode):
			Error(w, http.StatusBadRequest, "invalid verification code")
		default:
			Error(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	Success(w, http.StatusOK, totpConfirmResponse{
		RecoveryCodes: codes,
	})
}

// Disable handles POST /api/v1/me/2fa/disable - disables 2FA for the user.
//
//	@Summary		Disable 2FA
//	@Description	Disables two-factor authentication. Requires password confirmation.
//	@Tags			2fa
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		totpDisableRequest	true	"Password confirmation"
//	@Success		200		{object}	APIResponse
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/me/2fa/disable [post]
func (h *TOTPHandler) Disable(w http.ResponseWriter, r *http.Request) {
	var req totpDisableRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Password == "" {
		ValidationError(w, map[string]string{"password": "password is required"})
		return
	}

	userID := middleware.GetUserID(r.Context())

	if isOIDC, err := h.isOIDCUser(r.Context(), userID); err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	} else if isOIDC {
		Error(w, http.StatusForbidden, "2FA is not available for SSO accounts")
		return
	}

	// Verify password before allowing 2FA disable
	if err := h.passwordVerifier.VerifyPassword(r.Context(), userID, req.Password); err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			Error(w, http.StatusUnauthorized, "invalid password")
			return
		}
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.totpService.Disable(r.Context(), userID); err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	Success(w, http.StatusOK, nil)
}

// RegenerateRecoveryCodes handles POST /api/v1/me/2fa/recovery-codes - regenerates recovery codes.
//
//	@Summary		Regenerate recovery codes
//	@Description	Generates new recovery codes, replacing all existing ones. Requires password confirmation.
//	@Tags			2fa
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		totpRegenerateRequest	true	"Password confirmation"
//	@Success		200		{object}	APIResponse{data=totpConfirmResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/me/2fa/recovery-codes [post]
func (h *TOTPHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	var req totpRegenerateRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Password == "" {
		ValidationError(w, map[string]string{"password": "password is required"})
		return
	}

	userID := middleware.GetUserID(r.Context())

	if isOIDC, err := h.isOIDCUser(r.Context(), userID); err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	} else if isOIDC {
		Error(w, http.StatusForbidden, "2FA is not available for SSO accounts")
		return
	}

	// Verify password before allowing recovery code regeneration
	if err := h.passwordVerifier.VerifyPassword(r.Context(), userID, req.Password); err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			Error(w, http.StatusUnauthorized, "invalid password")
			return
		}
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	codes, err := h.totpService.RegenerateRecoveryCodes(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrTOTPNotEnabled) {
			Error(w, http.StatusBadRequest, "2FA is not enabled")
			return
		}
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	Success(w, http.StatusOK, totpConfirmResponse{
		RecoveryCodes: codes,
	})
}

// --- Public verification endpoints (under /api/v1/auth/2fa) ---

// Verify handles POST /api/v1/auth/2fa/verify - validates a TOTP code and completes login.
//
//	@Summary		Verify 2FA code
//	@Description	Validates a TOTP code against a pending 2FA token and returns JWT tokens on success.
//	@Tags			2fa
//	@Accept			json
//	@Produce		json
//	@Param			body	body		totpVerifyRequest	true	"Pending token and TOTP code"
//	@Success		200		{object}	APIResponse{data=loginResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		429		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/auth/2fa/verify [post]
func (h *TOTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req totpVerifyRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fieldErrors := make(map[string]string)
	if req.PendingToken == "" {
		fieldErrors["pending_token"] = "pending_token is required"
	}
	if req.Code == "" {
		fieldErrors["code"] = "code is required"
	}
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	// Validate pending token
	claims, err := h.totpService.ValidatePendingToken(req.PendingToken)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid or expired pending token")
		return
	}

	// Verify TOTP code
	if err := h.totpService.Verify(r.Context(), claims.UserID, req.Code); err != nil {
		if errors.Is(err, service.ErrInvalidTOTPCode) {
			Error(w, http.StatusUnauthorized, "invalid 2FA code")
			return
		}
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Generate real tokens
	tokens, err := h.authTokenService.GenerateTokensForUser(claims.UserID, claims.IsAdmin)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	Success(w, http.StatusOK, loginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         &userResponse{ID: claims.UserID},
	})
}

// Recovery handles POST /api/v1/auth/2fa/recovery - validates a recovery code and completes login.
//
//	@Summary		Verify recovery code
//	@Description	Validates a recovery code against a pending 2FA token and returns JWT tokens on success. The recovery code is consumed (single-use).
//	@Tags			2fa
//	@Accept			json
//	@Produce		json
//	@Param			body	body		totpRecoveryRequest	true	"Pending token and recovery code"
//	@Success		200		{object}	APIResponse{data=loginResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		429		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/auth/2fa/recovery [post]
func (h *TOTPHandler) Recovery(w http.ResponseWriter, r *http.Request) {
	var req totpRecoveryRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fieldErrors := make(map[string]string)
	if req.PendingToken == "" {
		fieldErrors["pending_token"] = "pending_token is required"
	}
	if req.RecoveryCode == "" {
		fieldErrors["recovery_code"] = "recovery_code is required"
	}
	if len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	// Validate pending token
	claims, err := h.totpService.ValidatePendingToken(req.PendingToken)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid or expired pending token")
		return
	}

	// Verify recovery code
	if err := h.totpService.VerifyRecoveryCode(r.Context(), claims.UserID, req.RecoveryCode); err != nil {
		if errors.Is(err, service.ErrInvalidRecoveryCode) {
			Error(w, http.StatusUnauthorized, "invalid recovery code")
			return
		}
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Generate real tokens
	tokens, err := h.authTokenService.GenerateTokensForUser(claims.UserID, claims.IsAdmin)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	Success(w, http.StatusOK, loginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         &userResponse{ID: claims.UserID},
	})
}
