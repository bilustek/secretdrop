package noop

import (
	"context"

	"github.com/bilustek/secretdrop/internal/slack"
)

// compile-time interface check.
var _ slack.Notifier = (*Notifier)(nil)

// Call records the arguments of a Notify call.
type Call struct {
	Event slack.Event
}

// Notifier is a no-op slack notifier that records calls for testing.
type Notifier struct {
	Calls []Call
}

// New creates a new no-op slack notifier.
func New() *Notifier {
	return &Notifier{}
}

// Notify records the call and returns nil.
func (n *Notifier) Notify(_ context.Context, event slack.Event) error {
	n.Calls = append(n.Calls, Call{Event: event})

	return nil
}
