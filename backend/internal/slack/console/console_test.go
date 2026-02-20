package console_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/console"
)

func TestNotifierImplementsNotifier(t *testing.T) {
	t.Parallel()

	var _ slack.Notifier = (*console.Notifier)(nil)
}

func TestNotifyOutputContainsEventType(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	notifier := console.New(console.WithWriter(&buf))
	event := slack.Event{
		Type:    slack.EventSubscriptionCreated,
		Message: "New subscription",
	}

	if err := notifier.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "[SLACK]") {
		t.Errorf("output missing [SLACK] prefix; got %q", output)
	}

	if !strings.Contains(output, "type=subscription_created") {
		t.Errorf("output missing event type; got %q", output)
	}

	if !strings.Contains(output, "message=New subscription") {
		t.Errorf("output missing message; got %q", output)
	}
}

func TestNotifyOutputContainsFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	notifier := console.New(console.WithWriter(&buf))
	event := slack.Event{
		Type:    slack.EventError,
		Message: "Something failed",
		Fields:  map[string]string{"email": "alice@example.com", "error": "timeout"},
	}

	if err := notifier.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "email=alice@example.com") {
		t.Errorf("output missing email field; got %q", output)
	}

	if !strings.Contains(output, "error=timeout") {
		t.Errorf("output missing error field; got %q", output)
	}
}

func TestNotifyFieldsAreSorted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	notifier := console.New(console.WithWriter(&buf))
	event := slack.Event{
		Type:    slack.EventUserDeleted,
		Message: "User deleted",
		Fields:  map[string]string{"zebra": "z", "alpha": "a", "middle": "m"},
	}

	if err := notifier.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	output := buf.String()

	alphaIdx := strings.Index(output, "alpha=a")
	middleIdx := strings.Index(output, "middle=m")
	zebraIdx := strings.Index(output, "zebra=z")

	if alphaIdx == -1 || middleIdx == -1 || zebraIdx == -1 {
		t.Fatalf("output missing fields; got %q", output)
	}

	if !(alphaIdx < middleIdx && middleIdx < zebraIdx) {
		t.Errorf("fields not sorted alphabetically; got %q", output)
	}
}
