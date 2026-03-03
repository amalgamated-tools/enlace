package model

import "time"

// ShareRecipient represents an email notification sent for a share.
type ShareRecipient struct {
	ID      string
	ShareID string
	Email   string
	SentAt  time.Time
}
