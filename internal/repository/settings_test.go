package repository_test

import (
	"context"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/repository"
)

func TestSettingsRepository_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	err := repo.Set(ctx, "storage_type", "s3")
	if err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}

	val, err := repo.Get(ctx, "storage_type")
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}
	if val != "s3" {
		t.Errorf("expected value 's3', got '%s'", val)
	}
}

func TestSettingsRepository_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSettingsRepository_Set_Upsert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	// Set initial value
	err := repo.Set(ctx, "storage_type", "local")
	if err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}

	// Overwrite with new value
	err = repo.Set(ctx, "storage_type", "s3")
	if err != nil {
		t.Fatalf("failed to upsert setting: %v", err)
	}

	val, err := repo.Get(ctx, "storage_type")
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}
	if val != "s3" {
		t.Errorf("expected value 's3' after upsert, got '%s'", val)
	}
}

func TestSettingsRepository_GetMultiple(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	// Set some values
	_ = repo.Set(ctx, "storage_type", "s3")
	_ = repo.Set(ctx, "s3_bucket", "my-bucket")

	// Request existing and non-existing keys
	result, err := repo.GetMultiple(ctx, []string{"storage_type", "s3_bucket", "nonexistent"})
	if err != nil {
		t.Fatalf("failed to get multiple settings: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	if result["storage_type"] != "s3" {
		t.Errorf("expected storage_type 's3', got '%s'", result["storage_type"])
	}
	if result["s3_bucket"] != "my-bucket" {
		t.Errorf("expected s3_bucket 'my-bucket', got '%s'", result["s3_bucket"])
	}
	if _, ok := result["nonexistent"]; ok {
		t.Error("expected nonexistent key to be absent from results")
	}
}

func TestSettingsRepository_GetMultiple_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	result, err := repo.GetMultiple(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("failed to get multiple settings: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestSettingsRepository_SetMultiple(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	settings := map[string]string{
		"storage_type": "s3",
		"s3_bucket":    "my-bucket",
		"s3_region":    "us-east-1",
	}

	err := repo.SetMultiple(ctx, settings)
	if err != nil {
		t.Fatalf("failed to set multiple settings: %v", err)
	}

	// Verify all were saved
	for key, expected := range settings {
		val, err := repo.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get setting %s: %v", key, err)
		}
		if val != expected {
			t.Errorf("expected %s = '%s', got '%s'", key, expected, val)
		}
	}
}

func TestSettingsRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	_ = repo.Set(ctx, "storage_type", "s3")

	err := repo.Delete(ctx, "storage_type")
	if err != nil {
		t.Fatalf("failed to delete setting: %v", err)
	}

	_, err = repo.Get(ctx, "storage_type")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestSettingsRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSettingsRepository_DeleteMultiple(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSettingsRepository(db.DB())
	ctx := context.Background()

	// Set some values
	_ = repo.Set(ctx, "storage_type", "s3")
	_ = repo.Set(ctx, "s3_bucket", "my-bucket")
	_ = repo.Set(ctx, "s3_region", "us-east-1")

	// Delete two of them
	err := repo.DeleteMultiple(ctx, []string{"storage_type", "s3_bucket"})
	if err != nil {
		t.Fatalf("failed to delete multiple settings: %v", err)
	}

	// Verify deleted
	_, err = repo.Get(ctx, "storage_type")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for storage_type, got %v", err)
	}
	_, err = repo.Get(ctx, "s3_bucket")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for s3_bucket, got %v", err)
	}

	// Verify remaining key still exists
	val, err := repo.Get(ctx, "s3_region")
	if err != nil {
		t.Fatalf("failed to get remaining setting: %v", err)
	}
	if val != "us-east-1" {
		t.Errorf("expected s3_region 'us-east-1', got '%s'", val)
	}
}
