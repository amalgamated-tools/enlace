package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/service"
)

// WebhookServiceInterface defines webhook operations needed by handlers.
type WebhookServiceInterface interface {
	CreateSubscription(ctx context.Context, creatorID string, input service.WebhookSubscriptionCreateInput) (*model.WebhookSubscription, string, error)
	ListSubscriptionsByCreator(ctx context.Context, creatorID string) ([]*model.WebhookSubscription, error)
	UpdateSubscription(ctx context.Context, creatorID, id string, input service.WebhookSubscriptionUpdateInput) (*model.WebhookSubscription, error)
	DeleteSubscription(ctx context.Context, creatorID, id string) error
	ListDeliveries(ctx context.Context, creatorID string, input service.WebhookDeliveryListInput) ([]*model.WebhookDelivery, error)
}

// WebhookAdminHandler handles webhook admin routes.
type WebhookAdminHandler struct {
	service WebhookServiceInterface
}

// NewWebhookAdminHandler creates a WebhookAdminHandler.
func NewWebhookAdminHandler(svc WebhookServiceInterface) *WebhookAdminHandler {
	return &WebhookAdminHandler{service: svc}
}

type createWebhookRequest struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type updateWebhookRequest struct {
	Name    *string  `json:"name"`
	URL     *string  `json:"url"`
	Events  []string `json:"events"`
	Enabled *bool    `json:"enabled"`
}

type webhookResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Enabled   bool     `json:"enabled"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

type createWebhookResponse struct {
	webhookResponse
	Secret string `json:"secret"`
}

type webhookDeliveryResponse struct {
	ID             string  `json:"id"`
	SubscriptionID string  `json:"subscription_id"`
	EventType      string  `json:"event_type"`
	EventID        string  `json:"event_id"`
	IdempotencyKey string  `json:"idempotency_key"`
	Attempt        int     `json:"attempt"`
	Status         string  `json:"status"`
	StatusCode     *int    `json:"status_code,omitempty"`
	NextAttemptAt  *string `json:"next_attempt_at,omitempty"`
	DeliveredAt    *string `json:"delivered_at,omitempty"`
	Error          string  `json:"error,omitempty"`
	RequestBody    string  `json:"request_body,omitempty"`
	DurationMS     int64   `json:"duration_ms"`
	CreatedAt      string  `json:"created_at"`
}

// ListEvents handles GET /api/v1/admin/webhooks/events.
//
//	@Summary		List allowed webhook event types
//	@Description	Returns the list of event types that can be subscribed to. Requires admin role.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse{data=[]string}
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Router			/api/v1/admin/webhooks/events [get]
func (h *WebhookAdminHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	Success(w, http.StatusOK, service.AllowedWebhookEvents())
}

// ListSubscriptions handles GET /api/v1/admin/webhooks.
//
//	@Summary		List webhook subscriptions
//	@Description	Returns all webhook subscriptions created by the authenticated admin. Requires admin role.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse{data=[]webhookResponse}
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/webhooks [get]
func (h *WebhookAdminHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	items, err := h.service.ListSubscriptionsByCreator(r.Context(), userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	resp := make([]webhookResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toWebhookResponse(item))
	}
	Success(w, http.StatusOK, resp)
}

// CreateSubscription handles POST /api/v1/admin/webhooks.
//
//	@Summary		Create webhook subscription
//	@Description	Creates a webhook subscription. The secret is returned only once at creation and is used to verify webhook signatures. Requires admin role.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		createWebhookRequest	true	"Webhook subscription details"
//	@Success		201		{object}	APIResponse{data=createWebhookResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/admin/webhooks [post]
func (h *WebhookAdminHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createWebhookRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		ValidationError(w, map[string]string{"url": "url is required"})
		return
	}

	sub, secret, err := h.service.CreateSubscription(r.Context(), userID, service.WebhookSubscriptionCreateInput{
		Name:   req.Name,
		URL:    req.URL,
		Events: req.Events,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidWebhookURL):
			ValidationError(w, map[string]string{"url": "url must be a valid https URL and not point to localhost or loopback addresses"})
		case errors.Is(err, service.ErrInvalidWebhookEvents):
			ValidationError(w, map[string]string{"events": "contains unsupported event"})
		default:
			Error(w, http.StatusInternalServerError, "failed to create webhook")
		}
		return
	}

	Success(w, http.StatusCreated, createWebhookResponse{
		webhookResponse: toWebhookResponse(sub),
		Secret:          secret,
	})
}

// UpdateSubscription handles PATCH /api/v1/admin/webhooks/{id}.
//
//	@Summary		Update webhook subscription
//	@Description	Updates an existing webhook subscription. All fields are optional; omitted fields are left unchanged. Requires admin role.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string					true	"Webhook subscription ID"
//	@Param			body	body		updateWebhookRequest	true	"Fields to update"
//	@Success		200		{object}	APIResponse{data=webhookResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		404		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/admin/webhooks/{id} [patch]
func (h *WebhookAdminHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		Error(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	var req updateWebhookRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := service.WebhookSubscriptionUpdateInput{
		Name:    req.Name,
		URL:     req.URL,
		Events:  req.Events,
		Enabled: req.Enabled,
	}
	sub, err := h.service.UpdateSubscription(r.Context(), userID, id, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWebhookNotFound):
			Error(w, http.StatusNotFound, "webhook not found")
		case errors.Is(err, service.ErrInvalidWebhookURL):
			ValidationError(w, map[string]string{"url": "url must be https"})
		case errors.Is(err, service.ErrInvalidWebhookEvents):
			ValidationError(w, map[string]string{"events": "contains unsupported event"})
		case errors.Is(err, service.ErrInvalidWebhookName):
			ValidationError(w, map[string]string{"name": "name must not be empty"})
		default:
			Error(w, http.StatusInternalServerError, "failed to update webhook")
		}
		return
	}

	Success(w, http.StatusOK, toWebhookResponse(sub))
}

// DeleteSubscription handles DELETE /api/v1/admin/webhooks/{id}.
//
//	@Summary		Delete webhook subscription
//	@Description	Deletes a webhook subscription. Pending deliveries for this subscription will not be retried. Returns HTTP 404 if the subscription does not exist or belongs to a different admin. Requires admin role.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Webhook subscription ID"
//	@Success		200	{object}	APIResponse
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		404	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/webhooks/{id} [delete]
func (h *WebhookAdminHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		Error(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	if err := h.service.DeleteSubscription(r.Context(), userID, id); err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		Error(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}
	Success(w, http.StatusOK, nil)
}

// ListDeliveries handles GET /api/v1/admin/webhooks/deliveries.
//
//	@Summary		List webhook delivery attempts
//	@Description	Lists recent webhook delivery attempts for the authenticated admin's subscriptions. Accepts optional query parameters: subscription_id, status, event_type, and limit (default 100, max 500). Requires admin role.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Param			subscription_id	query		string	false	"Filter by subscription ID"
//	@Param			status			query		string	false	"Filter by status (e.g. delivered, failed, pending)"
//	@Param			event_type		query		string	false	"Filter by event type (e.g. share.created)"
//	@Param			limit			query		int		false	"Maximum number of results (1–500, default 100)"
//	@Success		200				{object}	APIResponse{data=[]webhookDeliveryResponse}
//	@Failure		400				{object}	ValidationErrorResponse
//	@Failure		401				{object}	APIResponse
//	@Failure		403				{object}	APIResponse
//	@Failure		500				{object}	APIResponse
//	@Router			/api/v1/admin/webhooks/deliveries [get]
func (h *WebhookAdminHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			ValidationError(w, map[string]string{"limit": "must be a positive integer"})
			return
		}
		if v > 500 {
			ValidationError(w, map[string]string{"limit": "must not exceed 500"})
			return
		}
		limit = v
	}

	items, err := h.service.ListDeliveries(r.Context(), userID, service.WebhookDeliveryListInput{
		SubscriptionID: strings.TrimSpace(r.URL.Query().Get("subscription_id")),
		Status:         strings.TrimSpace(r.URL.Query().Get("status")),
		EventType:      strings.TrimSpace(r.URL.Query().Get("event_type")),
		Limit:          limit,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list webhook deliveries")
		return
	}

	resp := make([]webhookDeliveryResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toWebhookDeliveryResponse(item))
	}
	Success(w, http.StatusOK, resp)
}

func toWebhookResponse(sub *model.WebhookSubscription) webhookResponse {
	return webhookResponse{
		ID:        sub.ID,
		Name:      sub.Name,
		URL:       sub.URL,
		Events:    append([]string(nil), sub.Events...),
		Enabled:   sub.Enabled,
		CreatedAt: sub.CreatedAt.Format(time.RFC3339),
		UpdatedAt: sub.UpdatedAt.Format(time.RFC3339),
	}
}

func toWebhookDeliveryResponse(item *model.WebhookDelivery) webhookDeliveryResponse {
	resp := webhookDeliveryResponse{
		ID:             item.ID,
		SubscriptionID: item.SubscriptionID,
		EventType:      item.EventType,
		EventID:        item.EventID,
		IdempotencyKey: item.IdempotencyKey,
		Attempt:        item.Attempt,
		Status:         item.Status,
		StatusCode:     item.StatusCode,
		Error:          item.Error,
		RequestBody:    item.RequestBody,
		DurationMS:     item.DurationMS,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
	}
	if item.NextAttemptAt != nil {
		v := item.NextAttemptAt.Format(time.RFC3339)
		resp.NextAttemptAt = &v
	}
	if item.DeliveredAt != nil {
		v := item.DeliveredAt.Format(time.RFC3339)
		resp.DeliveredAt = &v
	}
	return resp
}
