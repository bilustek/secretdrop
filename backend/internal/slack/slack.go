package slack

import "context"

// EventType identifies the kind of Slack notification.
type EventType string

// Slack event types for notifications.
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
