package email

import "context"

// Sender defines the interface for sending secret notification emails.
type Sender interface {
	Send(ctx context.Context, to, subject, html string) error
}
