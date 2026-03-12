package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// SettingsRepository provides operations for the settings key-value store.
type SettingsRepository struct {
	db *sql.DB
}

// NewSettingsRepository creates a new SettingsRepository instance.
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// Get retrieves a setting value by key. Returns ErrNotFound if the key does not exist.
func (r *SettingsRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

// Set upserts a setting value. Inserts if absent, updates if present.
func (r *SettingsRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now(),
	)
	return err
}

// GetMultiple retrieves multiple settings by their keys in a single query.
// Returns a map of key->value for keys that exist. Missing keys are omitted.
func (r *SettingsRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(keys) == 0 {
		return result, nil
	}

	// Build WHERE key IN (?, ?, ...) with placeholders
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, key := range keys {
		placeholders[i] = "?"
		args[i] = key
	}

	query := `SELECT key, value FROM settings WHERE key IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// SetMultiple upserts multiple settings in a single transaction.
func (r *SettingsRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for key, value := range settings {
		if _, err := stmt.ExecContext(ctx, key, value, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Delete removes a setting by key. Returns ErrNotFound if the key does not exist.
func (r *SettingsRepository) Delete(ctx context.Context, key string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMultiple removes multiple settings by their keys in a single transaction.
func (r *SettingsRepository) DeleteMultiple(ctx context.Context, keys []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM settings WHERE key = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, key := range keys {
		if _, err := stmt.ExecContext(ctx, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}
