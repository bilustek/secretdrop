package model

import "time"

const (
	// MaxTextLength is the maximum allowed size for secret text (4KB).
	MaxTextLength = 4096
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
