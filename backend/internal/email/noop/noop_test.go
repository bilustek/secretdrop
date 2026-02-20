package noop_test

import (
	"context"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/email"
	"github.com/bilusteknoloji/secretdrop/internal/email/noop"
)

func TestSenderImplementsSender(t *testing.T) {
	t.Parallel()

	var _ email.Sender = (*noop.Sender)(nil)
}

func TestSendRecordsCalls(t *testing.T) {
	t.Parallel()

	sender := noop.New()
	ctx := context.Background()

	if err := sender.Send(ctx, "alice@example.com", "Subject 1", "<p>Body 1</p>"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if err := sender.Send(ctx, "bob@example.com", "Subject 2", "<p>Body 2</p>"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(sender.Calls) != 2 {
		t.Fatalf("Calls count = %d; want 2", len(sender.Calls))
	}

	if sender.Calls[0].To != "alice@example.com" {
		t.Errorf("Calls[0].To = %q; want %q", sender.Calls[0].To, "alice@example.com")
	}

	if sender.Calls[1].Subject != "Subject 2" {
		t.Errorf("Calls[1].Subject = %q; want %q", sender.Calls[1].Subject, "Subject 2")
	}
}
