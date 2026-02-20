package sloghandler_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/sloghandler"
)

func TestErrorSendsNotification(t *testing.T) {
	t.Parallel()

	notifier := &safeNotifier{}
	inner := slog.NewJSONHandler(discardWriter{}, nil)
	handler := sloghandler.New(inner, notifier)

	logger := slog.New(handler)
	logger.Error("something went wrong", "request_id", "abc-123")

	time.Sleep(50 * time.Millisecond)

	calls := notifier.calls()

	if len(calls) != 1 {
		t.Fatalf("Calls count = %d; want 1", len(calls))
	}

	call := calls[0]

	if call.Type != slack.EventError {
		t.Errorf("Event.Type = %q; want %q", call.Type, slack.EventError)
	}

	if call.Message != "something went wrong" {
		t.Errorf("Event.Message = %q; want %q", call.Message, "something went wrong")
	}

	if call.Fields["request_id"] != "abc-123" {
		t.Errorf("Event.Fields[request_id] = %q; want %q", call.Fields["request_id"], "abc-123")
	}
}

func TestInfoDoesNotSendNotification(t *testing.T) {
	t.Parallel()

	notifier := &safeNotifier{}
	inner := slog.NewJSONHandler(discardWriter{}, nil)
	handler := sloghandler.New(inner, notifier)

	logger := slog.New(handler)
	logger.Info("all is fine")

	time.Sleep(50 * time.Millisecond)

	if len(notifier.calls()) != 0 {
		t.Fatalf("Calls count = %d; want 0", len(notifier.calls()))
	}
}

func TestRateLimiting(t *testing.T) {
	t.Parallel()

	notifier := &safeNotifier{}
	inner := slog.NewJSONHandler(discardWriter{}, nil)
	handler := sloghandler.New(inner, notifier, sloghandler.WithDedup(200*time.Millisecond))

	logger := slog.New(handler)
	logger.Error("repeated error")
	logger.Error("repeated error")
	logger.Error("repeated error")

	time.Sleep(50 * time.Millisecond)

	if count := len(notifier.calls()); count != 1 {
		t.Fatalf("Calls count = %d; want 1", count)
	}
}

// safeNotifier is a thread-safe slack.Notifier for testing with goroutines.
type safeNotifier struct {
	mu     sync.Mutex
	events []slack.Event
}

func (n *safeNotifier) Notify(_ context.Context, event slack.Event) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.events = append(n.events, event)

	return nil
}

func (n *safeNotifier) calls() []slack.Event {
	n.mu.Lock()
	defer n.mu.Unlock()

	cp := make([]slack.Event, len(n.events))
	copy(cp, n.events)

	return cp
}

// discardWriter is an io.Writer that discards all data.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
