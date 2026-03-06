package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/amalgamated-tools/enlace/internal/middleware"
)

const (
	oidcStateCookie     = "oidc_state"
	oidcVerifierCookie  = "oidc_verifier"
	oidcLinkCookie      = "oidc_link"
	oidcPendingCookie   = "oidc_pending"
	stateCookieMaxAge   = 10 * 60 // 10 minutes
	pendingCookieMaxAge = 2 * 60  // 2 minutes
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
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// OIDCHandler handles OIDC authentication requests.
type OIDCHandler struct {
	oidcService   OIDCServiceInterface
	authService   AuthTokenServiceInterface
	baseURL       string
	secureCookies bool
	cookieSecret  []byte
}

// NewOIDCHandler creates a new OIDCHandler instance.
func NewOIDCHandler(oidcService OIDCServiceInterface, authService AuthTokenServiceInterface, baseURL string, cookieSecret []byte) *OIDCHandler {
	return &OIDCHandler{
		oidcService:   oidcService,
		authService:   authService,
		baseURL:       baseURL,
		secureCookies: strings.HasPrefix(baseURL, "https://"),
		cookieSecret:  cookieSecret,
	}
}

// signCookieValue produces "payload.hmacHex" from a raw payload string.
func (h *OIDCHandler) signCookieValue(payload string) string {
	mac := hmac.New(sha256.New, append([]byte("oidc-pending-cookie:"), h.cookieSecret...))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// verifyCookieValue splits "payload.hmacHex", verifies the HMAC, and returns the payload.
func (h *OIDCHandler) verifyCookieValue(signed string) (string, error) {
	idx := strings.LastIndex(signed, ".")
	if idx < 0 {
		return "", fmt.Errorf("malformed signed cookie")
	}
	payload := signed[:idx]
	sigHex := signed[idx+1:]

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return "", fmt.Errorf("invalid signature encoding")
	}

	mac := hmac.New(sha256.New, append([]byte("oidc-pending-cookie:"), h.cookieSecret...))
	mac.Write([]byte(payload))
	expected := mac.Sum(nil)

	if !hmac.Equal(sigBytes, expected) {
		return "", fmt.Errorf("invalid cookie signature")
	}
	return payload, nil
}

// oidcConfigResponse represents the OIDC configuration response.
type oidcConfigResponse struct {
	Enabled bool `json:"enabled"`
}

// Config returns OIDC configuration for the frontend.
//
//	@Summary		Get OIDC configuration
//	@Description	Returns whether OIDC/SSO login is enabled for this Enlace instance.
//	@Tags			oidc
//	@Produce		json
//	@Success		200	{object}	APIResponse{data=oidcConfigResponse}
//	@Router			/api/v1/auth/oidc/config [get]
func (h *OIDCHandler) Config(w http.ResponseWriter, r *http.Request) {
	enabled := h.oidcService != nil && h.oidcService.IsEnabled()
	Success(w, http.StatusOK, oidcConfigResponse{Enabled: enabled})
}

// Login initiates the OIDC login flow.
//
//	@Summary		Start OIDC login
//	@Description	Initiates the OIDC authorization code flow with PKCE. Redirects the browser to the configured identity provider.
//	@Tags			oidc
//	@Success		302	{string}	string	"Redirects to OIDC provider"
//	@Failure		404	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/auth/oidc/login [get]
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
		Secure:   h.secureCookies || r.TLS != nil,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcVerifierCookie,
		Value:    codeVerifier,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || r.TLS != nil,
	})

	authURL := h.oidcService.GetAuthURL(state, codeVerifier)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles the OIDC provider callback.
//
//	@Summary		OIDC callback
//	@Description	Handles the OAuth 2.0 authorization code callback. Verifies state, exchanges the code for tokens, and redirects to the frontend with JWT tokens or an error fragment.
//	@Tags			oidc
//	@Param			code	query		string	true	"Authorization code"
//	@Param			state	query		string	true	"State parameter"
//	@Success		302		{string}	string	"Redirects to frontend with tokens"
//	@Failure		302		{string}	string	"Redirects to frontend with error"
//	@Router			/api/v1/auth/oidc/callback [get]
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
			SameSite: http.SameSiteLaxMode,
			Secure:   h.secureCookies || r.TLS != nil,
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

	// Store tokens in a short-lived HttpOnly cookie for secure one-time exchange.
	// This avoids passing sensitive tokens via query parameters (browser history,
	// referrer headers, server logs).
	pending, err := json.Marshal(tokens)
	if err != nil {
		h.redirectWithError(w, r, "failed to generate tokens")
		return
	}
	encodedPending := base64.StdEncoding.EncodeToString(pending)
	http.SetCookie(w, &http.Cookie{
		Name:     oidcPendingCookie,
		Value:    h.signCookieValue(encodedPending),
		Path:     "/",
		MaxAge:   pendingCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || r.TLS != nil,
	})
	http.Redirect(w, r, h.baseURL+"/#/auth/callback", http.StatusFound)
}

// ExchangeOIDCTokens handles POST /api/v1/auth/oidc/exchange - exchanges the pending
// HttpOnly cookie set during the OIDC callback for the actual JWT token pair.
//
// Security note: the response body contains JWTs readable by JavaScript. A same-origin
// XSS vulnerability could therefore steal tokens from this endpoint. The risk is mitigated
// by the single-use HMAC-signed cookie (2-minute TTL) that gates access — an attacker
// would need to trigger the full OIDC redirect flow to obtain a fresh cookie before the
// exchange can succeed. This is an accepted trade-off; the frontend SPA must receive the
// tokens via JavaScript to store them for subsequent API calls.
//
//	@Summary		Exchange OIDC pending token
//	@Description	Exchanges the short-lived HttpOnly pending-token cookie (set during OIDC callback) for the JWT access and refresh token pair. The cookie is consumed on first use.
//	@Tags			oidc
//	@Produce		json
//	@Success		200	{object}	APIResponse{data=TokenPair}
//	@Failure		401	{object}	APIResponse
//	@Router			/api/v1/auth/oidc/exchange [post]
func (h *OIDCHandler) ExchangeOIDCTokens(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		Error(w, http.StatusNotFound, "OIDC is not enabled")
		return
	}

	cookie, err := r.Cookie(oidcPendingCookie)
	if err != nil {
		Error(w, http.StatusUnauthorized, "no pending token")
		return
	}

	// Clear the cookie immediately — it is single-use.
	http.SetCookie(w, &http.Cookie{
		Name:     oidcPendingCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || r.TLS != nil,
	})

	// Verify the HMAC signature to ensure the cookie was set by this server.
	payload, err := h.verifyCookieValue(cookie.Value)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid pending token")
		return
	}

	var tokens TokenPair
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid pending token")
		return
	}
	if err := json.Unmarshal(decoded, &tokens); err != nil {
		Error(w, http.StatusUnauthorized, "invalid pending token")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	Success(w, http.StatusOK, tokens)
}

// Link initiates the OIDC account linking flow.
//
//	@Summary		Start OIDC account linking
//	@Description	Initiates the OIDC authorization code flow to link an external identity to the current user account.
//	@Tags			oidc
//	@Security		BearerAuth
//	@Success		302	{string}	string	"Redirects to OIDC provider"
//	@Failure		401	{object}	APIResponse
//	@Failure		404	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/me/oidc/link [get]
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
		Secure:   h.secureCookies || r.TLS != nil,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcVerifierCookie,
		Value:    codeVerifier,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || r.TLS != nil,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcLinkCookie,
		Value:    userID,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || r.TLS != nil,
	})

	authURL := h.oidcService.GetLinkAuthURL(state, codeVerifier)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// LinkCallback handles the OIDC account linking callback.
//
//	@Summary		OIDC link callback
//	@Description	Handles the OAuth 2.0 callback for account linking. Verifies state, exchanges the code, and links the OIDC identity to the current user account.
//	@Tags			oidc
//	@Param			code	query		string	true	"Authorization code"
//	@Param			state	query		string	true	"State parameter"
//	@Success		302		{string}	string	"Redirects to settings"
//	@Failure		302		{string}	string	"Redirects with error"
//	@Router			/api/v1/me/oidc/callback [get]
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
			SameSite: http.SameSiteLaxMode,
			Secure:   h.secureCookies || r.TLS != nil,
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
//	@Summary		Unlink OIDC account
//	@Description	Removes the OIDC identity link from the current user account. Requires that the account has a local password set.
//	@Tags			oidc
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse
//	@Failure		400	{object}	APIResponse
//	@Failure		401	{object}	APIResponse
//	@Failure		404	{object}	APIResponse
//	@Router			/api/v1/me/oidc [delete]
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
