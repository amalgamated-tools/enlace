package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/amalgamated-tools/enlace/internal/crypto"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

var (
	// ErrInvalidWebhookURL is returned when the webhook URL is not allowed.
	ErrInvalidWebhookURL = errors.New("invalid webhook url")
	// ErrInvalidWebhookEvents is returned for unsupported event sets.
	ErrInvalidWebhookEvents = errors.New("invalid webhook events")
	// ErrWebhookNotFound is returned when a subscription does not exist.
	ErrWebhookNotFound = errors.New("webhook not found")
)

var defaultRetryBackoff = []time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
	6 * time.Hour,
}

const (
	webhookMaxAttempts      = 5
	webhookResponseBodyMax  = 2048
	webhookDeliveryTimeout  = 10 * time.Second
	webhookWorkerBatchLimit = 50
)

// WebhookSubscriptionCreateInput defines create input.
type WebhookSubscriptionCreateInput struct {
	Name   string
	URL    string
	Events []string
}

// WebhookSubscriptionUpdateInput defines patch input.
type WebhookSubscriptionUpdateInput struct {
	Name    *string
	URL     *string
	Events  []string
	Enabled *bool
}

// WebhookDeliveryListInput controls delivery listing.
type WebhookDeliveryListInput struct {
	SubscriptionID string
	Status         string
	EventType      string
	Limit          int
}

// WebhookEvent carries event metadata and payload.
type WebhookEvent struct {
	Type      string
	CreatorID string
	ActorID   string
	Resource  string
	Data      interface{}
}

// WebhookService manages subscriptions and deliveries.
type WebhookService struct {
	repo          *repository.WebhookRepository
	encryptionKey []byte
	httpClient    *http.Client
	now           func() time.Time
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(repo *repository.WebhookRepository, secret []byte, client *http.Client) *WebhookService {
	if client == nil {
		client = &http.Client{Timeout: webhookDeliveryTimeout}
	}
	return &WebhookService{
		repo:          repo,
		encryptionKey: crypto.DeriveKey(secret, "webhook-secret-encryption"),
		httpClient:    client,
		now:           time.Now,
	}
}

// AllowedWebhookEvents returns all supported event names.
func AllowedWebhookEvents() []string {
	return []string{
		"file.upload.completed",
		"share.viewed",
		"share.downloaded",
		"share.created",
	}
}

// CreateSubscription creates a webhook subscription and returns the generated secret once.
func (s *WebhookService) CreateSubscription(ctx context.Context, creatorID string, input WebhookSubscriptionCreateInput) (*model.WebhookSubscription, string, error) {
	if creatorID == "" {
		return nil, "", ErrWebhookNotFound
	}
	if err := validateWebhookURL(input.URL); err != nil {
		return nil, "", err
	}
	events, err := normalizeWebhookEvents(input.Events)
	if err != nil {
		return nil, "", err
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		return nil, "", err
	}
	encrypted, err := crypto.Encrypt(secret, s.encryptionKey)
	if err != nil {
		return nil, "", err
	}

	sub := &model.WebhookSubscription{
		ID:        uuid.NewString(),
		CreatorID: creatorID,
		Name:      strings.TrimSpace(input.Name),
		URL:       strings.TrimSpace(input.URL),
		SecretEnc: encrypted,
		Events:    events,
		Enabled:   true,
	}
	if sub.Name == "" {
		sub.Name = "webhook"
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return nil, "", err
	}
	return sub, secret, nil
}

// ListSubscriptionsByCreator returns webhook subscriptions for a user.
func (s *WebhookService) ListSubscriptionsByCreator(ctx context.Context, creatorID string) ([]*model.WebhookSubscription, error) {
	return s.repo.ListSubscriptionsByCreator(ctx, creatorID)
}

// UpdateSubscription updates mutable fields for a subscription owned by creatorID.
func (s *WebhookService) UpdateSubscription(ctx context.Context, creatorID, id string, input WebhookSubscriptionUpdateInput) (*model.WebhookSubscription, error) {
	sub, err := s.repo.GetSubscriptionByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	if sub.CreatorID != creatorID {
		return nil, ErrWebhookNotFound
	}

	if input.Name != nil {
		sub.Name = strings.TrimSpace(*input.Name)
	}
	if input.URL != nil {
		if err := validateWebhookURL(*input.URL); err != nil {
			return nil, err
		}
		sub.URL = strings.TrimSpace(*input.URL)
	}
	if input.Events != nil {
		normalized, err := normalizeWebhookEvents(input.Events)
		if err != nil {
			return nil, err
		}
		sub.Events = normalized
	}
	if input.Enabled != nil {
		sub.Enabled = *input.Enabled
	}

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	return sub, nil
}

// DeleteSubscription deletes a webhook subscription owned by creatorID.
func (s *WebhookService) DeleteSubscription(ctx context.Context, creatorID, id string) error {
	sub, err := s.repo.GetSubscriptionByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrWebhookNotFound
		}
		return err
	}
	if sub.CreatorID != creatorID {
		return ErrWebhookNotFound
	}
	if err := s.repo.DeleteSubscription(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrWebhookNotFound
		}
		return err
	}
	return nil
}

// ListDeliveries returns delivery logs for a creator.
func (s *WebhookService) ListDeliveries(ctx context.Context, creatorID string, input WebhookDeliveryListInput) ([]*model.WebhookDelivery, error) {
	return s.repo.ListDeliveries(ctx, repository.WebhookDeliveryFilter{
		CreatorID:      creatorID,
		SubscriptionID: strings.TrimSpace(input.SubscriptionID),
		Status:         strings.TrimSpace(input.Status),
		EventType:      strings.TrimSpace(input.EventType),
		Limit:          input.Limit,
	})
}

// Emit creates delivery rows for matching subscriptions and attempts immediate delivery.
func (s *WebhookService) Emit(ctx context.Context, event WebhookEvent) error {
	eventType := strings.TrimSpace(event.Type)
	if eventType == "" || strings.TrimSpace(event.CreatorID) == "" {
		return nil
	}
	if !isAllowedEvent(eventType) {
		return ErrInvalidWebhookEvents
	}

	subs, err := s.repo.ListEnabledByCreatorAndEvent(ctx, event.CreatorID, eventType)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	now := s.now().UTC()
	eventID := uuid.NewString()
	envelope := map[string]interface{}{
		"id":          eventID,
		"type":        eventType,
		"occurred_at": now.Format(time.RFC3339Nano),
		"data":        event.Data,
	}
	if strings.TrimSpace(event.ActorID) != "" {
		envelope["actor"] = map[string]string{"id": strings.TrimSpace(event.ActorID)}
	}
	if strings.TrimSpace(event.Resource) != "" {
		envelope["resource"] = map[string]string{"id": strings.TrimSpace(event.Resource)}
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		delivery := &model.WebhookDelivery{
			ID:             uuid.NewString(),
			SubscriptionID: sub.ID,
			EventType:      eventType,
			EventID:        eventID,
			IdempotencyKey: eventID + ":" + sub.ID,
			Attempt:        0,
			Status:         model.WebhookDeliveryStatusPending,
			RequestBody:    string(body),
		}
		next := now
		delivery.NextAttemptAt = &next

		if err := s.repo.CreateDelivery(ctx, delivery); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				continue
			}
			return err
		}

		if err := s.processDelivery(ctx, delivery, sub); err != nil {
			return err
		}
	}

	return nil
}

// RunDeliveryWorker continuously processes due webhook deliveries.
func (s *WebhookService) RunDeliveryWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.ProcessDueDeliveries(ctx, webhookWorkerBatchLimit)
		}
	}
}

// ProcessDueDeliveries processes pending deliveries that are due.
func (s *WebhookService) ProcessDueDeliveries(ctx context.Context, limit int) error {
	deliveries, err := s.repo.ListDueDeliveries(ctx, s.now(), limit)
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		sub, err := s.repo.GetSubscriptionByID(ctx, delivery.SubscriptionID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				now := s.now()
				delivery.Attempt++
				delivery.Status = model.WebhookDeliveryStatusFailed
				delivery.NextAttemptAt = nil
				delivery.DeliveredAt = nil
				delivery.Error = "subscription not found"
				delivery.DurationMS = 0
				delivery.StatusCode = nil
				delivery.UpdatedAt = now
				_ = s.repo.UpdateDelivery(ctx, delivery)
				continue
			}
			return err
		}
		if !sub.Enabled {
			now := s.now()
			delivery.Attempt++
			delivery.Status = model.WebhookDeliveryStatusFailed
			delivery.NextAttemptAt = nil
			delivery.DeliveredAt = nil
			delivery.Error = "subscription disabled"
			delivery.DurationMS = 0
			delivery.StatusCode = nil
			delivery.UpdatedAt = now
			_ = s.repo.UpdateDelivery(ctx, delivery)
			continue
		}
		if err := s.processDelivery(ctx, delivery, sub); err != nil {
			return err
		}
	}
	return nil
}

func (s *WebhookService) processDelivery(ctx context.Context, delivery *model.WebhookDelivery, sub *model.WebhookSubscription) error {
	secret, err := crypto.Decrypt(sub.SecretEnc, s.encryptionKey)
	if err != nil {
		return err
	}

	attempt := delivery.Attempt + 1
	start := s.now()
	timestamp := s.now().UTC().Format(time.RFC3339)
	signature := computeWebhookSignature(secret, timestamp, delivery.RequestBody)

	reqCtx, cancel := context.WithTimeout(ctx, webhookDeliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, sub.URL, bytes.NewBufferString(delivery.RequestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Enlace-Event", delivery.EventType)
	req.Header.Set("X-Enlace-Event-Id", delivery.EventID)
	req.Header.Set("X-Enlace-Timestamp", timestamp)
	req.Header.Set("X-Enlace-Signature", "sha256="+signature)
	req.Header.Set("Idempotency-Key", delivery.IdempotencyKey)

	resp, reqErr := s.httpClient.Do(req)
	elapsed := time.Since(start).Milliseconds()

	delivery.Attempt = attempt
	delivery.DurationMS = elapsed
	delivery.NextAttemptAt = nil
	delivery.DeliveredAt = nil
	delivery.StatusCode = nil
	delivery.Error = ""
	delivery.ResponseBody = ""

	var statusCode int
	var responseBody string
	if resp != nil {
		statusCode = resp.StatusCode
		delivery.StatusCode = &statusCode
		responseBody = readBody(resp.Body)
		delivery.ResponseBody = truncate(responseBody, webhookResponseBodyMax)
	}

	if reqErr == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		now := s.now()
		delivery.Status = model.WebhookDeliveryStatusDelivered
		delivery.DeliveredAt = &now
		return s.repo.UpdateDelivery(ctx, delivery)
	}

	retryable := reqErr != nil || statusCode == http.StatusTooManyRequests || statusCode >= 500
	if reqErr != nil {
		delivery.Error = reqErr.Error()
	} else {
		delivery.Error = fmt.Sprintf("receiver returned HTTP %d", statusCode)
	}

	if retryable && attempt < webhookMaxAttempts {
		delivery.Status = model.WebhookDeliveryStatusPending
		next := s.now().Add(backoffForAttempt(attempt))
		delivery.NextAttemptAt = &next
	} else {
		delivery.Status = model.WebhookDeliveryStatusFailed
	}

	return s.repo.UpdateDelivery(ctx, delivery)
}

func validateWebhookURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ErrInvalidWebhookURL
	}
	if u.Scheme == "" || u.Host == "" {
		return ErrInvalidWebhookURL
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
		return nil
	}
	return ErrInvalidWebhookURL
}

func normalizeWebhookEvents(events []string) ([]string, error) {
	allowed := make(map[string]struct{}, len(AllowedWebhookEvents()))
	for _, event := range AllowedWebhookEvents() {
		allowed[event] = struct{}{}
	}

	if len(events) == 0 {
		return nil, ErrInvalidWebhookEvents
	}

	seen := make(map[string]struct{}, len(events))
	out := make([]string, 0, len(events))
	for _, event := range events {
		normalized := strings.TrimSpace(event)
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; !ok {
			return nil, ErrInvalidWebhookEvents
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, ErrInvalidWebhookEvents
	}
	sort.Strings(out)
	return out, nil
}

func isAllowedEvent(eventType string) bool {
	for _, allowed := range AllowedWebhookEvents() {
		if allowed == eventType {
			return true
		}
	}
	return false
}

func generateWebhookSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func computeWebhookSignature(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func backoffForAttempt(attempt int) time.Duration {
	if attempt <= 0 {
		return defaultRetryBackoff[0]
	}
	idx := attempt - 1
	if idx >= len(defaultRetryBackoff) {
		idx = len(defaultRetryBackoff) - 1
	}
	return defaultRetryBackoff[idx]
}

func readBody(body io.ReadCloser) string {
	if body == nil {
		return ""
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, webhookResponseBodyMax+1))
	if err != nil {
		return ""
	}
	return string(data)
}

func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit]
}
