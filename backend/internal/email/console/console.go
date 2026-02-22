package console

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/email"
)

const separator = "-------------------------------------------------------------------------------"

// compile-time interface check.
var _ email.Sender = (*Sender)(nil)

// Sender logs emails to the console in RFC 2822-like format (similar to
// Django's console email backend) instead of actually sending them.
type Sender struct {
	from    string
	replyTo string
}

// Option configures a Sender value.
type Option func(*Sender)

// WithFrom sets the sender email address shown in the output.
func WithFrom(from string) Option {
	return func(s *Sender) {
		s.from = from
	}
}

// WithReplyTo sets the Reply-To address shown in the output.
func WithReplyTo(replyTo string) Option {
	return func(s *Sender) {
		s.replyTo = replyTo
	}
}

// New creates a new console email sender for development use.
func New(opts ...Option) *Sender {
	s := &Sender{
		from: "webmaster@localhost",
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Send prints the email to stderr in an RFC 2822-like format.
func (s *Sender) Send(_ context.Context, to, subject, body string) error {
	hostname, _ := os.Hostname()

	replyToLine := ""
	if s.replyTo != "" {
		replyToLine = fmt.Sprintf("Reply-To: %s\n", s.replyTo)
	}

	msg := fmt.Sprintf(`%s
Content-Type: text/html; charset="utf-8"
MIME-Version: 1.0
Subject: %s
From: %s
To: %s
%sDate: %s
Message-ID: <%d@%s>

%s
%s`,
		separator,
		subject,
		s.from,
		to,
		replyToLine,
		time.Now().Format(time.RFC1123Z),
		time.Now().UnixNano(),
		hostname,
		body,
		separator,
	)

	fmt.Fprintln(os.Stderr, msg)

	return nil
}
