package model

import "time"

type Share struct {
	ID             string
	CreatorID      *string
	Slug           string
	Name           string
	Description    string
	PasswordHash   *string
	ExpiresAt      *time.Time
	MaxDownloads   *int
	DownloadCount  int
	MaxViews       *int
	ViewCount      int
	IsReverseShare bool
	IsE2EEncrypted bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsExpired reports whether the share has passed its expiration time.
func (s *Share) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

// IsDownloadLimitReached reports whether the share has reached its download limit.
func (s *Share) IsDownloadLimitReached() bool {
	if s.MaxDownloads == nil {
		return false
	}
	return s.DownloadCount >= *s.MaxDownloads
}

// IsViewLimitReached reports whether the share has reached its view limit.
func (s *Share) IsViewLimitReached() bool {
	if s.MaxViews == nil {
		return false
	}
	return s.ViewCount >= *s.MaxViews
}

// HasPassword reports whether the share requires a password.
func (s *Share) HasPassword() bool {
	return s.PasswordHash != nil
}
