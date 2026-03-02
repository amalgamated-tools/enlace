package model

import "time"

// UserTOTP represents a user's TOTP two-factor authentication configuration.
type UserTOTP struct {
	UserID     string
	Secret     string
	Enabled    bool
	VerifiedAt *time.Time
	CreatedAt  time.Time
}

// RecoveryCode represents a hashed recovery code for a user.
type RecoveryCode struct {
	ID        string
	UserID    string
	CodeHash  string
	CreatedAt time.Time
}
