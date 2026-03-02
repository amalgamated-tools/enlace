package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// ShareRepository provides CRUD operations for shares.
type ShareRepository struct {
	db *sql.DB
}

// NewShareRepository creates a new ShareRepository instance.
func NewShareRepository(db *sql.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

// Create inserts a new share into the database.
func (r *ShareRepository) Create(ctx context.Context, share *model.Share) error {
	now := time.Now()
	share.CreatedAt = now
	share.UpdatedAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO shares (id, creator_id, slug, name, description, password_hash, expires_at, max_downloads, download_count, max_views, view_count, is_reverse_share, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		share.ID, share.CreatorID, share.Slug, share.Name, share.Description, share.PasswordHash,
		share.ExpiresAt, share.MaxDownloads, share.DownloadCount, share.MaxViews, share.ViewCount,
		share.IsReverseShare, share.CreatedAt, share.UpdatedAt,
	)
	return err
}

// GetByID retrieves a share by its ID.
func (r *ShareRepository) GetByID(ctx context.Context, id string) (*model.Share, error) {
	share := &model.Share{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, creator_id, slug, name, description, password_hash, expires_at, max_downloads, download_count, max_views, view_count, is_reverse_share, created_at, updated_at
		 FROM shares WHERE id = ?`, id,
	).Scan(
		&share.ID, &share.CreatorID, &share.Slug, &share.Name, &share.Description,
		&share.PasswordHash, &share.ExpiresAt, &share.MaxDownloads, &share.DownloadCount,
		&share.MaxViews, &share.ViewCount, &share.IsReverseShare, &share.CreatedAt, &share.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return share, err
}

// GetBySlug retrieves a share by its URL slug.
func (r *ShareRepository) GetBySlug(ctx context.Context, slug string) (*model.Share, error) {
	share := &model.Share{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, creator_id, slug, name, description, password_hash, expires_at, max_downloads, download_count, max_views, view_count, is_reverse_share, created_at, updated_at
		 FROM shares WHERE slug = ?`, slug,
	).Scan(
		&share.ID, &share.CreatorID, &share.Slug, &share.Name, &share.Description,
		&share.PasswordHash, &share.ExpiresAt, &share.MaxDownloads, &share.DownloadCount,
		&share.MaxViews, &share.ViewCount, &share.IsReverseShare, &share.CreatedAt, &share.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return share, err
}

// Update modifies an existing share in the database.
func (r *ShareRepository) Update(ctx context.Context, share *model.Share) error {
	share.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE shares SET creator_id = ?, slug = ?, name = ?, description = ?, password_hash = ?, expires_at = ?, max_downloads = ?, download_count = ?, max_views = ?, view_count = ?, is_reverse_share = ?, updated_at = ?
		 WHERE id = ?`,
		share.CreatorID, share.Slug, share.Name, share.Description, share.PasswordHash,
		share.ExpiresAt, share.MaxDownloads, share.DownloadCount, share.MaxViews, share.ViewCount,
		share.IsReverseShare, share.UpdatedAt, share.ID,
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

// Delete removes a share from the database by its ID.
func (r *ShareRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM shares WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByCreator retrieves all shares created by a specific user, ordered by creation date descending.
func (r *ShareRepository) ListByCreator(ctx context.Context, creatorID string) ([]*model.Share, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, creator_id, slug, name, description, password_hash, expires_at, max_downloads, download_count, max_views, view_count, is_reverse_share, created_at, updated_at
		 FROM shares WHERE creator_id = ? ORDER BY created_at DESC`, creatorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []*model.Share
	for rows.Next() {
		share := &model.Share{}
		if err := rows.Scan(
			&share.ID, &share.CreatorID, &share.Slug, &share.Name, &share.Description,
			&share.PasswordHash, &share.ExpiresAt, &share.MaxDownloads, &share.DownloadCount,
			&share.MaxViews, &share.ViewCount, &share.IsReverseShare, &share.CreatedAt, &share.UpdatedAt,
		); err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, rows.Err()
}

// SlugExists checks if a share with the given slug already exists.
func (r *ShareRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shares WHERE slug = ?`, slug).Scan(&count)
	return count > 0, err
}

// IncrementDownloadCount atomically increments the download counter for a share.
func (r *ShareRepository) IncrementDownloadCount(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE shares SET download_count = download_count + 1, updated_at = ? WHERE id = ?`,
		time.Now(), id,
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

// IncrementViewCount atomically increments the view counter for a share.
func (r *ShareRepository) IncrementViewCount(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE shares SET view_count = view_count + 1, updated_at = ? WHERE id = ?`,
		time.Now(), id,
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
