package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/slack"
)

const (
	maxWebhookBodySize = 65536 // 64KB

	retryBackoff1s  = 1
	retryBackoff2s  = 2
	retryBackoff4s  = 4
	retryBackoff8s  = 8
	retryBackoff12s = 12

	slogKeySubscriptionID = "subscription_id"
	slogKeyUserID         = "user_id"
	slogKeyCustomerID     = "customer_id"
)

var defaultRetryBackoffs = []time.Duration{
	retryBackoff1s * time.Second,
	retryBackoff2s * time.Second,
	retryBackoff4s * time.Second,
	retryBackoff8s * time.Second,
	retryBackoff12s * time.Second,
}

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

		if s.projectMetaKey != "" && !s.matchesProjectMetadata(event) {
			slog.Debug("skipping webhook event (project mismatch)", "type", event.Type)
			w.WriteHeader(http.StatusOK)

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

	// Determine tier from session metadata (set during checkout)
	tier := session.Metadata["tier"]
	if tier == "" {
		tier = model.TierPro // fallback for legacy sessions
	}

	if err := s.userRepo.UpdateTier(ctx, userID, tier); err != nil {
		slog.Error("update user tier", slogKeyError, err, slogKeyUserID, userID, "tier", tier)
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

	u, err := s.findUserWithRetry(ctx, customerID)
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

	u, err := s.findUserWithRetry(ctx, customerID)
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

	u, err := s.findUserWithRetry(ctx, customerID)
	if err != nil {
		slog.Error(
			"find user by stripe customer ID",
			slogKeyError, err,
			slogKeyCustomerID, customerID,
		)

		return
	}

	priceID := ""
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		if sub.Items.Data[0].Price != nil {
			priceID = sub.Items.Data[0].Price.ID
		}
	}

	tier := s.resolveTier(ctx, sub.Status, priceID)

	if err := s.userRepo.UpdateTier(ctx, u.ID, tier); err != nil {
		slog.Error("update user tier", slogKeyError, err, slogKeyUserID, u.ID, "tier", tier)
	}
}

// matchesProjectMetadata checks if a Stripe event belongs to this project
// by inspecting metadata on the event's data object. Different event types
// carry metadata in different locations (session, subscription, invoice).
func (s *Service) matchesProjectMetadata(event stripe.Event) bool {
	var raw struct {
		Metadata map[string]string `json:"metadata"`
	}

	if err := json.Unmarshal(event.Data.Raw, &raw); err != nil {
		return false
	}

	return raw.Metadata[s.projectMetaKey] == s.projectMetaVal
}

func (s *Service) findUserWithRetry(ctx context.Context, customerID string) (*model.User, error) {
	u, err := s.userRepo.FindUserByStripeCustomerID(ctx, customerID)
	if err == nil {
		return u, nil
	}

	if !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("find user by stripe customer ID: %w", err)
	}

	for _, backoff := range s.retryBackoffs {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("find user by stripe customer ID: %w", ctx.Err())
		case <-time.After(backoff):
		}

		u, err = s.userRepo.FindUserByStripeCustomerID(ctx, customerID)
		if err == nil {
			return u, nil
		}

		if !errors.Is(err, model.ErrNotFound) {
			return nil, fmt.Errorf("find user by stripe customer ID: %w", err)
		}
	}

	return nil, fmt.Errorf("find user by stripe customer ID after retries: %w", err)
}

func (s *Service) resolveTier(ctx context.Context, status stripe.SubscriptionStatus, priceID string) string {
	if status != stripe.SubscriptionStatusActive {
		return model.TierFree
	}

	if priceID != "" {
		tier, err := s.userRepo.FindTierByPriceID(ctx, priceID)
		if err == nil {
			return tier
		}

		slog.Warn("resolve tier from price ID", slogKeyError, err, "price_id", priceID)
	}

	return model.TierPro
}
