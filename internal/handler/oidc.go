package handler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/amalgamated-tools/enlace/internal/middleware"
)

const (
	oidcStateCookie    = "oidc_state"
	oidcVerifierCookie = "oidc_verifier"
	oidcLinkCookie     = "oidc_link"
	stateCookieMaxAge  = 10 * 60 // 10 minutes
)

// OIDCUserInfo contains user information from the OIDC provider.
// This mirrors service.OIDCUserInfo but is exposed for handler interface.
type OIDCUserInfo struct {
	Subject     string
	Email       string
	DisplayName string
	Issuer      string
}

// OIDCUser represents a user for OIDC operations.
// This is a minimal struct for the handler interface.
type OIDCUser struct {
	ID      string
	IsAdmin bool
}

// OIDCServiceInterface defines the interface for OIDC service operations.
// Using an interface allows for easier testing with mocks.
type OIDCServiceInterface interface {
	IsEnabled() bool
	GenerateState() (string, error)
	GenerateCodeVerifier() (string, error)
	GetAuthURL(state, codeVerifier string) string
	GetLinkAuthURL(state, codeVerifier string) string
	ExchangeCode(ctx context.Context, code, codeVerifier string) (*OIDCUserInfo, error)
	FindOrCreateUser(ctx context.Context, info *OIDCUserInfo) (*OIDCUser, error)
	LinkOIDC(ctx context.Context, userID string, info *OIDCUserInfo) error
	UnlinkOIDC(ctx context.Context, userID string) error
}

// AuthTokenServiceInterface defines the interface for generating tokens.
// This is a subset of AuthService operations needed for OIDC.
type AuthTokenServiceInterface interface {
	GenerateTokensForUser(userID string, isAdmin bool) (*TokenPair, error)
}

// TokenPair represents an access and refresh token pair.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// OIDCHandler handles OIDC authentication requests.
type OIDCHandler struct {
	oidcService OIDCServiceInterface
	authService AuthTokenServiceInterface
	baseURL     string
}

// NewOIDCHandler creates a new OIDCHandler instance.
func NewOIDCHandler(oidcService OIDCServiceInterface, authService AuthTokenServiceInterface, baseURL string) *OIDCHandler {
	return &OIDCHandler{
		oidcService: oidcService,
		authService: authService,
		baseURL:     baseURL,
	}
}

// oidcConfigResponse represents the OIDC configuration response.
type oidcConfigResponse struct {
	Enabled bool `json:"enabled"`
}

// Config returns OIDC configuration for the frontend.
//
//	@Summary	Get OIDC configuration
//	@Tags		oidc
//	@Produce	json
//	@Success	200	{object}	APIResponse{data=oidcConfigResponse}
//	@Router		/api/v1/auth/oidc/config [get]
func (h *OIDCHandler) Config(w http.ResponseWriter, r *http.Request) {
	enabled := h.oidcService != nil && h.oidcService.IsEnabled()
	Success(w, http.StatusOK, oidcConfigResponse{Enabled: enabled})
}

// Login initiates the OIDC login flow.
//
//	@Summary	Start OIDC login
//	@Tags		oidc
//	@Success	302	{string}	string	"Redirects to OIDC provider"
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/auth/oidc/login [get]
func (h *OIDCHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		Error(w, http.StatusNotFound, "OIDC is not enabled")
		return
	}

	state, err := h.oidcService.GenerateState()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	codeVerifier, err := h.oidcService.GenerateCodeVerifier()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate code verifier")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcVerifierCookie,
		Value:    codeVerifier,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	authURL := h.oidcService.GetAuthURL(state, codeVerifier)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles the OIDC provider callback.
//
//	@Summary	OIDC callback
//	@Tags		oidc
//	@Param		code	query		string	true	"Authorization code"
//	@Param		state	query		string	true	"State parameter"
//	@Success	302		{string}	string	"Redirects to frontend with tokens"
//	@Failure	302		{string}	string	"Redirects to frontend with error"
//	@Router		/api/v1/auth/oidc/callback [get]
func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		h.redirectWithError(w, r, "OIDC is not enabled")
		return
	}

	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil {
		h.redirectWithError(w, r, "missing state cookie")
		return
	}

	state := r.URL.Query().Get("state")
	if state != stateCookie.Value {
		h.redirectWithError(w, r, "state mismatch")
		return
	}

	verifierCookie, err := r.Cookie(oidcVerifierCookie)
	if err != nil {
		h.redirectWithError(w, r, "missing code verifier cookie")
		return
	}

	// Clear cookies
	for _, name := range []string{oidcStateCookie, oidcVerifierCookie} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
	}

	// Check for error from provider
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		desc := r.URL.Query().Get("error_description")
		h.redirectWithError(w, r, errMsg+": "+desc)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.redirectWithError(w, r, "missing authorization code")
		return
	}

	userInfo, err := h.oidcService.ExchangeCode(r.Context(), code, verifierCookie.Value)
	if err != nil {
		h.redirectWithError(w, r, "failed to exchange code: "+err.Error())
		return
	}

	user, err := h.oidcService.FindOrCreateUser(r.Context(), userInfo)
	if err != nil {
		h.redirectWithError(w, r, "failed to create user: "+err.Error())
		return
	}

	tokens, err := h.authService.GenerateTokensForUser(user.ID, user.IsAdmin)
	if err != nil {
		h.redirectWithError(w, r, "failed to generate tokens")
		return
	}

	redirectURL := h.baseURL + "/#/auth/callback?token=" + url.QueryEscape(tokens.AccessToken) +
		"&refresh=" + url.QueryEscape(tokens.RefreshToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// Link initiates the OIDC account linking flow.
//
//	@Summary	Start OIDC account linking
//	@Tags		oidc
//	@Security	BearerAuth
//	@Success	302	{string}	string	"Redirects to OIDC provider"
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Failure	500	{object}	APIResponse
//	@Router		/api/v1/me/oidc/link [get]
func (h *OIDCHandler) Link(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		Error(w, http.StatusNotFound, "OIDC is not enabled")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	state, err := h.oidcService.GenerateState()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	codeVerifier, err := h.oidcService.GenerateCodeVerifier()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate code verifier")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcVerifierCookie,
		Value:    codeVerifier,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcLinkCookie,
		Value:    userID,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	authURL := h.oidcService.GetLinkAuthURL(state, codeVerifier)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// LinkCallback handles the OIDC account linking callback.
//
//	@Summary	OIDC link callback
//	@Tags		oidc
//	@Param		code	query		string	true	"Authorization code"
//	@Param		state	query		string	true	"State parameter"
//	@Success	302		{string}	string	"Redirects to settings"
//	@Failure	302		{string}	string	"Redirects with error"
//	@Router		/api/v1/me/oidc/callback [get]
func (h *OIDCHandler) LinkCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		h.redirectWithError(w, r, "OIDC is not enabled")
		return
	}

	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil {
		h.redirectWithError(w, r, "missing state cookie")
		return
	}

	state := r.URL.Query().Get("state")
	if state != stateCookie.Value {
		h.redirectWithError(w, r, "state mismatch")
		return
	}

	verifierCookie, err := r.Cookie(oidcVerifierCookie)
	if err != nil {
		h.redirectWithError(w, r, "missing code verifier cookie")
		return
	}

	linkCookie, err := r.Cookie(oidcLinkCookie)
	if err != nil {
		h.redirectWithError(w, r, "missing link cookie")
		return
	}
	userID := linkCookie.Value

	// Clear cookies
	for _, name := range []string{oidcStateCookie, oidcVerifierCookie, oidcLinkCookie} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.redirectWithError(w, r, "missing authorization code")
		return
	}

	userInfo, err := h.oidcService.ExchangeCode(r.Context(), code, verifierCookie.Value)
	if err != nil {
		h.redirectWithError(w, r, "failed to exchange code: "+err.Error())
		return
	}

	if err := h.oidcService.LinkOIDC(r.Context(), userID, userInfo); err != nil {
		h.redirectWithError(w, r, "failed to link account: "+err.Error())
		return
	}

	http.Redirect(w, r, h.baseURL+"/#/settings?oidc=linked", http.StatusFound)
}

// Unlink removes OIDC from the user's account.
//
//	@Summary	Unlink OIDC account
//	@Tags		oidc
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	APIResponse
//	@Failure	400	{object}	APIResponse
//	@Failure	401	{object}	APIResponse
//	@Failure	404	{object}	APIResponse
//	@Router		/api/v1/me/oidc [delete]
func (h *OIDCHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		Error(w, http.StatusNotFound, "OIDC is not enabled")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.oidcService.UnlinkOIDC(r.Context(), userID); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	Success(w, http.StatusOK, nil)
}

// redirectWithError redirects to the frontend with an error message.
func (h *OIDCHandler) redirectWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	redirectURL := h.baseURL + "/#/login?error=" + url.QueryEscape(errMsg)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
