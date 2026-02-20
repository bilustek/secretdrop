package model

import "time"

// Subscription status constants.
const (
	SubscriptionActive   = "active"
	SubscriptionCanceled = "canceled"
	SubscriptionPastDue  = "past_due"
)

// Subscription represents a Stripe subscription linked to a user.
type Subscription struct {
	ID                   int64
	UserID               int64
	StripeCustomerID     string
	StripeSubscriptionID string
	Status               string
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
	CreatedAt            time.Time
}
