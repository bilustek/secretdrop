package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/stripe/stripe-go/v82"

	"github.com/bilustek/secretdrop/internal/middleware"
	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/slack"
	"github.com/bilustek/secretdrop/internal/user"
)

const slogKeyError = "error"

// StripeClient defines the Stripe operations needed by the billing service.
type StripeClient interface {
	CreateCheckoutSession(
		ctx context.Context,
		params *stripe.CheckoutSessionCreateParams,
	) (*stripe.CheckoutSession, error)

	CreatePortalSession(
		ctx context.Context,
		params *stripe.BillingPortalSessionCreateParams,
	) (*stripe.BillingPortalSession, error)

	CancelSubscription(ctx context.Context, id string) error
}

// stripeClientAdapter wraps a stripe.Client to implement StripeClient.
type stripeClientAdapter struct {
	client *stripe.Client
}

func (a *stripeClientAdapter) CreateCheckoutSession(
	ctx context.Context,
	params *stripe.CheckoutSessionCreateParams,
) (*stripe.CheckoutSession, error) {
	sess, err := a.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create checkout session: %w", err)
	}

	return sess, nil
}

func (a *stripeClientAdapter) CreatePortalSession(
	ctx context.Context,
	params *stripe.BillingPortalSessionCreateParams,
) (*stripe.BillingPortalSession, error) {
	sess, err := a.client.V1BillingPortalSessions.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create portal session: %w", err)
	}

	return sess, nil
}

func (a *stripeClientAdapter) CancelSubscription(ctx context.Context, id string) error {
	_, err := a.client.V1Subscriptions.Cancel(ctx, id, &stripe.SubscriptionCancelParams{})
	if err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}

	return nil
}

// Option configures the billing Service.
type Option func(*Service) error

// Service handles Stripe billing operations.
type Service struct {
	priceID         string
	webhookSecret   string
	userRepo        user.Repository
	stripeClient    StripeClient
	successURL      string
	cancelURL       string
	portalReturnURL string
	notifier        slack.Notifier
	projectMetaKey  string
	projectMetaVal  string
}

// New creates a new billing Service.
func New(
	secretKey, webhookSecret, priceID string,
	userRepo user.Repository,
	opts ...Option,
) (*Service, error) {
	if secretKey == "" {
		return nil, errors.New("stripe secret key cannot be empty")
	}

	if webhookSecret == "" {
		return nil, errors.New("stripe webhook secret cannot be empty")
	}

	if priceID == "" {
		return nil, errors.New("stripe price ID cannot be empty")
	}

	sc := stripe.NewClient(secretKey)

	s := &Service{
		priceID:       priceID,
		webhookSecret: webhookSecret,
		userRepo:      userRepo,
		stripeClient:  &stripeClientAdapter{client: sc},
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return s, nil
}

// WithSuccessURL sets the URL to redirect to after successful checkout.
func WithSuccessURL(url string) Option {
	return func(s *Service) error {
		s.successURL = url

		return nil
	}
}

// WithCancelURL sets the URL to redirect to if checkout is canceled.
func WithCancelURL(url string) Option {
	return func(s *Service) error {
		s.cancelURL = url

		return nil
	}
}

// WithPortalReturnURL sets the URL to redirect to after leaving the customer portal.
func WithPortalReturnURL(url string) Option {
	return func(s *Service) error {
		s.portalReturnURL = url

		return nil
	}
}

// WithNotifier sets the Slack notifier for subscription events.
func WithNotifier(n slack.Notifier) Option {
	return func(s *Service) error {
		s.notifier = n

		return nil
	}
}

// WithProjectMetadata sets the metadata key-value pair for filtering Stripe events
// by project. When set, only webhook events with matching metadata are processed.
func WithProjectMetadata(key, value string) Option {
	return func(s *Service) error {
		s.projectMetaKey = key
		s.projectMetaVal = value

		return nil
	}
}

// WithStripeClient replaces the default Stripe client.
// This is primarily useful for testing.
func WithStripeClient(sc StripeClient) Option {
	return func(s *Service) error {
		s.stripeClient = sc

		return nil
	}
}

// WebhookSecret returns the webhook signing secret.
func (s *Service) WebhookSecret() string { return s.webhookSecret }

// CancelSubscription cancels a Stripe subscription by its ID.
func (s *Service) CancelSubscription(ctx context.Context, stripeSubID string) error {
	if err := s.stripeClient.CancelSubscription(ctx, stripeSubID); err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}

	return nil
}

// HandleCheckout creates a Stripe Checkout Session and returns the URL.
// Requires authentication (user must be in context).
func (s *Service) HandleCheckout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)

			return
		}

		params := &stripe.CheckoutSessionCreateParams{
			Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
			LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
				{
					Price:    stripe.String(s.priceID),
					Quantity: stripe.Int64(1),
				},
			},
			SuccessURL:    stripe.String(s.successURL),
			CancelURL:     stripe.String(s.cancelURL),
			CustomerEmail: stripe.String(claims.Email),
			ClientReferenceID: stripe.String(
				fmt.Sprintf("%d", claims.UserID),
			),
		}

		if s.projectMetaKey != "" && s.projectMetaVal != "" {
			meta := map[string]string{s.projectMetaKey: s.projectMetaVal}
			params.Metadata = meta
			params.SubscriptionData = &stripe.CheckoutSessionCreateSubscriptionDataParams{
				Metadata: meta,
			}
		}

		sess, err := s.stripeClient.CreateCheckoutSession(r.Context(), params)
		if err != nil {
			slog.Error("create checkout session", slogKeyError, err)
			writeError(
				w, "internal_error",
				"Failed to create checkout session",
				http.StatusInternalServerError,
			)

			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"url": sess.URL})
	}
}

// HandlePortal creates a Stripe Customer Portal session and returns the URL.
// Requires authentication.
func (s *Service) HandlePortal() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)

			return
		}

		sub, err := s.userRepo.FindSubscriptionByUserID(r.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				writeError(w, "not_found", "No active subscription found", http.StatusNotFound)
			} else {
				slog.Error("find subscription", slogKeyError, err)
				writeError(
					w, "internal_error",
					"Failed to find subscription",
					http.StatusInternalServerError,
				)
			}

			return
		}

		params := &stripe.BillingPortalSessionCreateParams{
			Customer:  stripe.String(sub.StripeCustomerID),
			ReturnURL: stripe.String(s.portalReturnURL),
		}

		sess, err := s.stripeClient.CreatePortalSession(r.Context(), params)
		if err != nil {
			slog.Error("create portal session", slogKeyError, err)
			writeError(
				w, "internal_error",
				"Failed to create portal session",
				http.StatusInternalServerError,
			)

			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"url": sess.URL})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode json response", slogKeyError, err)
	}
}

func writeError(w http.ResponseWriter, errType, message string, status int) {
	writeJSON(w, status, model.ErrorResponse{
		Error: model.ErrorDetail{Type: errType, Message: message},
	})
}
