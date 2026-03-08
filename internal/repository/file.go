package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// FileRepository provides CRUD operations for files.
type FileRepository struct {
	db *sql.DB
}

// NewFileRepository creates a new FileRepository instance.
func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

// Create inserts a new file into the database.
func (r *FileRepository) Create(ctx context.Context, file *model.File) error {
	file.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO files (id, share_id, uploader_id, name, size, mime_type, storage_key, encryption_iv, encrypted_metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		file.ID, file.ShareID, file.UploaderID, file.Name, file.Size, file.MimeType, file.StorageKey, file.EncryptionIV, file.EncryptedMetadata, file.CreatedAt,
	)
	return err
}

// GetByID retrieves a file by its ID.
func (r *FileRepository) GetByID(ctx context.Context, id string) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, share_id, uploader_id, name, size, mime_type, storage_key, encryption_iv, encrypted_metadata, created_at
		 FROM files WHERE id = ?`, id,
	).Scan(&file.ID, &file.ShareID, &file.UploaderID, &file.Name, &file.Size, &file.MimeType, &file.StorageKey, &file.EncryptionIV, &file.EncryptedMetadata, &file.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return file, err
}

// Delete removes a file from the database by its ID.
func (r *FileRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByShare retrieves all files for a specific share, ordered by creation date ascending.
func (r *FileRepository) ListByShare(ctx context.Context, shareID string) ([]*model.File, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, share_id, uploader_id, name, size, mime_type, storage_key, encryption_iv, encrypted_metadata, created_at
		 FROM files WHERE share_id = ? ORDER BY created_at ASC`, shareID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*model.File
	for rows.Next() {
		file := &model.File{}
		if err := rows.Scan(&file.ID, &file.ShareID, &file.UploaderID, &file.Name, &file.Size, &file.MimeType, &file.StorageKey, &file.EncryptionIV, &file.EncryptedMetadata, &file.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

// DeleteByShare removes all files for a specific share.
func (r *FileRepository) DeleteByShare(ctx context.Context, shareID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM files WHERE share_id = ?`, shareID)
	return err
}

// GetStorageKeysByShare retrieves all storage keys for files in a specific share.
// This is useful for cleanup operations when deleting a share.
func (r *FileRepository) GetStorageKeysByShare(ctx context.Context, shareID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT storage_key FROM files WHERE share_id = ?`, shareID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
