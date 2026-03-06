package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// WebhookRepository persists webhook subscriptions and delivery logs.
type WebhookRepository struct {
	db *sql.DB
}

// WebhookDeliveryFilter controls list filtering for delivery logs.
type WebhookDeliveryFilter struct {
	CreatorID      string
	SubscriptionID string
	Status         string
	EventType      string
	Limit          int
}

// NewWebhookRepository creates a new WebhookRepository.
func NewWebhookRepository(db *sql.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// CreateSubscription inserts a webhook subscription.
func (r *WebhookRepository) CreateSubscription(ctx context.Context, sub *model.WebhookSubscription) error {
	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_subscriptions (id, creator_id, name, url, secret_enc, events, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sub.ID, sub.CreatorID, sub.Name, sub.URL, sub.SecretEnc, encodeEvents(sub.Events), sub.Enabled, sub.CreatedAt, sub.UpdatedAt,
	)
	return err
}

// GetSubscriptionByID retrieves a subscription by ID.
func (r *WebhookRepository) GetSubscriptionByID(ctx context.Context, id string) (*model.WebhookSubscription, error) {
	sub := &model.WebhookSubscription{}
	var events string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, creator_id, name, url, secret_enc, events, enabled, created_at, updated_at
		 FROM webhook_subscriptions WHERE id = ?`,
		id,
	).Scan(
		&sub.ID, &sub.CreatorID, &sub.Name, &sub.URL, &sub.SecretEnc, &events, &sub.Enabled, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sub.Events = decodeEvents(events)
	return sub, nil
}

// ListSubscriptionsByCreator lists subscriptions created by a specific user.
func (r *WebhookRepository) ListSubscriptionsByCreator(ctx context.Context, creatorID string) ([]*model.WebhookSubscription, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, creator_id, name, url, secret_enc, events, enabled, created_at, updated_at
		 FROM webhook_subscriptions WHERE creator_id = ? ORDER BY created_at DESC`,
		creatorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*model.WebhookSubscription
	for rows.Next() {
		sub := &model.WebhookSubscription{}
		var events string
		if err := rows.Scan(
			&sub.ID, &sub.CreatorID, &sub.Name, &sub.URL, &sub.SecretEnc, &events, &sub.Enabled, &sub.CreatedAt, &sub.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sub.Events = decodeEvents(events)
		items = append(items, sub)
	}
	return items, rows.Err()
}

// ListEnabledByCreatorAndEvent returns active subscriptions matching an event.
func (r *WebhookRepository) ListEnabledByCreatorAndEvent(ctx context.Context, creatorID, eventType string) ([]*model.WebhookSubscription, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, creator_id, name, url, secret_enc, events, enabled, created_at, updated_at
		 FROM webhook_subscriptions
		 WHERE creator_id = ? AND enabled = 1 AND instr(',' || events || ',', ',' || ? || ',') > 0
		 ORDER BY created_at ASC`,
		creatorID, eventType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*model.WebhookSubscription
	for rows.Next() {
		sub := &model.WebhookSubscription{}
		var events string
		if err := rows.Scan(
			&sub.ID, &sub.CreatorID, &sub.Name, &sub.URL, &sub.SecretEnc, &events, &sub.Enabled, &sub.CreatedAt, &sub.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sub.Events = decodeEvents(events)
		items = append(items, sub)
	}
	return items, rows.Err()
}

// UpdateSubscription updates mutable fields of a subscription.
func (r *WebhookRepository) UpdateSubscription(ctx context.Context, sub *model.WebhookSubscription) error {
	sub.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE webhook_subscriptions
		 SET name = ?, url = ?, secret_enc = ?, events = ?, enabled = ?, updated_at = ?
		 WHERE id = ?`,
		sub.Name, sub.URL, sub.SecretEnc, encodeEvents(sub.Events), sub.Enabled, sub.UpdatedAt, sub.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSubscription removes a subscription by ID.
func (r *WebhookRepository) DeleteSubscription(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM webhook_subscriptions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateDelivery inserts a webhook delivery row.
func (r *WebhookRepository) CreateDelivery(ctx context.Context, delivery *model.WebhookDelivery) error {
	now := time.Now()
	delivery.CreatedAt = now
	delivery.UpdatedAt = now
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries
		 (id, subscription_id, event_type, event_id, idempotency_key, attempt, status, status_code, next_attempt_at, delivered_at, error, request_body, response_body, duration_ms, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		delivery.ID,
		delivery.SubscriptionID,
		delivery.EventType,
		delivery.EventID,
		delivery.IdempotencyKey,
		delivery.Attempt,
		delivery.Status,
		delivery.StatusCode,
		delivery.NextAttemptAt,
		delivery.DeliveredAt,
		delivery.Error,
		delivery.RequestBody,
		delivery.ResponseBody,
		delivery.DurationMS,
		delivery.CreatedAt,
		delivery.UpdatedAt,
	)
	if err != nil && isUniqueConstraintError(err) {
		return fmt.Errorf("%w: idempotency_key %s", ErrDuplicate, delivery.IdempotencyKey)
	}
	return err
}

// GetDeliveryByID retrieves a delivery by ID.
func (r *WebhookRepository) GetDeliveryByID(ctx context.Context, id string) (*model.WebhookDelivery, error) {
	d := &model.WebhookDelivery{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, subscription_id, event_type, event_id, idempotency_key, attempt, status, status_code, next_attempt_at, delivered_at, error, request_body, response_body, duration_ms, created_at, updated_at
		 FROM webhook_deliveries WHERE id = ?`,
		id,
	).Scan(
		&d.ID,
		&d.SubscriptionID,
		&d.EventType,
		&d.EventID,
		&d.IdempotencyKey,
		&d.Attempt,
		&d.Status,
		&d.StatusCode,
		&d.NextAttemptAt,
		&d.DeliveredAt,
		&d.Error,
		&d.RequestBody,
		&d.ResponseBody,
		&d.DurationMS,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

// UpdateDelivery updates mutable fields for a delivery row.
func (r *WebhookRepository) UpdateDelivery(ctx context.Context, delivery *model.WebhookDelivery) error {
	delivery.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE webhook_deliveries
		 SET attempt = ?, status = ?, status_code = ?, next_attempt_at = ?, delivered_at = ?, error = ?, response_body = ?, duration_ms = ?, updated_at = ?
		 WHERE id = ?`,
		delivery.Attempt,
		delivery.Status,
		delivery.StatusCode,
		delivery.NextAttemptAt,
		delivery.DeliveredAt,
		delivery.Error,
		delivery.ResponseBody,
		delivery.DurationMS,
		delivery.UpdatedAt,
		delivery.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDueDeliveries returns pending deliveries ready for processing.
func (r *WebhookRepository) ListDueDeliveries(ctx context.Context, now time.Time, limit int) ([]*model.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, subscription_id, event_type, event_id, idempotency_key, attempt, status, status_code, next_attempt_at, delivered_at, error, request_body, response_body, duration_ms, created_at, updated_at
		 FROM webhook_deliveries
		 WHERE status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
		 ORDER BY created_at ASC
		 LIMIT ?`,
		model.WebhookDeliveryStatusPending,
		now,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.WebhookDelivery
	for rows.Next() {
		d := &model.WebhookDelivery{}
		if err := rows.Scan(
			&d.ID,
			&d.SubscriptionID,
			&d.EventType,
			&d.EventID,
			&d.IdempotencyKey,
			&d.Attempt,
			&d.Status,
			&d.StatusCode,
			&d.NextAttemptAt,
			&d.DeliveredAt,
			&d.Error,
			&d.RequestBody,
			&d.ResponseBody,
			&d.DurationMS,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListDeliveries lists delivery logs with optional filters.
func (r *WebhookRepository) ListDeliveries(ctx context.Context, filter WebhookDeliveryFilter) ([]*model.WebhookDelivery, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}

	args := make([]interface{}, 0, 5)
	where := make([]string, 0, 4)
	join := ""

	if filter.CreatorID != "" {
		join = "JOIN webhook_subscriptions ws ON ws.id = wd.subscription_id"
		where = append(where, "ws.creator_id = ?")
		args = append(args, filter.CreatorID)
	}
	if filter.SubscriptionID != "" {
		where = append(where, "wd.subscription_id = ?")
		args = append(args, filter.SubscriptionID)
	}
	if filter.Status != "" {
		where = append(where, "wd.status = ?")
		args = append(args, filter.Status)
	}
	if filter.EventType != "" {
		where = append(where, "wd.event_type = ?")
		args = append(args, filter.EventType)
	}

	query := `SELECT wd.id, wd.subscription_id, wd.event_type, wd.event_id, wd.idempotency_key, wd.attempt, wd.status, wd.status_code, wd.next_attempt_at, wd.delivered_at, wd.error, wd.request_body, wd.response_body, wd.duration_ms, wd.created_at, wd.updated_at
	FROM webhook_deliveries wd ` + join
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY wd.created_at DESC LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.WebhookDelivery
	for rows.Next() {
		d := &model.WebhookDelivery{}
		if err := rows.Scan(
			&d.ID,
			&d.SubscriptionID,
			&d.EventType,
			&d.EventID,
			&d.IdempotencyKey,
			&d.Attempt,
			&d.Status,
			&d.StatusCode,
			&d.NextAttemptAt,
			&d.DeliveredAt,
			&d.Error,
			&d.RequestBody,
			&d.ResponseBody,
			&d.DurationMS,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func encodeEvents(events []string) string {
	if len(events) == 0 {
		return ""
	}
	clone := append([]string(nil), events...)
	sort.Strings(clone)
	out := make([]string, 0, len(clone))
	seen := make(map[string]struct{}, len(clone))
	for _, event := range clone {
		trimmed := strings.TrimSpace(event)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return strings.Join(out, ",")
}

func decodeEvents(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// isUniqueConstraintError checks whether err is a SQLite UNIQUE constraint
// violation. modernc.org/sqlite surfaces these as error strings containing
// "UNIQUE constraint failed".
func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
