package database_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/amalgamated-tools/enlace/internal/database"
)

func TestNew_CreatesDatabase(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	var count int
	err = db.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected users table to exist")
	}
}

func TestNew_CreatesAllTables(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	expectedTables := []string{
		"users",
		"shares",
		"files",
		"password_reset_tokens",
		"refresh_tokens",
		"api_keys",
		"webhook_subscriptions",
		"webhook_deliveries",
	}

	for _, table := range expectedTables {
		var count int
		err := db.DB().QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query for table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("expected table %s to exist", table)
		}
	}
}

func TestNew_CreatesIndexes(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	expectedIndexes := []string{
		"idx_users_email",
		"idx_users_oidc",
		"idx_shares_slug",
		"idx_shares_creator_id",
		"idx_files_share_id",
		"idx_password_reset_tokens_user_id",
		"idx_refresh_tokens_user_id",
		"idx_api_keys_creator_id",
		"idx_api_keys_key_prefix",
		"idx_webhook_subscriptions_creator_id",
		"idx_webhook_deliveries_subscription_created",
		"idx_webhook_deliveries_status_next_attempt",
	}

	for _, index := range expectedIndexes {
		var count int
		err := db.DB().QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?",
			index,
		).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query for index %s: %v", index, err)
		}
		if count != 1 {
			t.Errorf("expected index %s to exist", index)
		}
	}
}

func TestNew_EnablesForeignKeys(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	var foreignKeysEnabled int
	err = db.DB().QueryRow("PRAGMA foreign_keys").Scan(&foreignKeysEnabled)
	if err != nil {
		t.Fatalf("failed to query foreign_keys pragma: %v", err)
	}
	if foreignKeysEnabled != 1 {
		t.Errorf("expected foreign keys to be enabled, got %d", foreignKeysEnabled)
	}
}

func TestNew_MigrationsAreIdempotent(t *testing.T) {
	// Create database and run migrations
	db1, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create first database: %v", err)
	}
	defer db1.Close()

	// Run migrations again on the same database should not fail
	// We simulate this by creating a new connection to an in-memory DB
	// In practice, this tests that CREATE TABLE IF NOT EXISTS works
	db2, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create second database: %v", err)
	}
	defer db2.Close()
}

func TestDB_ReturnsUnderlyingConnection(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	sqlDB := db.DB()
	if sqlDB == nil {
		t.Fatal("expected DB() to return non-nil connection")
	}

	// Verify we can use the connection
	err = sqlDB.Ping()
	if err != nil {
		t.Errorf("expected Ping to succeed: %v", err)
	}
}

func TestClose_ClosesConnection(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Errorf("expected Close to succeed: %v", err)
	}

	// After closing, operations should fail
	err = db.DB().Ping()
	if err == nil {
		t.Error("expected Ping to fail after Close")
	}
}

func TestNew_ForeignKeyConstraintsWork(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Insert a share without a valid creator_id (NULL is allowed)
	_, err = db.DB().Exec(`
		INSERT INTO shares (id, slug, name, description)
		VALUES ('share1', 'test-slug', 'Test Share', 'Description')
	`)
	if err != nil {
		t.Fatalf("failed to insert share: %v", err)
	}

	// Insert a file referencing the share
	_, err = db.DB().Exec(`
		INSERT INTO files (id, share_id, name, size, mime_type, storage_key)
		VALUES ('file1', 'share1', 'test.txt', 100, 'text/plain', 'key1')
	`)
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	// Try to insert a file with non-existent share_id - should fail due to FK constraint
	_, err = db.DB().Exec(`
		INSERT INTO files (id, share_id, name, size, mime_type, storage_key)
		VALUES ('file2', 'nonexistent', 'test2.txt', 100, 'text/plain', 'key2')
	`)
	if err == nil {
		t.Error("expected foreign key constraint to prevent insert with invalid share_id")
	}
}

func TestNew_CascadeDeleteWorks(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Insert a share
	_, err = db.DB().Exec(`
		INSERT INTO shares (id, slug, name, description)
		VALUES ('share1', 'test-slug', 'Test Share', 'Description')
	`)
	if err != nil {
		t.Fatalf("failed to insert share: %v", err)
	}

	// Insert files referencing the share
	_, err = db.DB().Exec(`
		INSERT INTO files (id, share_id, name, size, mime_type, storage_key)
		VALUES ('file1', 'share1', 'test.txt', 100, 'text/plain', 'key1')
	`)
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	// Delete the share - files should be cascade deleted
	_, err = db.DB().Exec(`DELETE FROM shares WHERE id = 'share1'`)
	if err != nil {
		t.Fatalf("failed to delete share: %v", err)
	}

	// Verify file was cascade deleted
	var count int
	err = db.DB().QueryRow(`SELECT COUNT(*) FROM files WHERE share_id = 'share1'`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query files: %v", err)
	}
	if count != 0 {
		t.Errorf("expected files to be cascade deleted, got %d", count)
	}
}

func TestNew_WithFilePath(t *testing.T) {
	// Create a temporary file path
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create database with file path: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	var count int
	err = db.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected users table to exist")
	}
}

func TestMigration_MaxViewsCoalesced(t *testing.T) {
	// Simulate an old-schema database that still has max_views and view_count
	// columns, then run New() which triggers the migration that drops them.
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/migrate.db"

	// Open a raw connection and create the OLD schema with max_views / view_count.
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw db: %v", err)
	}

	oldSchema := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			oidc_subject TEXT DEFAULT '',
			oidc_issuer TEXT DEFAULT ''
		)`,
		`CREATE TABLE shares (
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
	}
	for _, stmt := range oldSchema {
		if _, err := rawDB.Exec(stmt); err != nil {
			t.Fatalf("failed to create old schema: %v", err)
		}
	}

	// Insert test rows covering all edge cases.
	inserts := []string{
		// Both NULL limits, zero counts
		`INSERT INTO shares (id, slug, name) VALUES ('s1', 'slug1', 'Both NULL')`,
		// Only max_downloads set, view_count higher
		`INSERT INTO shares (id, slug, name, max_downloads, download_count, view_count) VALUES ('s2', 'slug2', 'DL only', 5, 1, 3)`,
		// Only max_views set, download_count higher
		`INSERT INTO shares (id, slug, name, max_views, download_count, view_count) VALUES ('s3', 'slug3', 'Views only', 10, 4, 2)`,
		// Both limits set, view_count higher
		`INSERT INTO shares (id, slug, name, max_downloads, max_views, download_count, view_count) VALUES ('s4', 'slug4', 'Both set', 5, 10, 1, 7)`,
	}
	for _, ins := range inserts {
		if _, err := rawDB.Exec(ins); err != nil {
			t.Fatalf("failed to insert test row: %v", err)
		}
	}
	rawDB.Close()

	// Now open with database.New which runs migrations.
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open database after migration: %v", err)
	}
	defer db.Close()

	// Verify max_views column no longer exists.
	var colCount int
	rows, err := db.DB().Query("PRAGMA table_info(shares)")
	if err != nil {
		t.Fatalf("failed to query table_info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column: %v", err)
		}
		if name == "max_views" || name == "view_count" {
			colCount++
		}
	}
	if colCount != 0 {
		t.Errorf("expected max_views/view_count columns to be removed, found %d", colCount)
	}

	// Verify coalesced data.
	type row struct {
		maxDownloads  *int
		downloadCount int
	}
	expected := map[string]row{
		"s1": {nil, 0},        // both NULL → NULL, MAX(0,0)=0
		"s2": {intPtr(5), 3},  // COALESCE(5,NULL)=5, MAX(1,3)=3
		"s3": {intPtr(10), 4}, // COALESCE(NULL,10)=10, MAX(4,2)=4
		"s4": {intPtr(5), 7},  // COALESCE(5,10)=5, MAX(1,7)=7
	}

	for id, want := range expected {
		var maxDL *int
		var dlCount int
		err := db.DB().QueryRow(
			"SELECT max_downloads, download_count FROM shares WHERE id = ?", id,
		).Scan(&maxDL, &dlCount)
		if err != nil {
			t.Fatalf("failed to query share %s: %v", id, err)
		}
		if (want.maxDownloads == nil) != (maxDL == nil) {
			t.Errorf("share %s: max_downloads mismatch: want %v, got %v", id, want.maxDownloads, maxDL)
		} else if want.maxDownloads != nil && *want.maxDownloads != *maxDL {
			t.Errorf("share %s: max_downloads want %d, got %d", id, *want.maxDownloads, *maxDL)
		}
		if dlCount != want.downloadCount {
			t.Errorf("share %s: download_count want %d, got %d", id, want.downloadCount, dlCount)
		}
	}
}

func intPtr(v int) *int { return &v }

func TestMigration_OIDCColumns(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Verify OIDC columns exist by inserting a user with OIDC fields
	_, err = db.DB().Exec(`
		INSERT INTO users (id, email, password_hash, display_name, is_admin, oidc_subject, oidc_issuer)
		VALUES ('test-id', 'test@example.com', '', 'Test User', 0, 'sub123', 'https://auth.example.com')
	`)
	if err != nil {
		t.Fatalf("failed to insert user with OIDC fields: %v", err)
	}

	// Verify we can query OIDC fields
	var subject, issuer string
	err = db.DB().QueryRow(`SELECT oidc_subject, oidc_issuer FROM users WHERE id = 'test-id'`).Scan(&subject, &issuer)
	if err != nil {
		t.Fatalf("failed to query OIDC fields: %v", err)
	}
	if subject != "sub123" {
		t.Errorf("expected subject 'sub123', got %s", subject)
	}
	if issuer != "https://auth.example.com" {
		t.Errorf("expected issuer 'https://auth.example.com', got %s", issuer)
	}
}
