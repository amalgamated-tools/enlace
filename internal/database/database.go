package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Database wraps a SQL database connection with migration support.
type Database struct {
	db *sql.DB
}

// New creates a new Database instance, running migrations if needed.
// Use ":memory:" for an in-memory database or a file path for persistent storage.
func New(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Database{db: db}, nil
}

// DB returns the underlying sql.DB connection.
func (d *Database) DB() *sql.DB {
	return d.db
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

func runMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE TABLE IF NOT EXISTS shares (
			id TEXT PRIMARY KEY,
			creator_id TEXT,
			slug TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			password_hash TEXT,
			expires_at DATETIME,
			max_downloads INTEGER,
			download_count INTEGER NOT NULL DEFAULT 0,
			max_views INTEGER,
			view_count INTEGER NOT NULL DEFAULT 0,
			is_reverse_share INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_shares_slug ON shares(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_shares_creator_id ON shares(creator_id)`,
		`CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			share_id TEXT NOT NULL,
			uploader_id TEXT,
			name TEXT NOT NULL,
			size INTEGER NOT NULL,
			mime_type TEXT NOT NULL,
			storage_key TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (share_id) REFERENCES shares(id) ON DELETE CASCADE,
			FOREIGN KEY (uploader_id) REFERENCES users(id) ON DELETE SET NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_files_share_id ON files(share_id)`,
		`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id)`,
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id)`,
		`CREATE TABLE IF NOT EXISTS share_recipients (
			id TEXT PRIMARY KEY,
			share_id TEXT NOT NULL,
			email TEXT NOT NULL,
			sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (share_id) REFERENCES shares(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_share_recipients_share_id ON share_recipients(share_id)`,
		`CREATE TABLE IF NOT EXISTS user_totp (
			user_id TEXT PRIMARY KEY,
			secret TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 0,
			verified_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS user_recovery_codes (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			code_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_recovery_codes_user_id ON user_recovery_codes(user_id)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			creator_id TEXT NOT NULL,
			name TEXT NOT NULL,
			key_prefix TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			scopes TEXT NOT NULL DEFAULT '',
			revoked_at DATETIME,
			last_used_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_creator_id ON api_keys(creator_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix)`,
		`CREATE TABLE IF NOT EXISTS webhook_subscriptions (
			id TEXT PRIMARY KEY,
			creator_id TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			secret_enc TEXT NOT NULL,
			events TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_creator_id ON webhook_subscriptions(creator_id)`,
		`CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id TEXT PRIMARY KEY,
			subscription_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			event_id TEXT NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			attempt INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			status_code INTEGER,
			next_attempt_at DATETIME,
			delivered_at DATETIME,
			error TEXT NOT NULL DEFAULT '',
			request_body TEXT NOT NULL,
			response_body TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (subscription_id) REFERENCES webhook_subscriptions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_subscription_created ON webhook_deliveries(subscription_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status_next_attempt ON webhook_deliveries(status, next_attempt_at)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return err
		}
	}

	// OIDC columns for users (idempotent: only add if missing)
	if !columnExists(db, "users", "oidc_subject") {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN oidc_subject TEXT DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "users", "oidc_issuer") {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN oidc_issuer TEXT DEFAULT ''`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_oidc ON users(oidc_issuer, oidc_subject)`); err != nil {
		return err
	}

	// E2E encryption columns (idempotent: only add if missing)
	if !columnExists(db, "shares", "is_e2e_encrypted") {
		if _, err := db.Exec(`ALTER TABLE shares ADD COLUMN is_e2e_encrypted INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "files", "encryption_iv") {
		if _, err := db.Exec(`ALTER TABLE files ADD COLUMN encryption_iv TEXT DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "files", "encrypted_metadata") {
		if _, err := db.Exec(`ALTER TABLE files ADD COLUMN encrypted_metadata TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	return nil
}

// columnExists checks whether a column exists on a table using PRAGMA table_info.
func columnExists(db *sql.DB, table, column string) bool {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}
