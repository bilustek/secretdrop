package noop_test

import (
	"context"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/noop"
)

func TestNotifierImplementsNotifier(t *testing.T) {
	t.Parallel()

	var _ slack.Notifier = (*noop.Notifier)(nil)
}

func TestNotifyRecordsCalls(t *testing.T) {
	t.Parallel()

	notifier := noop.New()
	ctx := context.Background()

	event1 := slack.Event{
		Type:    slack.EventSubscriptionCreated,
		Message: "New subscription",
		Fields:  map[string]string{"email": "alice@example.com", "plan": "pro"},
	}

	event2 := slack.Event{
		Type:    slack.EventUserDeleted,
		Message: "User deleted",
		Fields:  map[string]string{"email": "bob@example.com"},
	}

	if err := notifier.Notify(ctx, event1); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if err := notifier.Notify(ctx, event2); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if len(notifier.Calls) != 2 {
		t.Fatalf("Calls count = %d; want 2", len(notifier.Calls))
	}

	if notifier.Calls[0].Event.Type != slack.EventSubscriptionCreated {
		t.Errorf("Calls[0].Event.Type = %q; want %q", notifier.Calls[0].Event.Type, slack.EventSubscriptionCreated)
	}

	if notifier.Calls[0].Event.Message != "New subscription" {
		t.Errorf("Calls[0].Event.Message = %q; want %q", notifier.Calls[0].Event.Message, "New subscription")
	}

	if notifier.Calls[0].Event.Fields["email"] != "alice@example.com" {
		t.Errorf("Calls[0].Event.Fields[email] = %q; want %q", notifier.Calls[0].Event.Fields["email"], "alice@example.com")
	}

	if notifier.Calls[1].Event.Type != slack.EventUserDeleted {
		t.Errorf("Calls[1].Event.Type = %q; want %q", notifier.Calls[1].Event.Type, slack.EventUserDeleted)
	}
}
