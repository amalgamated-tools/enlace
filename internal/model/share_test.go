package model

import (
	"testing"
	"time"
)

func TestShare_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "nil expiry - not expired",
			expiresAt: nil,
			want:      false,
		},
		{
			name:      "future expiry - not expired",
			expiresAt: timePtr(time.Now().Add(1 * time.Hour)),
			want:      false,
		},
		{
			name:      "past expiry - expired",
			expiresAt: timePtr(time.Now().Add(-1 * time.Hour)),
			want:      true,
		},
		{
			name:      "far future expiry - not expired",
			expiresAt: timePtr(time.Now().Add(365 * 24 * time.Hour)),
			want:      false,
		},
		{
			name:      "just expired",
			expiresAt: timePtr(time.Now().Add(-1 * time.Second)),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Share{ExpiresAt: tt.expiresAt}
			if got := s.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShare_IsDownloadLimitReached(t *testing.T) {
	tests := []struct {
		name          string
		maxDownloads  *int
		downloadCount int
		want          bool
	}{
		{
			name:          "nil limit - not reached",
			maxDownloads:  nil,
			downloadCount: 100,
			want:          false,
		},
		{
			name:          "under limit",
			maxDownloads:  intPtr(10),
			downloadCount: 5,
			want:          false,
		},
		{
			name:          "at limit",
			maxDownloads:  intPtr(10),
			downloadCount: 10,
			want:          true,
		},
		{
			name:          "over limit",
			maxDownloads:  intPtr(10),
			downloadCount: 15,
			want:          true,
		},
		{
			name:          "zero limit zero downloads",
			maxDownloads:  intPtr(0),
			downloadCount: 0,
			want:          true,
		},
		{
			name:          "zero downloads under limit",
			maxDownloads:  intPtr(5),
			downloadCount: 0,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Share{
				MaxDownloads:  tt.maxDownloads,
				DownloadCount: tt.downloadCount,
			}
			if got := s.IsDownloadLimitReached(); got != tt.want {
				t.Errorf("IsDownloadLimitReached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShare_HasPassword(t *testing.T) {
	tests := []struct {
		name         string
		passwordHash *string
		want         bool
	}{
		{
			name:         "nil password - no password",
			passwordHash: nil,
			want:         false,
		},
		{
			name:         "has password hash",
			passwordHash: strPtr("$2a$12$somehash"),
			want:         true,
		},
		{
			name:         "empty string password hash",
			passwordHash: strPtr(""),
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Share{PasswordHash: tt.passwordHash}
			if got := s.HasPassword(); got != tt.want {
				t.Errorf("HasPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions for creating pointers to primitives
func timePtr(t time.Time) *time.Time { return &t }
func intPtr(i int) *int              { return &i }
func strPtr(s string) *string        { return &s }
