package billing

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/slack"
)

const (
	maxWebhookBodySize = 65536 // 64KB

	slogKeySubscriptionID = "subscription_id"
	slogKeyUserID         = "user_id"
	slogKeyCustomerID     = "customer_id"
)

// HandleWebhook processes Stripe webhook events.
// This endpoint has NO auth — it uses Stripe signature verification instead.
func (s *Service) HandleWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
		if err != nil {
			writeError(w, "invalid_request", "Failed to read request body", http.StatusBadRequest)

			return
		}

		sig := r.Header.Get("Stripe-Signature")

		event, err := webhook.ConstructEventWithOptions(body, sig, s.webhookSecret, webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
		if err != nil {
			writeError(w, "invalid_signature", "Invalid webhook signature", http.StatusBadRequest)

			return
		}

		switch event.Type {
		case "checkout.session.completed":
			s.handleCheckoutCompleted(r.Context(), event)
		case "invoice.paid":
			s.handleInvoicePaid(r.Context(), event)
		case "customer.subscription.deleted":
			s.handleSubscriptionDeleted(r.Context(), event)
		case "customer.subscription.updated":
			s.handleSubscriptionUpdated(r.Context(), event)
		default:
			slog.Debug("unhandled webhook event", "type", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s *Service) handleCheckoutCompleted(ctx context.Context, event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		slog.Error("unmarshal checkout session", slogKeyError, err)

		return
	}

	userID, err := strconv.ParseInt(session.ClientReferenceID, 10, 64)
	if err != nil {
		slog.Error("parse client reference ID", slogKeyError, err, "value", session.ClientReferenceID)

		return
	}

	var customerID string
	if session.Customer != nil {
		customerID = session.Customer.ID
	}

	var subscriptionID string
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	sub := &model.Subscription{
		UserID:               userID,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscriptionID,
		Status:               model.SubscriptionActive,
	}

	if err := s.userRepo.UpsertSubscription(ctx, sub); err != nil {
		slog.Error("upsert subscription", slogKeyError, err)

		return
	}

	if err := s.userRepo.UpdateTier(ctx, userID, model.TierPro); err != nil {
		slog.Error("update user tier to pro", slogKeyError, err, slogKeyUserID, userID)
	}

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
}

func (s *Service) handleInvoicePaid(ctx context.Context, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		slog.Error("unmarshal invoice", slogKeyError, err)

		return
	}

	var customerID string
	if invoice.Customer != nil {
		customerID = invoice.Customer.ID
	}

	if customerID == "" {
		slog.Error("invoice has no customer ID")

		return
	}

	u, err := s.userRepo.FindUserByStripeCustomerID(ctx, customerID)
	if err != nil {
		slog.Error("find user by stripe customer ID", slogKeyError, err, slogKeyCustomerID, customerID)

		return
	}

	if err := s.userRepo.ResetSecretsUsed(ctx, u.ID); err != nil {
		slog.Error("reset secrets used", slogKeyError, err, slogKeyUserID, u.ID)
	}

	if invoice.Lines != nil && len(invoice.Lines.Data) > 0 {
		line := invoice.Lines.Data[0]
		periodStart := time.Unix(line.Period.Start, 0)
		periodEnd := time.Unix(line.Period.End, 0)

		sub, subErr := s.userRepo.FindSubscriptionByUserID(ctx, u.ID)
		if subErr == nil {
			periodErr := s.userRepo.UpdateSubscriptionPeriod(
				ctx, sub.StripeSubscriptionID, periodStart, periodEnd,
			)
			if periodErr != nil {
				slog.Error("update subscription period from invoice", slogKeyError, periodErr)
			}
		}
	}
}

func (s *Service) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("unmarshal subscription", slogKeyError, err)

		return
	}

	if err := s.userRepo.UpdateSubscriptionStatus(ctx, sub.ID, model.SubscriptionCanceled); err != nil {
		slog.Error("update subscription status to canceled", slogKeyError, err, slogKeySubscriptionID, sub.ID)
	}

	var customerID string
	if sub.Customer != nil {
		customerID = sub.Customer.ID
	}

	if customerID == "" {
		slog.Error("subscription has no customer ID", slogKeySubscriptionID, sub.ID)

		return
	}

	u, err := s.userRepo.FindUserByStripeCustomerID(ctx, customerID)
	if err != nil {
		slog.Error(
			"find user by stripe customer ID",
			slogKeyError, err,
			slogKeyCustomerID, customerID,
		)

		return
	}

	if err := s.userRepo.UpdateTier(ctx, u.ID, model.TierFree); err != nil {
		slog.Error("downgrade user to free tier", slogKeyError, err, slogKeyUserID, u.ID)
	}

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
}

func (s *Service) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("unmarshal subscription", slogKeyError, err)

		return
	}

	status := string(sub.Status)

	if err := s.userRepo.UpdateSubscriptionStatus(ctx, sub.ID, status); err != nil {
		slog.Error("update subscription status", slogKeyError, err, slogKeySubscriptionID, sub.ID)
	}

	// In stripe-go v82, CurrentPeriodStart/End live on SubscriptionItem, not Subscription.
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		periodStart := time.Unix(item.CurrentPeriodStart, 0)
		periodEnd := time.Unix(item.CurrentPeriodEnd, 0)

		if err := s.userRepo.UpdateSubscriptionPeriod(ctx, sub.ID, periodStart, periodEnd); err != nil {
			slog.Error("update subscription period", slogKeyError, err, slogKeySubscriptionID, sub.ID)
		}
	}

	var customerID string
	if sub.Customer != nil {
		customerID = sub.Customer.ID
	}

	if customerID == "" {
		slog.Error("subscription has no customer ID", slogKeySubscriptionID, sub.ID)

		return
	}

	u, err := s.userRepo.FindUserByStripeCustomerID(ctx, customerID)
	if err != nil {
		slog.Error(
			"find user by stripe customer ID",
			slogKeyError, err,
			slogKeyCustomerID, customerID,
		)

		return
	}

	tier := tierForStatus(sub.Status)

	if err := s.userRepo.UpdateTier(ctx, u.ID, tier); err != nil {
		slog.Error("update user tier", slogKeyError, err, slogKeyUserID, u.ID, "tier", tier)
	}
}

func tierForStatus(status stripe.SubscriptionStatus) string {
	if status == stripe.SubscriptionStatusActive {
		return model.TierPro
	}

	return model.TierFree
}
