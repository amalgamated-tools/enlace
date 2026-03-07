package handler

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type directUploadFinalizeClaims struct {
	UploadID   string `json:"upload_id"`
	ShareID    string `json:"share_id"`
	UploaderID string `json:"uploader_id,omitempty"`
	StorageKey string `json:"storage_key"`
	jwt.RegisteredClaims
}

func generateDirectUploadFinalizeToken(secret []byte, uploadID string, shareID string, uploaderID string, storageKey string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := &directUploadFinalizeClaims{
		UploadID:   uploadID,
		ShareID:    shareID,
		UploaderID: uploaderID,
		StorageKey: storageKey,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func parseDirectUploadFinalizeToken(secret []byte, token string) (*directUploadFinalizeClaims, error) {
	claims := &directUploadFinalizeClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
