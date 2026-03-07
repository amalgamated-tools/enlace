package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

const (
	PendingUploadStatusPending   = "pending"
	PendingUploadStatusFinalized = "finalized"
	PendingUploadStatusExpired   = "expired"
)

var ErrPendingUploadConflict = errors.New("pending upload conflict")

// PendingUploadRepository provides persistence for replay-safe direct uploads.
type PendingUploadRepository struct {
	db *sql.DB
}

func NewPendingUploadRepository(db *sql.DB) *PendingUploadRepository {
	return &PendingUploadRepository{db: db}
}

func (r *PendingUploadRepository) Create(ctx context.Context, upload *model.PendingUpload) error {
	upload.CreatedAt = time.Now()
	if upload.Status == "" {
		upload.Status = PendingUploadStatusPending
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pending_uploads (id, file_id, share_id, uploader_id, filename, size, mime_type, storage_key, status, expires_at, created_at, finalized_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		upload.ID, upload.FileID, upload.ShareID, upload.UploaderID, upload.Filename, upload.Size, upload.MimeType,
		upload.StorageKey, upload.Status, upload.ExpiresAt, upload.CreatedAt, upload.FinalizedAt,
	)
	return err
}

func (r *PendingUploadRepository) GetByID(ctx context.Context, id string) (*model.PendingUpload, error) {
	upload := &model.PendingUpload{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, file_id, share_id, uploader_id, filename, size, mime_type, storage_key, status, expires_at, created_at, finalized_at
		 FROM pending_uploads WHERE id = ?`,
		id,
	).Scan(
		&upload.ID, &upload.FileID, &upload.ShareID, &upload.UploaderID, &upload.Filename, &upload.Size, &upload.MimeType,
		&upload.StorageKey, &upload.Status, &upload.ExpiresAt, &upload.CreatedAt, &upload.FinalizedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return upload, err
}

func (r *PendingUploadRepository) Finalize(ctx context.Context, uploadID string, file *model.File) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM pending_uploads WHERE id = ?`, uploadID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != PendingUploadStatusPending {
		return ErrPendingUploadConflict
	}

	finalizedAt := time.Now()
	result, err := tx.ExecContext(ctx,
		`UPDATE pending_uploads
		 SET status = ?, finalized_at = ?
		 WHERE id = ? AND status = ?`,
		PendingUploadStatusFinalized, finalizedAt, uploadID, PendingUploadStatusPending,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrPendingUploadConflict
	}

	file.CreatedAt = finalizedAt
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO files (id, share_id, uploader_id, name, size, mime_type, storage_key, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		file.ID, file.ShareID, file.UploaderID, file.Name, file.Size, file.MimeType, file.StorageKey, file.CreatedAt,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PendingUploadRepository) ExpireStale(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE pending_uploads
		 SET status = ?
		 WHERE status = ? AND expires_at <= ?`,
		PendingUploadStatusExpired, PendingUploadStatusPending, now,
	)
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}
