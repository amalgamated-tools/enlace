package handler

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidDirectTransferToken = errors.New("invalid direct transfer token")

type directUploadFinalizeClaims struct {
	UploadID   string `json:"upload_id"`
	ShareID    string `json:"share_id"`
	UploaderID string `json:"uploader_id,omitempty"`
	Public     bool   `json:"public"`
	StorageKey string `json:"storage_key"`
	jwt.RegisteredClaims
}

type directUploadInitiateRequest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type directUploadInitiateResponse struct {
	UploadID      string            `json:"upload_id"`
	FileID        string            `json:"file_id"`
	Filename      string            `json:"filename"`
	Size          int64             `json:"size"`
	MimeType      string            `json:"mime_type"`
	URL           string            `json:"url"`
	Method        string            `json:"method"`
	Headers       map[string]string `json:"headers,omitempty"`
	ExpiresAt     string            `json:"expires_at"`
	FinalizeToken string            `json:"finalize_token"`
}

type directUploadFinalizeRequest struct {
	Token string `json:"token"`
}

type directDownloadURLResponse struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt string            `json:"expires_at"`
}

func generateDirectUploadFinalizeToken(secret []byte, claims directUploadFinalizeClaims, expiresAt time.Time) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func validateDirectUploadFinalizeToken(secret []byte, tokenStr string) (*directUploadFinalizeClaims, error) {
	if tokenStr == "" {
		return nil, ErrInvalidDirectTransferToken
	}

	token, err := jwt.ParseWithClaims(tokenStr, &directUploadFinalizeClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidDirectTransferToken
		}
		return secret, nil
	})
	if err != nil {
		return nil, ErrInvalidDirectTransferToken
	}

	claims, ok := token.Claims.(*directUploadFinalizeClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidDirectTransferToken
	}
	return claims, nil
}
