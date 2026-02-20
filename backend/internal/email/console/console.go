package console

import (
	"context"
	"log/slog"

	"github.com/bilusteknoloji/secretdrop/internal/email"
)

// compile-time interface check.
var _ email.Sender = (*Sender)(nil)

// Sender logs emails to the console via slog instead of sending them.
type Sender struct{}

// New creates a new console email sender for development use.
func New() *Sender {
	return &Sender{}
}

// Send logs the email details to the console.
func (*Sender) Send(_ context.Context, to, subject, html string) error {
	slog.Info("email sent (dev mode)", "to", to, "subject", subject, "html", html)

	return nil
}
