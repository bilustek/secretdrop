# Slack Integration Design

## Goal

Send Slack notifications for subscription lifecycle events and backend errors
via two separate webhook URLs, non-blocking, using Block Kit formatting.

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `SLACK_WEBHOOK_SUBSCRIPTIONS` | Webhook for subscription/user events |
| `SLACK_WEBHOOK_NOTIFICATIONS` | Webhook for error notifications |

Both optional — if empty, noop notifier is used (no messages sent).

## Package Structure

```
internal/slack/
├── slack.go              # Notifier interface + Event type
├── webhook/
│   └── webhook.go        # Production: HTTP POST to Slack webhook URL
├── console/
│   └── console.go        # Development: log to stderr
└── noop/
    └── noop.go           # Test: record calls
```

Follows the existing `internal/email/` pattern exactly:
- Interface in parent package
- Implementations in sub-packages
- Functional options for configuration
- `New()` constructors (never `NewFoo()`)
- Compile-time interface checks

## Interface

```go
// internal/slack/slack.go

type EventType string

const (
    EventSubscriptionCreated   EventType = "subscription_created"
    EventSubscriptionCancelled EventType = "subscription_cancelled"
    EventUserDeleted           EventType = "user_deleted"
    EventError                 EventType = "error"
)

type Event struct {
    Type    EventType
    Message string
    Fields  map[string]string
}

type Notifier interface {
    Notify(ctx context.Context, event Event) error
}
```

## Two Webhook URLs, Two Notifier Instances

`main.go` creates two separate notifier instances:
- `subscriptionNotifier` — wired into billing webhook handler + account delete handler
- `errorNotifier` — wired into custom slog handler

In development: both use console notifier.
In production with no URL configured: both use noop notifier.

## Subscription/User Event Notifications

Explicit `notifier.Notify()` calls at these hook points:

| Event | Location | After |
|-------|----------|-------|
| Subscription created | `billing/webhook.go:handleCheckoutCompleted()` | `UpdateTier()` succeeds |
| Subscription cancelled | `billing/webhook.go:handleSubscriptionDeleted()` | `UpdateTier()` succeeds |
| User deleted | `handler/account.go:DeleteAccountHandler` | `DeleteUser()` succeeds |

All calls are fire-and-forget in a goroutine. Errors are logged via `slog.Warn`
(not `slog.Error` to avoid infinite loop with error notifier).

### Block Kit Format — Subscription Events

```json
{
  "attachments": [{
    "color": "#36a64f",
    "blocks": [{
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "🎉 *New Subscription*"
      }
    }, {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*User:*\nuser@email.com"},
        {"type": "mrkdwn", "text": "*Plan:*\nPro"}
      ]
    }]
  }]
}
```

Color: green (#36a64f) for created, red (#d32f2f) for cancelled/deleted.

## Error Notifications via Custom slog.Handler

`internal/slack/sloghandler/sloghandler.go` — wraps any `slog.Handler`:

- Delegates all calls to the inner handler (normal logging continues)
- On `Handle()` with `LevelError` or above, sends Slack notification in a goroutine
- Rate limiting: skip duplicate messages within 1 minute window (keyed on message text)

### Block Kit Format — Error Events

```json
{
  "attachments": [{
    "color": "#d32f2f",
    "blocks": [{
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "🚨 *Backend Error*"
      }
    }, {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "```\nmessage: update user tier to pro\nerror:   connection refused\nrequest: abc-123\n```"
      }
    }]
  }]
}
```

Error details in code block for easy copy/paste.

## Config Changes

Add to `internal/config/config.go`:
- Fields: `slackWebhookSubscriptions`, `slackWebhookNotifications`
- Getters: `SlackWebhookSubscriptions()`, `SlackWebhookNotifications()`
- Options: `WithSlackWebhookSubscriptions()`, `WithSlackWebhookNotifications()`
- NOT added to production required list (both are optional)

## main.go Wiring

```
var subscriptionNotifier slack.Notifier
var errorNotifier slack.Notifier

if cfg.IsDev() {
    subscriptionNotifier = console.New()
    errorNotifier = console.New()
} else {
    // each falls back to noop if URL is empty
    subscriptionNotifier = selectNotifier(cfg.SlackWebhookSubscriptions())
    errorNotifier = selectNotifier(cfg.SlackWebhookNotifications())
}

// Wrap slog handler for error notifications
baseHandler := slog.NewJSONHandler(os.Stdout, nil)
slogHandler := sloghandler.New(baseHandler, errorNotifier)
logger := slog.New(slogHandler)
slog.SetDefault(logger)
```

## Non-Goals

- No retry logic for failed webhook calls (Slack webhooks are best-effort)
- No queue/buffer for notifications (goroutine per event is sufficient)
- No Slack API (only incoming webhooks)
