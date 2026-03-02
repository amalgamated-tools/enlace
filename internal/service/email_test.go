package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	mail "github.com/wneessen/go-mail"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
)

var configuredSMTP = service.SMTPConfig{
	Host: "smtp.example.com",
	Port: 587,
	From: "noreply@example.com",
}

// stubSender implements service.MailSender for testing.
type stubSender struct {
	sentMsgs []*mail.Msg
	err      error
}

func (s *stubSender) DialAndSendWithContext(_ context.Context, msgs ...*mail.Msg) error {
	s.sentMsgs = append(s.sentMsgs, msgs...)
	return s.err
}

type emailTestEnv struct {
	svc           *service.EmailService
	recipientRepo *repository.RecipientRepository
	db            *sql.DB
	cleanup       func()
}

func setupEmailService(t *testing.T, cfg service.SMTPConfig) *emailTestEnv {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	recipientRepo := repository.NewRecipientRepository(db.DB())
	emailService := service.NewEmailService(cfg, recipientRepo, "http://localhost:8080")

	return &emailTestEnv{
		svc:           emailService,
		recipientRepo: recipientRepo,
		db:            db.DB(),
		cleanup:       func() { db.Close() },
	}
}

func TestEmailService_IsConfigured(t *testing.T) {
	t.Run("configured when host, port, and from are set", func(t *testing.T) {
		env := setupEmailService(t, configuredSMTP)
		defer env.cleanup()

		if !env.svc.IsConfigured() {
			t.Error("expected IsConfigured to return true")
		}
	})

	t.Run("not configured when host is empty", func(t *testing.T) {
		env := setupEmailService(t, service.SMTPConfig{Port: 587, From: "noreply@example.com"})
		defer env.cleanup()

		if env.svc.IsConfigured() {
			t.Error("expected IsConfigured to return false")
		}
	})

	t.Run("not configured when port is zero", func(t *testing.T) {
		env := setupEmailService(t, service.SMTPConfig{Host: "smtp.example.com", From: "noreply@example.com"})
		defer env.cleanup()

		if env.svc.IsConfigured() {
			t.Error("expected IsConfigured to return false")
		}
	})

	t.Run("not configured when from is empty", func(t *testing.T) {
		env := setupEmailService(t, service.SMTPConfig{Host: "smtp.example.com", Port: 587})
		defer env.cleanup()

		if env.svc.IsConfigured() {
			t.Error("expected IsConfigured to return false")
		}
	})
}

func TestEmailService_SendShareNotification_SkipsWhenNotConfigured(t *testing.T) {
	env := setupEmailService(t, service.SMTPConfig{})
	defer env.cleanup()

	ctx := context.Background()
	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}

	err := env.svc.SendShareNotification(ctx, share, []string{"test@example.com"})
	if err != nil {
		t.Fatalf("expected no error when SMTP not configured, got: %v", err)
	}

	// No recipients should be recorded
	recipients, err := env.recipientRepo.ListByShare(ctx, "share-123")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients when not configured, got %d", len(recipients))
	}
}

func TestEmailService_SendShareNotification_SkipsEmptyEmails(t *testing.T) {
	env := setupEmailService(t, configuredSMTP)
	defer env.cleanup()

	stub := &stubSender{}
	env.svc.SetSender(stub)

	ctx := context.Background()
	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}

	err := env.svc.SendShareNotification(ctx, share, []string{"", "  ", ""})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(stub.sentMsgs) != 0 {
		t.Errorf("expected 0 messages sent for empty emails, got %d", len(stub.sentMsgs))
	}

	recipients, err := env.recipientRepo.ListByShare(ctx, "share-123")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients for empty emails, got %d", len(recipients))
	}
}

func TestEmailService_SendShareNotification_RecordsRecipients(t *testing.T) {
	env := setupEmailService(t, configuredSMTP)
	defer env.cleanup()

	stub := &stubSender{}
	env.svc.SetSender(stub)

	ctx := context.Background()

	// Create a share so foreign key is satisfied
	shareRepo := repository.NewShareRepository(env.db)
	share := &model.Share{ID: "share-rec", Slug: "rec-test", Name: "Rec Test"}
	if err := shareRepo.Create(ctx, share); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	err := env.svc.SendShareNotification(ctx, share, []string{"a@example.com", "b@example.com"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(stub.sentMsgs) != 2 {
		t.Errorf("expected 2 messages sent, got %d", len(stub.sentMsgs))
	}

	recipients, err := env.recipientRepo.ListByShare(ctx, "share-rec")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 2 {
		t.Errorf("expected 2 recipients recorded, got %d", len(recipients))
	}
}

func TestEmailService_SendShareNotification_ReturnsErrorOnFailure(t *testing.T) {
	env := setupEmailService(t, configuredSMTP)
	defer env.cleanup()

	stub := &stubSender{err: fmt.Errorf("connection refused")}
	env.svc.SetSender(stub)

	ctx := context.Background()
	share := &model.Share{ID: "share-fail", Slug: "fail-test", Name: "Fail Test"}

	err := env.svc.SendShareNotification(ctx, share, []string{"a@example.com"})
	if err == nil {
		t.Fatal("expected error when send fails, got nil")
	}

	if !strings.Contains(err.Error(), "a@example.com") {
		t.Errorf("expected error to contain failed address, got: %v", err)
	}

	// No recipients should be recorded for failed sends
	recipients, err := env.recipientRepo.ListByShare(ctx, "share-fail")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients for failed sends, got %d", len(recipients))
	}
}

func TestEmailService_SendShareNotification_MessageContent(t *testing.T) {
	env := setupEmailService(t, configuredSMTP)
	defer env.cleanup()

	stub := &stubSender{}
	env.svc.SetSender(stub)

	ctx := context.Background()

	shareRepo := repository.NewShareRepository(env.db)
	share := &model.Share{ID: "share-content", Slug: "content-test", Name: "Content Test"}
	if err := shareRepo.Create(ctx, share); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	err := env.svc.SendShareNotification(ctx, share, []string{"user@example.com"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(stub.sentMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(stub.sentMsgs))
	}

	msg := stub.sentMsgs[0]

	// Verify From address
	fromAddrs := msg.GetFrom()
	if len(fromAddrs) != 1 || fromAddrs[0].Address != "noreply@example.com" {
		t.Errorf("expected From to be noreply@example.com, got %v", fromAddrs)
	}

	// Verify To address
	toAddrs := msg.GetTo()
	if len(toAddrs) != 1 || toAddrs[0].Address != "user@example.com" {
		t.Errorf("expected To to be user@example.com, got %v", toAddrs)
	}

	// Verify Subject
	subjects := msg.GetGenHeader(mail.HeaderSubject)
	if len(subjects) != 1 || subjects[0] != "Content Test has been shared with you on Enlace" {
		t.Errorf("unexpected Subject: %v", subjects)
	}
}

func TestEmailService_ListRecipients(t *testing.T) {
	env := setupEmailService(t, service.SMTPConfig{})
	defer env.cleanup()

	ctx := context.Background()

	// Create a share in the same DB so the foreign key is satisfied
	shareRepo := repository.NewShareRepository(env.db)
	share := &model.Share{ID: "share-list", Slug: "list-test", Name: "List Test"}
	if err := shareRepo.Create(ctx, share); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Add recipients directly
	if err := env.recipientRepo.Create(ctx, &model.ShareRecipient{ID: "r-1", ShareID: "share-list", Email: "a@example.com"}); err != nil {
		t.Fatalf("failed to create recipient: %v", err)
	}
	if err := env.recipientRepo.Create(ctx, &model.ShareRecipient{ID: "r-2", ShareID: "share-list", Email: "b@example.com"}); err != nil {
		t.Fatalf("failed to create recipient: %v", err)
	}

	recipients, err := env.svc.ListRecipients(ctx, "share-list")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(recipients))
	}
}
