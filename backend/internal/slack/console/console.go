package console

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
)

// compile-time interface check.
var _ slack.Notifier = (*Notifier)(nil)

// Notifier logs Slack events to a writer (stderr by default) for development use.
type Notifier struct {
	writer io.Writer
}

// Option configures a Notifier value.
type Option func(*Notifier)

// WithWriter sets the output writer for the notifier.
func WithWriter(w io.Writer) Option {
	return func(n *Notifier) {
		n.writer = w
	}
}

// New creates a new console slack notifier for development use.
func New(opts ...Option) *Notifier {
	n := &Notifier{
		writer: os.Stderr,
	}

	for _, opt := range opts {
		opt(n)
	}

	return n
}

// Notify prints the event to the configured writer in a simple key=value format.
func (n *Notifier) Notify(_ context.Context, event slack.Event) error {
	line := fmt.Sprintf("[SLACK] type=%s message=%s", event.Type, event.Message)

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(event.Fields))
	for k := range event.Fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		line += fmt.Sprintf(" %s=%s", k, event.Fields[k])
	}

	fmt.Fprintln(n.writer, line)

	return nil
}
