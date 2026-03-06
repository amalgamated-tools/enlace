package repository

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// APIKeyRepository provides CRUD operations for API keys.
type APIKeyRepository struct {
	db *sql.DB
}

// NewAPIKeyRepository creates a new APIKeyRepository.
func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create inserts a new API key record.
func (r *APIKeyRepository) Create(ctx context.Context, key *model.APIKey) error {
	now := time.Now()
	key.CreatedAt = now
	key.UpdatedAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, creator_id, name, key_prefix, key_hash, scopes, revoked_at, last_used_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.CreatorID, key.Name, key.KeyPrefix, key.KeyHash, encodeScopes(key.Scopes), key.RevokedAt, key.LastUsedAt, key.CreatedAt, key.UpdatedAt,
	)
	return err
}

// GetByID retrieves an API key by its ID.
func (r *APIKeyRepository) GetByID(ctx context.Context, id string) (*model.APIKey, error) {
	key := &model.APIKey{}
	var scopes string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, creator_id, name, key_prefix, key_hash, scopes, revoked_at, last_used_at, created_at, updated_at
		 FROM api_keys WHERE id = ?`,
		id,
	).Scan(
		&key.ID, &key.CreatorID, &key.Name, &key.KeyPrefix, &key.KeyHash, &scopes, &key.RevokedAt, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	key.Scopes = decodeScopes(scopes)
	return key, nil
}

// GetByHash retrieves an API key by key hash.
func (r *APIKeyRepository) GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error) {
	key := &model.APIKey{}
	var scopes string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, creator_id, name, key_prefix, key_hash, scopes, revoked_at, last_used_at, created_at, updated_at
		 FROM api_keys WHERE key_hash = ?`,
		keyHash,
	).Scan(
		&key.ID, &key.CreatorID, &key.Name, &key.KeyPrefix, &key.KeyHash, &scopes, &key.RevokedAt, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	key.Scopes = decodeScopes(scopes)
	return key, nil
}

// ListByCreator retrieves API keys for the creator ordered by creation date descending.
func (r *APIKeyRepository) ListByCreator(ctx context.Context, creatorID string) ([]*model.APIKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, creator_id, name, key_prefix, key_hash, scopes, revoked_at, last_used_at, created_at, updated_at
		 FROM api_keys WHERE creator_id = ? ORDER BY created_at DESC`,
		creatorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*model.APIKey
	for rows.Next() {
		key := &model.APIKey{}
		var scopes string
		if err := rows.Scan(
			&key.ID, &key.CreatorID, &key.Name, &key.KeyPrefix, &key.KeyHash, &scopes, &key.RevokedAt, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt,
		); err != nil {
			return nil, err
		}
		key.Scopes = decodeScopes(scopes)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// Revoke sets revoked_at for an API key.
func (r *APIKeyRepository) Revoke(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = ?, updated_at = ? WHERE id = ? AND revoked_at IS NULL`,
		time.Now(), time.Now(), id,
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

// TouchLastUsed updates the last_used_at timestamp.
func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ?, updated_at = ? WHERE id = ?`,
		at, at, id,
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

func encodeScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	clone := append([]string(nil), scopes...)
	sort.Strings(clone)
	out := make([]string, 0, len(clone))
	seen := make(map[string]struct{}, len(clone))
	for _, scope := range clone {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return strings.Join(out, ",")
}

func decodeScopes(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
