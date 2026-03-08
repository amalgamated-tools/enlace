package model

import "time"

type File struct {
	ID         string
	ShareID    string
	UploaderID *string
	Name       string
	Size       int64
	MimeType   string
	StorageKey        string
	EncryptionIV      string
	EncryptedMetadata string
	CreatedAt         time.Time
}
