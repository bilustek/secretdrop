package model

// CreateRequest is the incoming JSON body for creating secrets.
type CreateRequest struct {
	Text      string   `json:"text"`
	To        []string `json:"to"`
	ExpiresIn string   `json:"expires_in,omitempty"`
}

// RevealRequest is the incoming JSON body for revealing a secret.
type RevealRequest struct {
	Email string `json:"email"`
	Key   string `json:"key"`
}
