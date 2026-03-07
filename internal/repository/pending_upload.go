package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// PendingUploadRepository provides persistence for direct-transfer upload intents.
type PendingUploadRepository struct {
	db *sql.DB
}

// NewPendingUploadRepository creates a new PendingUploadRepository instance.
func NewPendingUploadRepository(db *sql.DB) *PendingUploadRepository {
	return &PendingUploadRepository{db: db}
}

// Create inserts a new pending upload intent.
func (r *PendingUploadRepository) Create(ctx context.Context, upload *model.PendingUpload) error {
	upload.CreatedAt = time.Now()
	if upload.Status == "" {
		upload.Status = "pending"
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pending_uploads
		(id, file_id, share_id, uploader_id, filename, size, mime_type, storage_key, status, expires_at, created_at, finalized_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		upload.ID, upload.FileID, upload.ShareID, upload.UploaderID, upload.Filename, upload.Size, upload.MimeType,
		upload.StorageKey, upload.Status, upload.ExpiresAt, upload.CreatedAt, upload.FinalizedAt,
	)
	return err
}

// GetByID retrieves a pending upload by ID.
func (r *PendingUploadRepository) GetByID(ctx context.Context, id string) (*model.PendingUpload, error) {
	upload := &model.PendingUpload{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, file_id, share_id, uploader_id, filename, size, mime_type, storage_key, status, expires_at, created_at, finalized_at
		FROM pending_uploads
		WHERE id = ?`,
		id,
	).Scan(
		&upload.ID,
		&upload.FileID,
		&upload.ShareID,
		&upload.UploaderID,
		&upload.Filename,
		&upload.Size,
		&upload.MimeType,
		&upload.StorageKey,
		&upload.Status,
		&upload.ExpiresAt,
		&upload.CreatedAt,
		&upload.FinalizedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return upload, err
}

// Finalize marks an upload as finalized if it's still pending.
func (r *PendingUploadRepository) Finalize(ctx context.Context, id string) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE pending_uploads
		SET status = 'finalized', finalized_at = ?
		WHERE id = ? AND status = 'pending'`,
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

// ExpireStale marks expired pending uploads as expired.
func (r *PendingUploadRepository) ExpireStale(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE pending_uploads
		SET status = 'expired'
		WHERE status = 'pending' AND expires_at <= ?`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
