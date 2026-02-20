package console_test

import (
	"context"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/email"
	"github.com/bilusteknoloji/secretdrop/internal/email/console"
)

func TestSenderImplementsSender(t *testing.T) {
	t.Parallel()

	var _ email.Sender = (*console.Sender)(nil)
}

func TestSendReturnsNil(t *testing.T) {
	t.Parallel()

	sender := console.New()
	err := sender.Send(context.Background(), "test@example.com", "Test Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("Send() error = %v; want nil", err)
	}
}
