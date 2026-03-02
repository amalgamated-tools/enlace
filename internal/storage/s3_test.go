package storage

import (
	"testing"
)

func TestNewS3Storage_MissingBucket(t *testing.T) {
	cfg := S3Config{
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Region:    "us-east-1",
	}

	_, err := NewS3Storage(t.Context(), cfg)
	if err == nil {
		t.Error("expected error for missing bucket")
	}
	if err.Error() != "bucket name is required" {
		t.Errorf("expected 'bucket name is required', got %q", err.Error())
	}
}

func TestNewS3Storage_MissingAccessKey(t *testing.T) {
	cfg := S3Config{
		Bucket:    "test-bucket",
		SecretKey: "test-secret",
		Region:    "us-east-1",
	}

	_, err := NewS3Storage(t.Context(), cfg)
	if err == nil {
		t.Error("expected error for missing access key")
	}
	if err.Error() != "access key is required" {
		t.Errorf("expected 'access key is required', got %q", err.Error())
	}
}

func TestNewS3Storage_MissingSecretKey(t *testing.T) {
	cfg := S3Config{
		Bucket:    "test-bucket",
		AccessKey: "test-key",
		Region:    "us-east-1",
	}

	_, err := NewS3Storage(t.Context(), cfg)
	if err == nil {
		t.Error("expected error for missing secret key")
	}
	if err.Error() != "secret key is required" {
		t.Errorf("expected 'secret key is required', got %q", err.Error())
	}
}

func TestNewS3Storage_DefaultRegion(t *testing.T) {
	cfg := S3Config{
		Bucket:    "test-bucket",
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Endpoint:  "http://localhost:9000",
	}

	storage, err := NewS3Storage(t.Context(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage == nil {
		t.Fatal("expected non-nil storage")
	}
	if storage.bucket != "test-bucket" {
		t.Errorf("expected bucket 'test-bucket', got %s", storage.bucket)
	}
}

func TestNewS3Storage_WithEndpoint(t *testing.T) {
	cfg := S3Config{
		Endpoint:  "http://localhost:9000",
		Bucket:    "test-bucket",
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Region:    "us-west-2",
	}

	storage, err := NewS3Storage(t.Context(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestNewS3Storage_WithPathPrefix(t *testing.T) {
	cfg := S3Config{
		Endpoint:   "http://localhost:9000",
		Bucket:     "test-bucket",
		AccessKey:  "test-key",
		SecretKey:  "test-secret",
		Region:     "us-east-1",
		PathPrefix: "uploads/",
	}

	storage, err := NewS3Storage(t.Context(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.pathPrefix != "uploads/" {
		t.Errorf("expected pathPrefix 'uploads/', got %s", storage.pathPrefix)
	}
}

func TestS3Storage_FullKey(t *testing.T) {
	tests := []struct {
		name       string
		pathPrefix string
		key        string
		want       string
	}{
		{
			name:       "no prefix",
			pathPrefix: "",
			key:        "file.txt",
			want:       "file.txt",
		},
		{
			name:       "with prefix",
			pathPrefix: "uploads",
			key:        "file.txt",
			want:       "uploads/file.txt",
		},
		{
			name:       "with trailing slash prefix",
			pathPrefix: "uploads/",
			key:        "file.txt",
			want:       "uploads/file.txt",
		},
		{
			name:       "nested key with prefix",
			pathPrefix: "data",
			key:        "user/files/doc.pdf",
			want:       "data/user/files/doc.pdf",
		},
		{
			name:       "no prefix nested key",
			pathPrefix: "",
			key:        "user/files/doc.pdf",
			want:       "user/files/doc.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &S3Storage{pathPrefix: tt.pathPrefix}
			got := s.fullKey(tt.key)
			if got != tt.want {
				t.Errorf("fullKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestS3Storage_InterfaceCompliance(t *testing.T) {
	// Verify that S3Storage implements the Storage interface at compile time
	var _ Storage = (*S3Storage)(nil)
}
