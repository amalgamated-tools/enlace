package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/amalgamated-tools/enlace/internal/model"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicate is returned when a unique constraint is violated.
var ErrDuplicate = errors.New("duplicate")

// UserRepository provides CRUD operations for users.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository instance.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user into the database.
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, display_name, is_admin, oidc_subject, oidc_issuer, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.PasswordHash, user.DisplayName, user.IsAdmin, user.OIDCSubject, user.OIDCIssuer, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// GetByID retrieves a user by their ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, is_admin, oidc_subject, oidc_issuer, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.IsAdmin, &user.OIDCSubject, &user.OIDCIssuer, &user.CreatedAt, &user.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return user, err
}

// GetByEmail retrieves a user by their email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, is_admin, oidc_subject, oidc_issuer, created_at, updated_at
		 FROM users WHERE email = ?`, email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.IsAdmin, &user.OIDCSubject, &user.OIDCIssuer, &user.CreatedAt, &user.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return user, err
}

// GetByOIDC retrieves a user by their OIDC issuer and subject.
func (r *UserRepository) GetByOIDC(ctx context.Context, issuer, subject string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, is_admin, oidc_subject, oidc_issuer, created_at, updated_at
		 FROM users WHERE oidc_issuer = ? AND oidc_subject = ?`, issuer, subject,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.IsAdmin, &user.OIDCSubject, &user.OIDCIssuer, &user.CreatedAt, &user.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return user, err
}

// Update modifies an existing user in the database.
func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	user.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET email = ?, password_hash = ?, display_name = ?, is_admin = ?, oidc_subject = ?, oidc_issuer = ?, updated_at = ?
		 WHERE id = ?`,
		user.Email, user.PasswordHash, user.DisplayName, user.IsAdmin, user.OIDCSubject, user.OIDCIssuer, user.UpdatedAt, user.ID,
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

// Delete removes a user from the database by their ID.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// List retrieves all users from the database, ordered by creation date descending.
func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, email, password_hash, display_name, is_admin, oidc_subject, oidc_issuer, created_at, updated_at
		 FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		user := &model.User{}
		if err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.IsAdmin, &user.OIDCSubject, &user.OIDCIssuer, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// Count returns the total number of users in the database.
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// EmailExists checks if a user with the given email address already exists.
func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&count)
	return count > 0, err
}
