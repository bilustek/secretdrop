package model

import "time"

const (
	// MaxTextLength is the maximum allowed size for secret text (4KB).
	MaxTextLength = 4096
	// MaxRecipients is the maximum number of recipients per secret.
	MaxRecipients = 5
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
