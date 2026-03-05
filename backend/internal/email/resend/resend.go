package resend

import (
	"context"
	"errors"
	"fmt"

	"github.com/bilustek/secretdrop/internal/email"

	resendapi "github.com/resend/resend-go/v2"
)

// compile-time interface check.
var _ email.Sender = (*Sender)(nil)

// Sender sends emails via the Resend API.
type Sender struct {
	client  *resendapi.Client
	from    string
	replyTo string
}

// Option configures a Sender value.
type Option func(*Sender) error

// WithFrom sets the sender email address.
func WithFrom(from string) Option {
	return func(s *Sender) error {
		if from == "" {
			return errors.New("from email cannot be empty")
		}

		s.from = from

		return nil
	}
}

// WithReplyTo sets the Reply-To address for outgoing emails.
func WithReplyTo(replyTo string) Option {
	return func(s *Sender) error {
		if replyTo == "" {
			return errors.New("reply-to email cannot be empty")
		}

		s.replyTo = replyTo

		return nil
	}
}

// New creates a new Resend email sender.
func New(apiKey string, opts ...Option) (*Sender, error) {
	if apiKey == "" {
		return nil, errors.New("API key cannot be empty")
	}

	s := &Sender{
		client: resendapi.NewClient(apiKey),
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return s, nil
}

// Send sends an email to the specified recipient.
func (s *Sender) Send(_ context.Context, to, subject, html string) error {
	params := &resendapi.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    html,
		ReplyTo: s.replyTo,
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("send email via resend: %w", err)
	}

	return nil
}
