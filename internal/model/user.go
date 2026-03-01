package model

import "time"

type User struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	IsAdmin      bool
	OIDCSubject  string // OIDC "sub" claim
	OIDCIssuer   string // OIDC issuer URL
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
