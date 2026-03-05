package model

import "time"

// APIKey stores metadata for a scoped automation key.
type APIKey struct {
	ID         string
	CreatorID  string
	Name       string
	KeyPrefix  string
	KeyHash    string
	Scopes     []string
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// IsRevoked returns true when the key has been revoked.
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}
