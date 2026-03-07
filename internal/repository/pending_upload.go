package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// PendingUploadRepository provides CRUD operations for pending direct uploads.
type PendingUploadRepository struct {
	db *sql.DB
}

// NewPendingUploadRepository creates a new PendingUploadRepository instance.
func NewPendingUploadRepository(db *sql.DB) *PendingUploadRepository {
	return &PendingUploadRepository{db: db}
}

// Create inserts a new pending upload record.
func (r *PendingUploadRepository) Create(ctx context.Context, pu *model.PendingUpload) error {
	pu.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pending_uploads (id, file_id, share_id, uploader_id, filename, size, mime_type, storage_key, status, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pu.ID, pu.FileID, pu.ShareID, pu.UploaderID, pu.Filename, pu.Size, pu.MimeType, pu.StorageKey, pu.Status, pu.ExpiresAt, pu.CreatedAt,
	)
	return err
}

// GetByID retrieves a pending upload by its ID.
func (r *PendingUploadRepository) GetByID(ctx context.Context, id string) (*model.PendingUpload, error) {
	pu := &model.PendingUpload{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, file_id, share_id, uploader_id, filename, size, mime_type, storage_key, status, expires_at, created_at, finalized_at
		 FROM pending_uploads WHERE id = ?`, id,
	).Scan(&pu.ID, &pu.FileID, &pu.ShareID, &pu.UploaderID, &pu.Filename, &pu.Size, &pu.MimeType, &pu.StorageKey, &pu.Status, &pu.ExpiresAt, &pu.CreatedAt, &pu.FinalizedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return pu, err
}

// Finalize atomically transitions a pending upload from "pending" to "finalized".
// Returns ErrNotFound if the upload does not exist or is not in "pending" status.
func (r *PendingUploadRepository) Finalize(ctx context.Context, id string) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE pending_uploads SET status = 'finalized', finalized_at = ? WHERE id = ? AND status = 'pending'`,
		now, id,
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

// ExpireStale marks pending uploads that have passed their expiry as "expired"
// and returns the number of rows affected.
func (r *PendingUploadRepository) ExpireStale(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE pending_uploads SET status = 'expired' WHERE status = 'pending' AND expires_at < ?`,
		time.Now(),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
