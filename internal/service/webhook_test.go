package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

func TestWebhookService_DeliverySignatureAndHeaders(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db.DB())
	if err := userRepo.Create(ctx, &model.User{
		ID:           "user-1",
		Email:        "user-1@example.com",
		PasswordHash: "hash",
		DisplayName:  "User One",
	}); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	var (
		receivedBody      string
		receivedTimestamp string
		receivedSignature string
		receivedEvent     string
		receivedEventID   string
		receivedIdem      string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body := readBody(r.Body)
		receivedBody = body
		receivedTimestamp = r.Header.Get("X-Enlace-Timestamp")
		receivedSignature = r.Header.Get("X-Enlace-Signature")
		receivedEvent = r.Header.Get("X-Enlace-Event")
		receivedEventID = r.Header.Get("X-Enlace-Event-Id")
		receivedIdem = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := repository.NewWebhookRepository(db.DB())
	svc := NewWebhookService(repo, []byte("jwt-secret-for-tests"), server.Client())

	sub, secret, err := svc.CreateSubscription(ctx, "user-1", WebhookSubscriptionCreateInput{
		Name:   "test",
		URL:    server.URL,
		Events: []string{"file.upload.completed"},
	})
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	err = svc.Emit(ctx, WebhookEvent{
		Type:      "file.upload.completed",
		CreatorID: "user-1",
		ActorID:   "user-1",
		Resource:  "share-1",
		Data: map[string]interface{}{
			"share_id": "share-1",
			"count":    1,
		},
	})
	if err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	if receivedEvent != "file.upload.completed" {
		t.Fatalf("expected event header file.upload.completed, got %q", receivedEvent)
	}
	if receivedEventID == "" {
		t.Fatal("expected event id header")
	}
	if receivedIdem == "" {
		t.Fatal("expected idempotency key header")
	}
	if receivedTimestamp == "" {
		t.Fatal("expected timestamp header")
	}
	if receivedSignature == "" {
		t.Fatal("expected signature header")
	}

	expectedSig := "sha256=" + computeWebhookSignature(secret, receivedTimestamp, receivedBody)
	if receivedSignature != expectedSig {
		t.Fatalf("unexpected signature: got %q want %q", receivedSignature, expectedSig)
	}
	if len(receivedIdem) < len(sub.ID) || receivedIdem[len(receivedIdem)-len(sub.ID):] != sub.ID {
		t.Fatalf("idempotency key %q should end with subscription id %q", receivedIdem, sub.ID)
	}

	deliveries, err := svc.ListDeliveries(ctx, "user-1", WebhookDeliveryListInput{Limit: 10})
	if err != nil {
		t.Fatalf("list deliveries failed: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0].Status != model.WebhookDeliveryStatusDelivered {
		t.Fatalf("expected delivery status delivered, got %s", deliveries[0].Status)
	}
}

func TestWebhookService_RetryKeepsIdempotencyKey(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db.DB())
	if err := userRepo.Create(ctx, &model.User{
		ID:           "user-2",
		Email:        "user-2@example.com",
		PasswordHash: "hash",
		DisplayName:  "User Two",
	}); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	var mu sync.Mutex
	attempt := 0
	idempotencyKeys := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		attempt++
		idempotencyKeys = append(idempotencyKeys, r.Header.Get("Idempotency-Key"))
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("retry"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	repo := repository.NewWebhookRepository(db.DB())
	svc := NewWebhookService(repo, []byte("jwt-secret-for-tests"), server.Client())

	fakeNow := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fakeNow }

	_, _, err = svc.CreateSubscription(ctx, "user-2", WebhookSubscriptionCreateInput{
		Name:   "retry-sub",
		URL:    server.URL,
		Events: []string{"share.viewed"},
	})
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	err = svc.Emit(ctx, WebhookEvent{
		Type:      "share.viewed",
		CreatorID: "user-2",
		Resource:  "share-2",
		Data:      map[string]interface{}{"share_id": "share-2"},
	})
	if err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	deliveries, err := svc.ListDeliveries(ctx, "user-2", WebhookDeliveryListInput{Limit: 10})
	if err != nil {
		t.Fatalf("list deliveries failed: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery after first attempt, got %d", len(deliveries))
	}
	if deliveries[0].Status != model.WebhookDeliveryStatusPending {
		t.Fatalf("expected pending after first failed attempt, got %s", deliveries[0].Status)
	}
	if deliveries[0].Attempt != 1 {
		t.Fatalf("expected attempt=1 after first send, got %d", deliveries[0].Attempt)
	}

	fakeNow = fakeNow.Add(2 * time.Minute)
	if err := svc.ProcessDueDeliveries(ctx, 10); err != nil {
		t.Fatalf("process due deliveries failed: %v", err)
	}

	deliveries, err = svc.ListDeliveries(ctx, "user-2", WebhookDeliveryListInput{Limit: 10})
	if err != nil {
		t.Fatalf("list deliveries failed: %v", err)
	}
	if deliveries[0].Status != model.WebhookDeliveryStatusDelivered {
		t.Fatalf("expected delivered after retry, got %s", deliveries[0].Status)
	}
	if deliveries[0].Attempt != 2 {
		t.Fatalf("expected attempt=2 after retry, got %d", deliveries[0].Attempt)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(idempotencyKeys) != 2 {
		t.Fatalf("expected 2 webhook attempts, got %d", len(idempotencyKeys))
	}
	if idempotencyKeys[0] == "" || idempotencyKeys[1] == "" {
		t.Fatalf("idempotency keys should not be empty: %#v", idempotencyKeys)
	}
	if idempotencyKeys[0] != idempotencyKeys[1] {
		t.Fatalf("idempotency key should stay stable across retries: %#v", idempotencyKeys)
	}
}
