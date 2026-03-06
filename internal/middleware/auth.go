package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/amalgamated-tools/enlace/internal/service"
)

type contextKey string

const (
	UserIDKey   contextKey = "userID"
	IsAdminKey  contextKey = "isAdmin"
	ScopesKey   contextKey = "scopes"
	AuthTypeKey contextKey = "authType"
)

// jsonError writes a JSON error response with the correct Content-Type header.
func jsonError(w http.ResponseWriter, body string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}

// APIKeyAuthenticator verifies API keys and returns principal details.
type APIKeyAuthenticator interface {
	Authenticate(ctx context.Context, token string) (*service.APIKeyIdentity, error)
}

type requireAuthConfig struct {
	apiKeyAuth APIKeyAuthenticator
}

// RequireAuthOption configures RequireAuth middleware behavior.
type RequireAuthOption func(*requireAuthConfig)

// WithAPIKeyAuth enables API key authentication fallback.
func WithAPIKeyAuth(auth APIKeyAuthenticator) RequireAuthOption {
	return func(cfg *requireAuthConfig) {
		cfg.apiKeyAuth = auth
	}
}

// RequireAuth validates JWT (and optionally API keys) from Authorization header.
func RequireAuth(authService *service.AuthService, opts ...RequireAuthOption) func(http.Handler) http.Handler {
	cfg := &requireAuthConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from "Bearer <token>"
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				jsonError(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				jsonError(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// API key path.
			if cfg.apiKeyAuth != nil && strings.HasPrefix(token, service.APIKeyTokenPrefix+"_") {
				identity, err := cfg.apiKeyAuth.Authenticate(r.Context(), token)
				if err != nil {
					jsonError(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
					return
				}

				ctx := context.WithValue(r.Context(), UserIDKey, identity.UserID)
				ctx = context.WithValue(ctx, IsAdminKey, false)
				ctx = context.WithValue(ctx, ScopesKey, append([]string(nil), identity.Scopes...))
				ctx = context.WithValue(ctx, AuthTypeKey, "api_key")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			claims, err := authService.ValidateToken(token)
			if err != nil {
				jsonError(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			// Reject pending 2FA tokens
			if claims.TFA {
				jsonError(w, `{"error":"2FA verification required"}`, http.StatusUnauthorized)
				return
			}

			// Require access tokens; reject refresh or unknown token types
			if claims.TokenType != service.TokenTypeAccess {
				jsonError(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, IsAdminKey, claims.IsAdmin)
			ctx = context.WithValue(ctx, AuthTypeKey, "jwt")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks if user is admin (must be used after RequireAuth).
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAdmin, ok := r.Context().Value(IsAdminKey).(bool)
		if !ok || !isAdmin {
			jsonError(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserID retrieves the user ID from the request context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// GetIsAdmin retrieves the admin status from the request context.
func GetIsAdmin(ctx context.Context) bool {
	if v, ok := ctx.Value(IsAdminKey).(bool); ok {
		return v
	}
	return false
}

// GetScopes retrieves API key scopes from the request context.
func GetScopes(ctx context.Context) []string {
	if v, ok := ctx.Value(ScopesKey).([]string); ok {
		return append([]string(nil), v...)
	}
	return nil
}

// GetAuthType retrieves the request authentication type ("jwt" or "api_key").
func GetAuthType(ctx context.Context) string {
	if v, ok := ctx.Value(AuthTypeKey).(string); ok {
		return v
	}
	return ""
}

// RequireScope enforces scope presence for API-key-authenticated requests.
// JWT-authenticated requests are not scope-gated by this middleware.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if GetAuthType(r.Context()) != "api_key" {
				next.ServeHTTP(w, r)
				return
			}
			if slices.Contains(GetScopes(r.Context()), scope) {
				next.ServeHTTP(w, r)
				return
			}
			jsonError(w, `{"error":"insufficient scope"}`, http.StatusForbidden)
		})
	}
}
