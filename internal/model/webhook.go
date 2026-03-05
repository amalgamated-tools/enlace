package model

import "time"

// WebhookSubscription defines a configured webhook destination.
type WebhookSubscription struct {
	ID        string
	CreatorID string
	Name      string
	URL       string
	SecretEnc string
	Events    []string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WebhookDelivery tracks one event delivery lifecycle for a subscription.
type WebhookDelivery struct {
	ID             string
	SubscriptionID string
	EventType      string
	EventID        string
	IdempotencyKey string
	Attempt        int
	Status         string
	StatusCode     *int
	NextAttemptAt  *time.Time
	DeliveredAt    *time.Time
	Error          string
	RequestBody    string
	ResponseBody   string
	DurationMS     int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const (
	WebhookDeliveryStatusPending   = "pending"
	WebhookDeliveryStatusDelivered = "delivered"
	WebhookDeliveryStatusFailed    = "failed"
)
