package email

import (
	"context"
	"fmt"

	"github.com/resend/resend-go/v2"
)

// Sender defines the interface for sending secret notification emails.
type Sender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// compile-time interface check.
var _ Sender = (*ResendSender)(nil)

// ResendSender sends emails via the Resend API.
type ResendSender struct {
	client *resend.Client
	from   string
}

// NewResendSender creates a new ResendSender with the given API key and from address.
func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

// Send sends an email to the specified recipient.
func (s *ResendSender) Send(_ context.Context, to, subject, html string) error {
	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    html,
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("send email via resend: %w", err)
	}

	return nil
}

// NoopSender is a no-op email sender for testing.
type NoopSender struct {
	Calls []SendCall
}

// SendCall records the arguments of a Send call.
type SendCall struct {
	To      string
	Subject string
	HTML    string
}

// Send records the call and returns nil.
func (s *NoopSender) Send(_ context.Context, to, subject, html string) error {
	s.Calls = append(s.Calls, SendCall{To: to, Subject: subject, HTML: html})

	return nil
}
