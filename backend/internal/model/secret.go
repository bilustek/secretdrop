package model

import "time"

const (
	// FreeMaxTextLength is the maximum secret text length for free tier (4KB).
	FreeMaxTextLength = 4096
	// ProMaxTextLength is the maximum secret text length for pro tier (64KB).
	ProMaxTextLength = 65536
	// FreeMaxRecipients is the maximum number of recipients for free tier.
	FreeMaxRecipients = 1
	// ProMaxRecipients is the maximum number of recipients for pro tier.
	ProMaxRecipients = 5
)

// Secret represents a stored encrypted secret in the database.
type Secret struct {
	ID            int64
	Token         string
	EncryptedBlob []byte
	Nonce         []byte
	RecipientHash string
	ExpiresAt     time.Time
	Viewed        bool
	CreatedAt     time.Time
}
