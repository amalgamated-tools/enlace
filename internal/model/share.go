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
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (s *Share) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

func (s *Share) IsDownloadLimitReached() bool {
	if s.MaxDownloads == nil {
		return false
	}
	return s.DownloadCount >= *s.MaxDownloads
}

func (s *Share) IsViewLimitReached() bool {
	if s.MaxViews == nil {
		return false
	}
	return s.ViewCount >= *s.MaxViews
}

func (s *Share) HasPassword() bool {
	return s.PasswordHash != nil
}
