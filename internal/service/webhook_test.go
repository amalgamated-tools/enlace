package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/crypto"
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

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		// Loopback
		{"IPv4 loopback", "127.0.0.1", true},
		{"IPv4 loopback other", "127.0.0.2", true},
		{"IPv6 loopback", "::1", true},

		// Private RFC 1918
		{"10.x private", "10.0.0.1", true},
		{"172.16.x private", "172.16.0.1", true},
		{"192.168.x private", "192.168.1.1", true},

		// Link-local
		{"IPv4 link-local", "169.254.1.1", true},
		{"IPv6 link-local", "fe80::1", true},

		// CGNAT
		{"CGNAT", "100.64.0.1", true},
		{"CGNAT upper", "100.127.255.254", true},

		// Benchmarking
		{"Benchmarking", "198.18.0.1", true},

		// Documentation
		{"TEST-NET-1", "192.0.2.1", true},
		{"TEST-NET-2", "198.51.100.1", true},
		{"TEST-NET-3", "203.0.113.1", true},

		// IPv6 unique local
		{"IPv6 unique local", "fd00::1", true},

		// IPv6 documentation
		{"IPv6 documentation", "2001:db8::1", true},

		// IPv4-mapped IPv6
		{"IPv4-mapped IPv6 private", "::ffff:10.0.0.1", true},
		{"IPv4-mapped IPv6 loopback", "::ffff:127.0.0.1", true},

		// Multicast
		{"IPv4 multicast", "224.0.0.1", true},
		{"IPv6 multicast", "ff02::1", true},

		// Unspecified
		{"IPv4 unspecified", "0.0.0.0", true},
		{"IPv6 unspecified", "::", true},

		// Allowed public IPs
		{"Public 8.8.8.8", "8.8.8.8", false},
		{"Public 1.1.1.1", "1.1.1.1", false},
		{"Public 93.184.216.34", "93.184.216.34", false},
		{"Public IPv6", "2607:f8b0:4004:800::200e", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			got := isBlockedIP(ip)
			if got != tt.blocked {
				t.Errorf("isBlockedIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
			}
		})
	}
}

func TestValidateWebhookURL_BlocksLocalAddresses(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Should be blocked
		{"HTTPS localhost", "https://localhost/webhook", true},
		{"HTTPS 127.0.0.1", "https://127.0.0.1/webhook", true},
		{"HTTPS IPv6 loopback", "https://[::1]/webhook", true},
		{"HTTPS 0.0.0.0", "https://0.0.0.0/webhook", true},
		{"HTTP non-local", "http://example.com/webhook", true},
		{"No scheme", "example.com/webhook", true},
		{"Empty", "", true},
		{"FTP", "ftp://example.com/webhook", true},

		// Should be allowed at URL validation level
		// (safeDialContext provides the second layer of defense)
		{"HTTPS public", "https://example.com/webhook", false},
		{"HTTPS with port", "https://hooks.example.com:8443/webhook", false},
		{"HTTP localhost (dev)", "http://localhost/webhook", false},
		{"HTTP localhost with port", "http://localhost:8080/webhook", false},
		{"HTTP 127.0.0.1 (dev)", "http://127.0.0.1/webhook", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebhookURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWebhookURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestSafeDialContext_BlocksPrivateIPs(t *testing.T) {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	dialFn := safeDialContext(dialer)
	ctx := context.Background()

	// 127.0.0.1:80 should be blocked
	_, err := dialFn(ctx, "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("expected safeDialContext to block 127.0.0.1")
	}
	if !strings.Contains(err.Error(), "blocked address") {
		t.Fatalf("expected SSRF block error, got: %v", err)
	}

	// [::1]:80 should be blocked
	_, err = dialFn(ctx, "tcp", "[::1]:80")
	if err == nil {
		t.Fatal("expected safeDialContext to block [::1]")
	}
	if !strings.Contains(err.Error(), "blocked address") {
		t.Fatalf("expected SSRF block error, got: %v", err)
	}
}

func TestWebhookDelivery_SSRFBlocked(t *testing.T) {
	// This test verifies the full end-to-end path: creating a subscription
	// with a URL that passes validateWebhookURL (HTTPS public domain) but
	// whose dialed IP is private should be blocked at delivery time.
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db.DB())
	if err := userRepo.Create(ctx, &model.User{
		ID:           "user-ssrf",
		Email:        "ssrf@example.com",
		PasswordHash: "hash",
		DisplayName:  "SSRF Test",
	}); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	repo := repository.NewWebhookRepository(db.DB())

	// Use the default safe client (not a test server client).
	svc := NewWebhookService(repo, []byte("jwt-secret-for-tests"), nil)

	// Create subscription pointing to a loopback address.
	// We directly insert via the repo to bypass URL validation,
	// simulating a DNS rebinding scenario.
	sub := &model.WebhookSubscription{
		ID:        "sub-ssrf",
		CreatorID: "user-ssrf",
		Name:      "ssrf-test",
		URL:       "http://127.0.0.1:1/webhook",
		Events:    []string{"share.created"},
		Enabled:   true,
	}
	encrypted, err := encryptTestSecret(svc.encryptionKey)
	if err != nil {
		t.Fatalf("failed to encrypt secret: %v", err)
	}
	sub.SecretEnc = encrypted
	if err := repo.CreateSubscription(ctx, sub); err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// Emit an event -- delivery should fail because the safe dialer blocks 127.0.0.1
	err = svc.Emit(ctx, WebhookEvent{
		Type:      "share.created",
		CreatorID: "user-ssrf",
		Resource:  "share-ssrf",
		Data:      map[string]interface{}{"share_id": "share-ssrf"},
	})
	if err != nil {
		t.Fatalf("emit returned error: %v", err)
	}

	deliveries, err := svc.ListDeliveries(ctx, "user-ssrf", WebhookDeliveryListInput{Limit: 10})
	if err != nil {
		t.Fatalf("list deliveries failed: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	// The delivery should be pending (retryable network error) or failed,
	// but NOT delivered.
	if deliveries[0].Status == model.WebhookDeliveryStatusDelivered {
		t.Fatal("expected delivery to NOT be delivered when targeting a blocked IP")
	}
	if deliveries[0].Error == "" {
		t.Fatal("expected delivery error to be set when targeting a blocked IP")
	}
}

func TestNewSafeHTTPClient_DoesNotFollowRedirects(t *testing.T) {
	// Stand up a server that redirects to a different URL.
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("should not reach here"))
	}))
	defer redirectTarget.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer redirector.Close()

	client := newSafeHTTPClient(5 * time.Second)
	// Override transport to allow test server connections (TLS skip etc.)
	// but keep CheckRedirect from newSafeHTTPClient.
	client.Transport = nil

	resp, err := client.Get(redirector.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Should get the redirect response, NOT follow it.
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d (redirect), got %d -- client followed the redirect", http.StatusFound, resp.StatusCode)
	}
}

// encryptTestSecret encrypts a dummy secret using the provided encryption key.
func encryptTestSecret(key []byte) (string, error) {
	return crypto.Encrypt("test-webhook-secret", key)
}
