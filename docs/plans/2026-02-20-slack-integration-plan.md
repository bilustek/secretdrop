# Slack Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Send Slack notifications for subscription lifecycle events and backend errors via incoming webhooks.

**Architecture:** Two separate webhook URLs (subscriptions + errors). Interface in parent package with webhook/console/noop implementations following the existing email sender pattern. Error notifications via custom slog.Handler that intercepts Error-level logs. Subscription/user events via explicit notifier calls in billing webhook handlers and account delete handler.

**Tech Stack:** Go stdlib (`net/http`, `log/slog`, `encoding/json`, `sync`), Slack incoming webhooks (Block Kit attachments)

---

### Task 1: Slack Package — Interface and Event Types

**Files:**
- Create: `backend/internal/slack/slack.go`

**Step 1: Create the interface and event types**

```go
package slack

import "context"

// EventType identifies the kind of Slack notification.
type EventType string

const (
	EventSubscriptionCreated   EventType = "subscription_created"
	EventSubscriptionCancelled EventType = "subscription_cancelled"
	EventUserDeleted           EventType = "user_deleted"
	EventError                 EventType = "error"
)

// Event represents a single Slack notification.
type Event struct {
	Type    EventType
	Message string
	Fields  map[string]string
}

// Notifier sends notifications to Slack.
type Notifier interface {
	Notify(ctx context.Context, event Event) error
}
```

**Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/slack/...`
Expected: success, no output

**Step 3: Commit**

```
git add backend/internal/slack/slack.go
git commit -m "add slack notifier interface and event types"
```

---

### Task 2: Noop Implementation

**Files:**
- Create: `backend/internal/slack/noop/noop.go`
- Create: `backend/internal/slack/noop/noop_test.go`

**Step 1: Write the test**

```go
package noop_test

import (
	"context"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/noop"
)

func TestNotify(t *testing.T) {
	t.Parallel()

	n := noop.New()
	ev := slack.Event{
		Type:    slack.EventSubscriptionCreated,
		Message: "test",
		Fields:  map[string]string{"user": "foo@bar.com"},
	}

	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(n.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(n.Calls))
	}

	if n.Calls[0].Type != slack.EventSubscriptionCreated {
		t.Errorf("expected event type %q, got %q", slack.EventSubscriptionCreated, n.Calls[0].Type)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/slack/noop/... -v`
Expected: FAIL — package does not exist

**Step 3: Write the implementation**

```go
package noop

import (
	"context"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
)

// compile-time interface check.
var _ slack.Notifier = (*Notifier)(nil)

// Call records the arguments of a Notify call.
type Call struct {
	Type    slack.EventType
	Message string
	Fields  map[string]string
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
	n.Calls = append(n.Calls, Call{
		Type:    event.Type,
		Message: event.Message,
		Fields:  event.Fields,
	})

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/slack/noop/... -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/slack/noop/
git commit -m "add noop slack notifier for testing"
```

---

### Task 3: Console Implementation

**Files:**
- Create: `backend/internal/slack/console/console.go`
- Create: `backend/internal/slack/console/console_test.go`

**Step 1: Write the test**

```go
package console_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/console"
)

func TestNotify(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	n := console.New(console.WithWriter(&buf))

	ev := slack.Event{
		Type:    slack.EventSubscriptionCreated,
		Message: "New subscription",
		Fields:  map[string]string{"user": "foo@bar.com"},
	}

	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	if !bytes.Contains([]byte(out), []byte("subscription_created")) {
		t.Errorf("expected output to contain event type, got: %s", out)
	}

	if !bytes.Contains([]byte(out), []byte("foo@bar.com")) {
		t.Errorf("expected output to contain user field, got: %s", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/slack/console/... -v`
Expected: FAIL — package does not exist

**Step 3: Write the implementation**

```go
package console

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
)

// compile-time interface check.
var _ slack.Notifier = (*Notifier)(nil)

// Notifier logs slack notifications to a writer (stderr by default).
type Notifier struct {
	w io.Writer
}

// Option configures a Notifier.
type Option func(*Notifier)

// WithWriter sets the output writer.
func WithWriter(w io.Writer) Option {
	return func(n *Notifier) {
		n.w = w
	}
}

// New creates a new console slack notifier for development use.
func New(opts ...Option) *Notifier {
	n := &Notifier{w: os.Stderr}

	for _, opt := range opts {
		opt(n)
	}

	return n
}

// Notify writes the event to the configured writer.
func (n *Notifier) Notify(_ context.Context, event slack.Event) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[SLACK] type=%s message=%q", event.Type, event.Message))

	if len(event.Fields) > 0 {
		keys := make([]string, 0, len(event.Fields))
		for k := range event.Fields {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			sb.WriteString(fmt.Sprintf(" %s=%q", k, event.Fields[k]))
		}
	}

	sb.WriteString("\n")

	fmt.Fprint(n.w, sb.String())

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/slack/console/... -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/slack/console/
git commit -m "add console slack notifier for development"
```

---

### Task 4: Webhook Implementation — Block Kit Formatting

**Files:**
- Create: `backend/internal/slack/webhook/webhook.go`
- Create: `backend/internal/slack/webhook/webhook_test.go`

**Step 1: Write the test**

Test the Block Kit payload construction and HTTP POST to a mock server.

```go
package webhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/webhook"
)

func TestNotifySubscriptionCreated(t *testing.T) {
	t.Parallel()

	var received []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n, err := webhook.New(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ev := slack.Event{
		Type:    slack.EventSubscriptionCreated,
		Message: "New subscription",
		Fields:  map[string]string{"user": "foo@bar.com", "plan": "Pro"},
	}

	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	attachments, ok := payload["attachments"].([]any)
	if !ok || len(attachments) == 0 {
		t.Fatal("expected attachments in payload")
	}

	att := attachments[0].(map[string]any)
	color := att["color"].(string)

	if color != "#36a64f" {
		t.Errorf("expected green color for subscription_created, got %q", color)
	}
}

func TestNotifyError(t *testing.T) {
	t.Parallel()

	var received []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n, err := webhook.New(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ev := slack.Event{
		Type:    slack.EventError,
		Message: "update user tier to pro",
		Fields:  map[string]string{"error": "connection refused", "request": "abc-123"},
	}

	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	attachments := payload["attachments"].([]any)
	att := attachments[0].(map[string]any)
	color := att["color"].(string)

	if color != "#d32f2f" {
		t.Errorf("expected red color for error, got %q", color)
	}

	// Verify the payload contains code block with error details
	raw, _ := json.Marshal(payload)
	body := string(raw)

	if !contains(body, "connection refused") {
		t.Error("expected error details in payload")
	}
}

func TestNewEmptyURL(t *testing.T) {
	t.Parallel()

	_, err := webhook.New("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/slack/webhook/... -v`
Expected: FAIL — package does not exist

**Step 3: Write the implementation**

```go
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
)

const httpTimeout = 5 * time.Second

// compile-time interface check.
var _ slack.Notifier = (*Notifier)(nil)

// Notifier sends notifications to a Slack incoming webhook URL.
type Notifier struct {
	webhookURL string
	client     *http.Client
}

// Option configures a Notifier.
type Option func(*Notifier) error

// New creates a new Slack webhook notifier.
func New(webhookURL string, opts ...Option) (*Notifier, error) {
	if webhookURL == "" {
		return nil, errors.New("webhook URL cannot be empty")
	}

	n := &Notifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: httpTimeout},
	}

	for _, opt := range opts {
		if err := opt(n); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return n, nil
}

// Notify sends the event to Slack as a Block Kit message.
func (n *Notifier) Notify(ctx context.Context, event slack.Event) error {
	payload := buildPayload(event)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func buildPayload(event slack.Event) map[string]any {
	color, emoji := colorAndEmoji(event.Type)
	title := titleForEvent(event.Type)

	blocks := []map[string]any{
		{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": emoji + " *" + title + "*",
			},
		},
	}

	if event.Type == slack.EventError {
		blocks = append(blocks, errorBlock(event))
	} else {
		blocks = append(blocks, fieldsBlock(event))
	}

	return map[string]any{
		"attachments": []map[string]any{
			{
				"color":  color,
				"blocks": blocks,
			},
		},
	}
}

func errorBlock(event slack.Event) map[string]any {
	var sb strings.Builder

	sb.WriteString("```\n")
	sb.WriteString("message: " + event.Message + "\n")

	keys := make([]string, 0, len(event.Fields))
	for k := range event.Fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		sb.WriteString(k + ": " + event.Fields[k] + "\n")
	}

	sb.WriteString("```")

	return map[string]any{
		"type": "section",
		"text": map[string]string{
			"type": "mrkdwn",
			"text": sb.String(),
		},
	}
}

func fieldsBlock(event slack.Event) map[string]any {
	fields := []map[string]string{
		{"type": "mrkdwn", "text": "*Event:*\n" + event.Message},
	}

	keys := make([]string, 0, len(event.Fields))
	for k := range event.Fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		fields = append(fields, map[string]string{
			"type": "mrkdwn",
			"text": "*" + strings.Title(k) + ":*\n" + event.Fields[k],
		})
	}

	return map[string]any{
		"type":   "section",
		"fields": fields,
	}
}

func colorAndEmoji(t slack.EventType) (string, string) {
	switch t {
	case slack.EventSubscriptionCreated:
		return "#36a64f", "\U0001f389"
	case slack.EventSubscriptionCancelled, slack.EventUserDeleted:
		return "#d32f2f", "\U0001f6a8"
	case slack.EventError:
		return "#d32f2f", "\U0001f6a8"
	default:
		return "#808080", "\u2139\ufe0f"
	}
}

func titleForEvent(t slack.EventType) string {
	switch t {
	case slack.EventSubscriptionCreated:
		return "New Subscription"
	case slack.EventSubscriptionCancelled:
		return "Subscription Cancelled"
	case slack.EventUserDeleted:
		return "User Deleted"
	case slack.EventError:
		return "Backend Error"
	default:
		return "Notification"
	}
}
```

Note: `strings.Title` is deprecated. Use `cases.Title(language.English).String(k)` from `golang.org/x/text` instead, or just capitalize manually:

```go
func capitalize(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}
```

Use `capitalize(k)` instead of `strings.Title(k)` in `fieldsBlock`.

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/slack/webhook/... -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/slack/webhook/
git commit -m "add slack webhook notifier with Block Kit formatting"
```

---

### Task 5: Custom slog.Handler for Error Notifications

**Files:**
- Create: `backend/internal/slack/sloghandler/sloghandler.go`
- Create: `backend/internal/slack/sloghandler/sloghandler_test.go`

**Step 1: Write the test**

```go
package sloghandler_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/slack/noop"
	"github.com/bilusteknoloji/secretdrop/internal/slack/sloghandler"
)

func TestErrorSendsNotification(t *testing.T) {
	t.Parallel()

	n := noop.New()
	inner := slog.NewJSONHandler(os.Stdout, nil)
	h := sloghandler.New(inner, n)

	logger := slog.New(h)
	logger.Error("something broke", "error", "connection refused", "request_id", "abc-123")

	// Give goroutine time to fire.
	time.Sleep(50 * time.Millisecond)

	if len(n.Calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(n.Calls))
	}

	if n.Calls[0].Type != slack.EventError {
		t.Errorf("expected error event, got %q", n.Calls[0].Type)
	}

	if n.Calls[0].Message != "something broke" {
		t.Errorf("expected message 'something broke', got %q", n.Calls[0].Message)
	}
}

func TestInfoDoesNotSendNotification(t *testing.T) {
	t.Parallel()

	n := noop.New()
	inner := slog.NewJSONHandler(os.Stdout, nil)
	h := sloghandler.New(inner, n)

	logger := slog.New(h)
	logger.Info("all good")

	time.Sleep(50 * time.Millisecond)

	if len(n.Calls) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(n.Calls))
	}
}

func TestRateLimiting(t *testing.T) {
	t.Parallel()

	n := noop.New()
	inner := slog.NewJSONHandler(os.Stdout, nil)
	h := sloghandler.New(inner, n, sloghandler.WithDedup(200*time.Millisecond))

	logger := slog.New(h)
	logger.Error("same error")
	logger.Error("same error")
	logger.Error("same error")

	time.Sleep(50 * time.Millisecond)

	if len(n.Calls) != 1 {
		t.Fatalf("expected 1 notification (deduped), got %d", len(n.Calls))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/slack/sloghandler/... -v`
Expected: FAIL — package does not exist

**Step 3: Write the implementation**

```go
package sloghandler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
)

const defaultDedupWindow = 1 * time.Minute

// Handler wraps a slog.Handler and sends Error-level logs to Slack.
type Handler struct {
	inner       slog.Handler
	notifier    slack.Notifier
	dedupWindow time.Duration

	mu   sync.Mutex
	seen map[string]time.Time
}

// Option configures a Handler.
type Option func(*Handler)

// WithDedup sets the deduplication window. Messages with the same text
// within this window are sent only once.
func WithDedup(d time.Duration) Option {
	return func(h *Handler) {
		h.dedupWindow = d
	}
}

// New creates a new slog handler that sends error-level logs to Slack.
func New(inner slog.Handler, notifier slack.Notifier, opts ...Option) *Handler {
	h := &Handler{
		inner:       inner,
		notifier:    notifier,
		dedupWindow: defaultDedupWindow,
		seen:        make(map[string]time.Time),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Enabled delegates to the inner handler.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle delegates to the inner handler, and for Error+ also sends to Slack.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	err := h.inner.Handle(ctx, record)

	if record.Level >= slog.LevelError {
		if h.shouldSend(record.Message) {
			event := buildEvent(record)

			go func() {
				_ = h.notifier.Notify(context.Background(), event)
			}()
		}
	}

	return err
}

// WithAttrs delegates to the inner handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		inner:       h.inner.WithAttrs(attrs),
		notifier:    h.notifier,
		dedupWindow: h.dedupWindow,
		seen:        h.seen,
		mu:          sync.Mutex{},
	}
}

// WithGroup delegates to the inner handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		inner:       h.inner.WithGroup(name),
		notifier:    h.notifier,
		dedupWindow: h.dedupWindow,
		seen:        h.seen,
		mu:          sync.Mutex{},
	}
}

func (h *Handler) shouldSend(msg string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	if last, ok := h.seen[msg]; ok && now.Sub(last) < h.dedupWindow {
		return false
	}

	h.seen[msg] = now

	// Lazy cleanup of old entries.
	for k, v := range h.seen {
		if now.Sub(v) >= h.dedupWindow {
			delete(h.seen, k)
		}
	}

	return true
}

func buildEvent(record slog.Record) slack.Event {
	fields := make(map[string]string)

	record.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.String()

		return true
	})

	return slack.Event{
		Type:    slack.EventError,
		Message: record.Message,
		Fields:  fields,
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/slack/sloghandler/... -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/slack/sloghandler/
git commit -m "add custom slog handler for Slack error notifications"
```

---

### Task 6: Config — Add Slack Webhook Environment Variables

**Files:**
- Modify: `backend/internal/config/config.go`

**Step 1: Add fields, options, and getters**

Add two fields to `Config` struct (after `stripePriceID`):

```go
slackWebhookSubscriptions string
slackWebhookNotifications string
```

Add getters:

```go
// SlackWebhookSubscriptions returns the Slack webhook URL for subscription events.
func (c *Config) SlackWebhookSubscriptions() string { return c.slackWebhookSubscriptions }

// SlackWebhookNotifications returns the Slack webhook URL for error notifications.
func (c *Config) SlackWebhookNotifications() string { return c.slackWebhookNotifications }
```

Add env var loading in `Load()` (after `c.stripePriceID = os.Getenv(...)`):

```go
c.slackWebhookSubscriptions = os.Getenv("SLACK_WEBHOOK_SUBSCRIPTIONS")
c.slackWebhookNotifications = os.Getenv("SLACK_WEBHOOK_NOTIFICATIONS")
```

Do NOT add these to the production required list — they are optional.

**Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: success

**Step 3: Commit**

```
git add backend/internal/config/config.go
git commit -m "add Slack webhook environment variables to config"
```

---

### Task 7: Billing Service — Add Notifier Dependency

**Files:**
- Modify: `backend/internal/billing/billing.go`
- Modify: `backend/internal/billing/webhook.go`

**Step 1: Add notifier field and option to billing.Service**

In `billing.go`, add import for `slack` package and add to `Service` struct:

```go
notifier slack.Notifier
```

Add functional option:

```go
// WithNotifier sets the Slack notifier for subscription events.
func WithNotifier(n slack.Notifier) Option {
	return func(s *Service) error {
		s.notifier = n

		return nil
	}
}
```

**Step 2: Add notification calls in webhook handlers**

In `webhook.go`, at the end of `handleCheckoutCompleted()` (after the `UpdateTier` call), add:

```go
if s.notifier != nil {
	go func() {
		ev := slack.Event{
			Type:    slack.EventSubscriptionCreated,
			Message: "New subscription",
			Fields: map[string]string{
				"user_id": strconv.FormatInt(userID, 10),
			},
		}

		if err := s.notifier.Notify(context.Background(), ev); err != nil {
			slog.Warn("send slack notification", "error", err)
		}
	}()
}
```

At the end of `handleSubscriptionDeleted()` (after the `UpdateTier` call to free), add:

```go
if s.notifier != nil {
	go func() {
		ev := slack.Event{
			Type:    slack.EventSubscriptionCancelled,
			Message: "Subscription cancelled",
			Fields: map[string]string{
				"user_id":     strconv.FormatInt(u.ID, 10),
				"customer_id": customerID,
			},
		}

		if err := s.notifier.Notify(context.Background(), ev); err != nil {
			slog.Warn("send slack notification", "error", err)
		}
	}()
}
```

Note: Use `slog.Warn` (not `slog.Error`) to avoid triggering infinite notification loop via the slog handler.

**Step 3: Verify it compiles and existing tests pass**

Run: `cd backend && go build ./... && go test ./internal/billing/... -v`
Expected: all pass

**Step 4: Commit**

```
git add backend/internal/billing/
git commit -m "add Slack notifications for subscription events in billing"
```

---

### Task 8: Account Handler — Add Notifier for User Deletion

**Files:**
- Modify: `backend/internal/handler/account.go`

**Step 1: Add notifier parameter to NewDeleteAccountHandler**

Change signature to:

```go
func NewDeleteAccountHandler(userRepo user.Repository, canceller SubscriptionCanceller, notifier slack.Notifier) http.HandlerFunc {
```

Add notification after successful `DeleteUser()`, before `w.WriteHeader(http.StatusNoContent)`:

```go
if notifier != nil {
	go func() {
		ev := slack.Event{
			Type:    slack.EventUserDeleted,
			Message: "User account deleted",
			Fields: map[string]string{
				"user_id": strconv.FormatInt(claims.UserID, 10),
				"email":   claims.Email,
			},
		}

		if err := notifier.Notify(context.Background(), ev); err != nil {
			slog.Warn("send slack notification", "error", err)
		}
	}()
}
```

Add necessary imports: `"strconv"`, `"github.com/bilusteknoloji/secretdrop/internal/slack"`.

**Step 2: Verify it compiles**

This will fail because `main.go` still calls with the old signature. That's expected — we'll fix it in Task 9.

Run: `cd backend && go build ./internal/handler/...`
Expected: success (handler package compiles independently)

**Step 3: Commit**

```
git add backend/internal/handler/account.go
git commit -m "add Slack notification for user deletion in account handler"
```

---

### Task 9: Wire Everything in main.go

**Files:**
- Modify: `backend/cmd/secretdrop/main.go`

**Step 1: Add imports**

Add to import block:

```go
slackpkg "github.com/bilusteknoloji/secretdrop/internal/slack"
slackconsole "github.com/bilusteknoloji/secretdrop/internal/slack/console"
slacknoop "github.com/bilusteknoloji/secretdrop/internal/slack/noop"
slackwebhook "github.com/bilusteknoloji/secretdrop/internal/slack/webhook"
"github.com/bilusteknoloji/secretdrop/internal/slack/sloghandler"
```

**Step 2: Create notifier instances and slog handler**

Add after `config.Load()` and before the logger setup (move logger setup down). The new wiring order is:

1. `config.Load()`
2. Create notifier instances
3. Setup logger with slog handler wrapping error notifier
4. Rest of the existing code...

```go
// Slack notifiers
var subscriptionNotifier slackpkg.Notifier
var errorNotifier slackpkg.Notifier

if cfg.IsDev() {
	subscriptionNotifier = slackconsole.New()
	errorNotifier = slackconsole.New()
	slog.Info("development mode: slack notifications will be printed to stderr")
} else {
	subscriptionNotifier = selectNotifier(cfg.SlackWebhookSubscriptions())
	errorNotifier = selectNotifier(cfg.SlackWebhookNotifications())
}

// Logger with Slack error handler
baseHandler := slog.NewJSONHandler(os.Stdout, nil)
slogHandler := sloghandler.New(baseHandler, errorNotifier)
logger := slog.New(slogHandler)
slog.SetDefault(logger)
```

Add helper function:

```go
func selectNotifier(webhookURL string) slackpkg.Notifier {
	if webhookURL == "" {
		return slacknoop.New()
	}

	n, err := slackwebhook.New(webhookURL)
	if err != nil {
		slog.Warn("invalid slack webhook URL, using noop notifier", "error", err)

		return slacknoop.New()
	}

	return n
}
```

**Step 3: Pass notifier to billing and account handler**

Update `setupBilling()` call to pass notifier — change signature to accept `slackpkg.Notifier` and pass it via `billing.WithNotifier(notifier)`.

Update `NewDeleteAccountHandler` call:

```go
mux.HandleFunc("DELETE /api/v1/me", handler.NewDeleteAccountHandler(userRepo, canceller, subscriptionNotifier))
```

**Step 4: Verify it compiles and all tests pass**

Run: `cd backend && go build ./... && go test -race ./...`
Expected: all pass

**Step 5: Commit**

```
git add backend/cmd/secretdrop/main.go
git commit -m "wire Slack notifiers in main.go"
```

---

### Task 10: Update CLAUDE.md and Run Full Lint + Test

**Files:**
- Modify: `CLAUDE.md` (add env vars to table)

**Step 1: Add env vars to CLAUDE.md table**

Add to Environment Variables table:

```
| `SLACK_WEBHOOK_SUBSCRIPTIONS` | No | — |
| `SLACK_WEBHOOK_NOTIFICATIONS` | No | — |
```

**Step 2: Run lint**

Run: `cd backend && golangci-lint run ./...`
Expected: no errors

**Step 3: Run all tests**

Run: `cd backend && go test -race -count=1 ./...`
Expected: all pass

**Step 4: Commit and push**

```
git add CLAUDE.md
git commit -m "add Slack webhook env vars to documentation"
git push -u origin <branch-name>
```

**Step 5: Create PR**

Create PR with title "Add Slack integration for subscription and error notifications" referencing issue #12.
