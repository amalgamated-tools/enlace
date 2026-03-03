package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// RecipientRepository provides CRUD operations for share recipients.
type RecipientRepository struct {
	db *sql.DB
}

// NewRecipientRepository creates a new RecipientRepository instance.
func NewRecipientRepository(db *sql.DB) *RecipientRepository {
	return &RecipientRepository{db: db}
}

// Create inserts a new share recipient into the database.
func (r *RecipientRepository) Create(ctx context.Context, recipient *model.ShareRecipient) error {
	recipient.SentAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO share_recipients (id, share_id, email, sent_at) VALUES (?, ?, ?, ?)`,
		recipient.ID, recipient.ShareID, recipient.Email, recipient.SentAt,
	)
	return err
}

// ListByShare retrieves all recipients for a given share, ordered by sent_at descending.
func (r *RecipientRepository) ListByShare(ctx context.Context, shareID string) ([]*model.ShareRecipient, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, share_id, email, sent_at FROM share_recipients WHERE share_id = ? ORDER BY sent_at DESC`,
		shareID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []*model.ShareRecipient
	for rows.Next() {
		recipient := &model.ShareRecipient{}
		if err := rows.Scan(&recipient.ID, &recipient.ShareID, &recipient.Email, &recipient.SentAt); err != nil {
			return nil, err
		}
		recipients = append(recipients, recipient)
	}
	return recipients, rows.Err()
}
