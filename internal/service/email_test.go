package service_test

import (
	"context"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
)

func setupEmailService(t *testing.T, cfg service.SMTPConfig) (*service.EmailService, *repository.RecipientRepository, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	recipientRepo := repository.NewRecipientRepository(db.DB())
	emailService := service.NewEmailService(cfg, recipientRepo, "http://localhost:8080")

	return emailService, recipientRepo, func() { db.Close() }
}

func TestEmailService_IsConfigured(t *testing.T) {
	t.Run("configured when host, port, and from are set", func(t *testing.T) {
		svc, _, cleanup := setupEmailService(t, service.SMTPConfig{Host: "smtp.example.com", Port: 587, From: "noreply@example.com"})
		defer cleanup()

		if !svc.IsConfigured() {
			t.Error("expected IsConfigured to return true")
		}
	})

	t.Run("not configured when host is empty", func(t *testing.T) {
		svc, _, cleanup := setupEmailService(t, service.SMTPConfig{Port: 587, From: "noreply@example.com"})
		defer cleanup()

		if svc.IsConfigured() {
			t.Error("expected IsConfigured to return false")
		}
	})

	t.Run("not configured when port is zero", func(t *testing.T) {
		svc, _, cleanup := setupEmailService(t, service.SMTPConfig{Host: "smtp.example.com", From: "noreply@example.com"})
		defer cleanup()

		if svc.IsConfigured() {
			t.Error("expected IsConfigured to return false")
		}
	})

	t.Run("not configured when from is empty", func(t *testing.T) {
		svc, _, cleanup := setupEmailService(t, service.SMTPConfig{Host: "smtp.example.com", Port: 587})
		defer cleanup()

		if svc.IsConfigured() {
			t.Error("expected IsConfigured to return false")
		}
	})
}

func TestEmailService_SendShareNotification_SkipsWhenNotConfigured(t *testing.T) {
	svc, recipientRepo, cleanup := setupEmailService(t, service.SMTPConfig{})
	defer cleanup()

	ctx := context.Background()
	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}

	err := svc.SendShareNotification(ctx, share, []string{"test@example.com"})
	if err != nil {
		t.Fatalf("expected no error when SMTP not configured, got: %v", err)
	}

	// No recipients should be recorded
	recipients, err := recipientRepo.ListByShare(ctx, "share-123")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients when not configured, got %d", len(recipients))
	}
}

func TestEmailService_SendShareNotification_SkipsEmptyEmails(t *testing.T) {
	svc, recipientRepo, cleanup := setupEmailService(t, service.SMTPConfig{})
	defer cleanup()

	ctx := context.Background()
	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}

	err := svc.SendShareNotification(ctx, share, []string{"", "  ", ""})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	recipients, err := recipientRepo.ListByShare(ctx, "share-123")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients for empty emails, got %d", len(recipients))
	}
}

func TestEmailService_ListRecipients(t *testing.T) {
	_, recipientRepo, cleanup := setupEmailService(t, service.SMTPConfig{})
	defer cleanup()

	ctx := context.Background()

	// Need to set up DB with a share first (through the database directly)
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	recRepo := repository.NewRecipientRepository(db.DB())
	svc := service.NewEmailService(service.SMTPConfig{}, recRepo, "http://localhost:8080")

	share := &model.Share{ID: "share-list", Slug: "list-test", Name: "List Test"}
	_ = shareRepo.Create(ctx, share)

	// Add recipients directly
	_ = recRepo.Create(ctx, &model.ShareRecipient{ID: "r-1", ShareID: "share-list", Email: "a@example.com"})
	_ = recRepo.Create(ctx, &model.ShareRecipient{ID: "r-2", ShareID: "share-list", Email: "b@example.com"})

	recipients, err := svc.ListRecipients(ctx, "share-list")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(recipients))
	}

	// Unused repo from setup
	_ = recipientRepo
}
