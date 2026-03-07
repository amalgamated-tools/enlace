package model

import "time"

type PendingUpload struct {
	ID          string
	FileID      string
	ShareID     string
	UploaderID  *string
	Filename    string
	Size        int64
	MimeType    string
	StorageKey  string
	Status      string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	FinalizedAt *time.Time
}
