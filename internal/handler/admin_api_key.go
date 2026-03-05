package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// APIKeyServiceInterface defines API key service behavior needed by handlers.
type APIKeyServiceInterface interface {
	Create(ctx context.Context, creatorID, name string, scopes []string) (*model.APIKey, string, error)
	ListByCreator(ctx context.Context, creatorID string) ([]*model.APIKey, error)
	Revoke(ctx context.Context, id string) error
}

// APIKeyHandler manages admin API key routes.
type APIKeyHandler struct {
	service APIKeyServiceInterface
}

// NewAPIKeyHandler creates an APIKeyHandler.
func NewAPIKeyHandler(svc APIKeyServiceInterface) *APIKeyHandler {
	return &APIKeyHandler{service: svc}
}

type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type apiKeyResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	KeyPrefix  string   `json:"key_prefix"`
	Scopes     []string `json:"scopes"`
	RevokedAt  *string  `json:"revoked_at,omitempty"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

type createAPIKeyResponse struct {
	apiKeyResponse
	Key string `json:"key"`
}

// List handles GET /api/v1/admin/api-keys.
func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	keys, err := h.service.ListByCreator(r.Context(), userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}

	resp := make([]apiKeyResponse, 0, len(keys))
	for _, key := range keys {
		resp = append(resp, toAPIKeyResponse(key))
	}
	Success(w, http.StatusOK, resp)
}

// Create handles POST /api/v1/admin/api-keys.
func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createAPIKeyRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	if len(req.Scopes) == 0 {
		ValidationError(w, map[string]string{"scopes": "at least one scope is required"})
		return
	}

	key, token, err := h.service.Create(r.Context(), userID, req.Name, req.Scopes)
	if err != nil {
		if errors.Is(err, service.ErrInvalidScopeSet) {
			ValidationError(w, map[string]string{"scopes": "contains unsupported scope"})
			return
		}
		Error(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	resp := createAPIKeyResponse{
		apiKeyResponse: toAPIKeyResponse(key),
		Key:            token,
	}
	Success(w, http.StatusCreated, resp)
}

// Revoke handles DELETE /api/v1/admin/api-keys/{id}.
func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		Error(w, http.StatusBadRequest, "api key id is required")
		return
	}

	if err := h.service.Revoke(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			Error(w, http.StatusNotFound, "api key not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}

	Success(w, http.StatusOK, nil)
}

func toAPIKeyResponse(key *model.APIKey) apiKeyResponse {
	resp := apiKeyResponse{
		ID:        key.ID,
		Name:      key.Name,
		KeyPrefix: key.KeyPrefix,
		Scopes:    append([]string(nil), key.Scopes...),
		CreatedAt: key.CreatedAt.Format(time.RFC3339),
	}
	if key.RevokedAt != nil {
		v := key.RevokedAt.Format(time.RFC3339)
		resp.RevokedAt = &v
	}
	if key.LastUsedAt != nil {
		v := key.LastUsedAt.Format(time.RFC3339)
		resp.LastUsedAt = &v
	}
	return resp
}
