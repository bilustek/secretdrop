package model

import "time"

// RecipientLink holds the generated link for a single recipient.
type RecipientLink struct {
	Email string `json:"email"`
	Link  string `json:"link"`
}

// CreateResponse is the JSON body returned after secret creation.
type CreateResponse struct {
	ID         string          `json:"id"`
	ExpiresAt  time.Time       `json:"expires_at"`
	Recipients []RecipientLink `json:"recipients"`
}

// RevealResponse is the JSON body returned after successful reveal.
type RevealResponse struct {
	Text string `json:"text"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the inner error object.
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
