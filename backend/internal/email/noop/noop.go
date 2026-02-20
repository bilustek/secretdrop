package noop

import (
	"context"

	"github.com/bilusteknoloji/secretdrop/internal/email"
)

// compile-time interface check.
var _ email.Sender = (*Sender)(nil)

// Call records the arguments of a Send call.
type Call struct {
	To      string
	Subject string
	HTML    string
}

// Sender is a no-op email sender that records calls for testing.
type Sender struct {
	Calls []Call
}

// New creates a new no-op email sender.
func New() *Sender {
	return &Sender{}
}

// Send records the call and returns nil.
func (s *Sender) Send(_ context.Context, to, subject, html string) error {
	s.Calls = append(s.Calls, Call{To: to, Subject: subject, HTML: html})

	return nil
}
