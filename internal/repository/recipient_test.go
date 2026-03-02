package repository_test

import (
	"context"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

func TestRecipientRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	recipientRepo := repository.NewRecipientRepository(db.DB())
	ctx := context.Background()

	// Create a share first for the foreign key
	share := &model.Share{
		ID:   "share-123",
		Slug: "my-share",
		Name: "Test Share",
	}
	if err := shareRepo.Create(ctx, share); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	recipient := &model.ShareRecipient{
		ID:      "recipient-1",
		ShareID: "share-123",
		Email:   "test@example.com",
	}

	err := recipientRepo.Create(ctx, recipient)
	if err != nil {
		t.Fatalf("failed to create recipient: %v", err)
	}

	if recipient.SentAt.IsZero() {
		t.Error("expected SentAt to be set")
	}
}

func TestRecipientRepository_ListByShare(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	recipientRepo := repository.NewRecipientRepository(db.DB())
	ctx := context.Background()

	// Create shares
	share1 := &model.Share{ID: "share-1", Slug: "share-1", Name: "Share 1"}
	share2 := &model.Share{ID: "share-2", Slug: "share-2", Name: "Share 2"}
	_ = shareRepo.Create(ctx, share1)
	_ = shareRepo.Create(ctx, share2)

	// Create recipients for share1
	for i, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		r := &model.ShareRecipient{
			ID:      "r-" + string(rune('1'+i)),
			ShareID: "share-1",
			Email:   email,
		}
		_ = recipientRepo.Create(ctx, r)
	}

	// Create recipient for share2
	r := &model.ShareRecipient{
		ID:      "r-other",
		ShareID: "share-2",
		Email:   "other@example.com",
	}
	_ = recipientRepo.Create(ctx, r)

	// List recipients for share1
	recipients, err := recipientRepo.ListByShare(ctx, "share-1")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 3 {
		t.Errorf("expected 3 recipients for share-1, got %d", len(recipients))
	}

	// List recipients for share2
	recipients, err = recipientRepo.ListByShare(ctx, "share-2")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 1 {
		t.Errorf("expected 1 recipient for share-2, got %d", len(recipients))
	}
}

func TestRecipientRepository_ListByShare_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recipientRepo := repository.NewRecipientRepository(db.DB())
	ctx := context.Background()

	recipients, err := recipientRepo.ListByShare(ctx, "nonexistent-share")
	if err != nil {
		t.Fatalf("failed to list recipients: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients, got %d", len(recipients))
	}
}

func TestRecipientRepository_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	shareRepo := repository.NewShareRepository(db.DB())
	recipientRepo := repository.NewRecipientRepository(db.DB())
	ctx := context.Background()

	// Create a share
	share := &model.Share{ID: "share-del", Slug: "share-del", Name: "Delete Me"}
	_ = shareRepo.Create(ctx, share)

	// Create recipients
	for i, email := range []string{"a@example.com", "b@example.com"} {
		r := &model.ShareRecipient{
			ID:      "r-del-" + string(rune('1'+i)),
			ShareID: "share-del",
			Email:   email,
		}
		_ = recipientRepo.Create(ctx, r)
	}

	// Verify recipients exist
	recipients, _ := recipientRepo.ListByShare(ctx, "share-del")
	if len(recipients) != 2 {
		t.Fatalf("expected 2 recipients before delete, got %d", len(recipients))
	}

	// Delete the share — recipients should cascade
	err := shareRepo.Delete(ctx, "share-del")
	if err != nil {
		t.Fatalf("failed to delete share: %v", err)
	}

	// Verify recipients are gone
	recipients, err = recipientRepo.ListByShare(ctx, "share-del")
	if err != nil {
		t.Fatalf("failed to list recipients after delete: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients after cascade delete, got %d", len(recipients))
	}
}
