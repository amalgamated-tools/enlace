package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// TOTPRepository provides CRUD operations for TOTP and recovery code data.
type TOTPRepository struct {
	db *sql.DB
}

// NewTOTPRepository creates a new TOTPRepository instance.
func NewTOTPRepository(db *sql.DB) *TOTPRepository {
	return &TOTPRepository{db: db}
}

// UpsertTOTP creates or replaces a user's TOTP configuration.
func (r *TOTPRepository) UpsertTOTP(ctx context.Context, totp *model.UserTOTP) error {
	now := time.Now()
	totp.CreatedAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_totp (user_id, secret, enabled, verified_at, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET secret = excluded.secret, enabled = excluded.enabled, verified_at = excluded.verified_at`,
		totp.UserID, totp.Secret, totp.Enabled, totp.VerifiedAt, totp.CreatedAt,
	)
	return err
}

// GetByUserID retrieves the TOTP configuration for a user.
func (r *TOTPRepository) GetByUserID(ctx context.Context, userID string) (*model.UserTOTP, error) {
	totp := &model.UserTOTP{}
	err := r.db.QueryRowContext(ctx,
		`SELECT user_id, secret, enabled, verified_at, created_at FROM user_totp WHERE user_id = ?`, userID,
	).Scan(&totp.UserID, &totp.Secret, &totp.Enabled, &totp.VerifiedAt, &totp.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return totp, err
}

// Enable marks the TOTP as verified and enabled.
func (r *TOTPRepository) Enable(ctx context.Context, userID string) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE user_totp SET enabled = 1, verified_at = ? WHERE user_id = ?`, now, userID,
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

// Delete removes a user's TOTP configuration.
func (r *TOTPRepository) Delete(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_totp WHERE user_id = ?`, userID)
	return err
}

// SaveRecoveryCodes stores a batch of hashed recovery codes for a user.
// This deletes any existing codes first.
func (r *TOTPRepository) SaveRecoveryCodes(ctx context.Context, codes []*model.RecoveryCode) error {
	if len(codes) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Delete existing codes for this user
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE user_id = ?`, codes[0].UserID); err != nil {
		return err
	}

	// Insert new codes
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO user_recovery_codes (id, user_id, code_hash, created_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, code := range codes {
		code.CreatedAt = now
		if _, err := stmt.ExecContext(ctx, code.ID, code.UserID, code.CodeHash, code.CreatedAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetRecoveryCodes retrieves all recovery codes for a user.
func (r *TOTPRepository) GetRecoveryCodes(ctx context.Context, userID string) ([]*model.RecoveryCode, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, code_hash, created_at FROM user_recovery_codes WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []*model.RecoveryCode
	for rows.Next() {
		code := &model.RecoveryCode{}
		if err := rows.Scan(&code.ID, &code.UserID, &code.CodeHash, &code.CreatedAt); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

// DeleteRecoveryCode deletes a single recovery code by ID (after use).
// Returns ErrNotFound if the code does not exist (e.g., already consumed).
func (r *TOTPRepository) DeleteRecoveryCode(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRecoveryCodesByUser deletes all recovery codes for a user.
func (r *TOTPRepository) DeleteRecoveryCodesByUser(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE user_id = ?`, userID)
	return err
}

// EnableAndSaveRecoveryCodes atomically enables 2FA and stores recovery codes in a single transaction.
// This prevents a state where 2FA is enabled but no recovery codes exist.
func (r *TOTPRepository) EnableAndSaveRecoveryCodes(ctx context.Context, userID string, codes []*model.RecoveryCode) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Enable TOTP
	now := time.Now()
	result, err := tx.ExecContext(ctx,
		`UPDATE user_totp SET enabled = 1, verified_at = ? WHERE user_id = ?`, now, userID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	// Delete any existing recovery codes
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE user_id = ?`, userID); err != nil {
		return err
	}

	// Insert new recovery codes
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO user_recovery_codes (id, user_id, code_hash, created_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, code := range codes {
		code.CreatedAt = now
		if _, err := stmt.ExecContext(ctx, code.ID, code.UserID, code.CodeHash, code.CreatedAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}
