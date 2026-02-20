package webhook_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/webhook"
)

func TestNotifierImplementsNotifier(t *testing.T) {
	t.Parallel()

	var _ slack.Notifier = (*webhook.Notifier)(nil)
}

func TestNewEmptyURL(t *testing.T) {
	t.Parallel()

	_, err := webhook.New("")
	if err == nil {
		t.Fatal("New(\"\") should return error")
	}

	if err != webhook.ErrEmptyURL {
		t.Errorf("error = %v; want %v", err, webhook.ErrEmptyURL)
	}
}

func TestNotifySubscriptionCreated(t *testing.T) {
	t.Parallel()

	var capturedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}

		capturedBody = string(body)

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q; want application/json", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier, err := webhook.New(srv.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	event := slack.Event{
		Type:    slack.EventSubscriptionCreated,
		Message: "New subscription",
		Fields:  map[string]string{"user": "foo@bar.com"},
	}

	if err := notifier.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Verify green color for subscription created.
	if !strings.Contains(capturedBody, "#36a64f") {
		t.Errorf("payload missing green color #36a64f; got %s", capturedBody)
	}

	// Verify attachments structure.
	if !strings.Contains(capturedBody, "attachments") {
		t.Errorf("payload missing attachments; got %s", capturedBody)
	}

	// Verify title.
	if !strings.Contains(capturedBody, "New Subscription") {
		t.Errorf("payload missing title 'New Subscription'; got %s", capturedBody)
	}

	// Verify fields section contains the event message.
	if !strings.Contains(capturedBody, "New subscription") {
		t.Errorf("payload missing event message; got %s", capturedBody)
	}

	// Verify fields section contains the user.
	if !strings.Contains(capturedBody, "foo@bar.com") {
		t.Errorf("payload missing user field; got %s", capturedBody)
	}
}

func TestNotifyError(t *testing.T) {
	t.Parallel()

	var capturedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}

		capturedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier, err := webhook.New(srv.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	event := slack.Event{
		Type:    slack.EventError,
		Message: "update user tier",
		Fields: map[string]string{
			"error":   "connection refused",
			"request": "abc-123",
		},
	}

	if err := notifier.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Verify red color for error events.
	if !strings.Contains(capturedBody, "#d32f2f") {
		t.Errorf("payload missing red color #d32f2f; got %s", capturedBody)
	}

	// Verify error title.
	if !strings.Contains(capturedBody, "Backend Error") {
		t.Errorf("payload missing title 'Backend Error'; got %s", capturedBody)
	}

	// Verify error details are present (code block content).
	if !strings.Contains(capturedBody, "update user tier") {
		t.Errorf("payload missing error message; got %s", capturedBody)
	}

	if !strings.Contains(capturedBody, "connection refused") {
		t.Errorf("payload missing error field; got %s", capturedBody)
	}

	if !strings.Contains(capturedBody, "abc-123") {
		t.Errorf("payload missing request field; got %s", capturedBody)
	}
}

func TestNotifyNon200Response(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	notifier, err := webhook.New(srv.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	event := slack.Event{
		Type:    slack.EventSubscriptionCreated,
		Message: "test",
	}

	err = notifier.Notify(context.Background(), event)
	if err == nil {
		t.Fatal("Notify() should return error on non-200 response")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code 500; got %v", err)
	}
}

func TestNotifySubscriptionCancelled(t *testing.T) {
	t.Parallel()

	var capturedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}

		capturedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier, err := webhook.New(srv.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	event := slack.Event{
		Type:    slack.EventSubscriptionCancelled,
		Message: "Subscription cancelled",
		Fields:  map[string]string{"user": "test@example.com"},
	}

	if err := notifier.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Verify red color for cancellation.
	if !strings.Contains(capturedBody, "#d32f2f") {
		t.Errorf("payload missing red color; got %s", capturedBody)
	}

	// Verify title.
	if !strings.Contains(capturedBody, "Subscription Cancelled") {
		t.Errorf("payload missing title; got %s", capturedBody)
	}
}
