package billing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/model"
)

// mockStripeClient is a test double for StripeClient.
type mockStripeClient struct {
	checkoutSession *stripe.CheckoutSession
	checkoutErr     error
	portalSession   *stripe.BillingPortalSession
	portalErr       error
}

func (m *mockStripeClient) CreateCheckoutSession(
	_ context.Context,
	_ *stripe.CheckoutSessionCreateParams,
) (*stripe.CheckoutSession, error) {
	return m.checkoutSession, m.checkoutErr
}

func (m *mockStripeClient) CreatePortalSession(
	_ context.Context,
	_ *stripe.BillingPortalSessionCreateParams,
) (*stripe.BillingPortalSession, error) {
	return m.portalSession, m.portalErr
}

// mockUserRepo is a minimal test double for user.Repository.
type mockUserRepo struct {
	subscription    *model.Subscription
	subscriptionErr error
}

func (m *mockUserRepo) Upsert(_ context.Context, _ *model.User) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) FindByID(_ context.Context, _ int64) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) FindByProvider(_ context.Context, _, _ string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) IncrementSecretsUsed(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) ResetSecretsUsed(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) UpdateTier(_ context.Context, _ int64, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) UpsertSubscription(_ context.Context, _ *model.Subscription) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) FindSubscriptionByUserID(_ context.Context, _ int64) (*model.Subscription, error) {
	return m.subscription, m.subscriptionErr
}

func (m *mockUserRepo) FindUserByStripeCustomerID(_ context.Context, _ string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) UpdateSubscriptionStatus(_ context.Context, _, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) UpdateSubscriptionPeriod(_ context.Context, _ string, _, _ time.Time) error {
	return errors.New("not implemented")
}

type errorEnvelope struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func newTestService(t *testing.T, sc StripeClient, repo *mockUserRepo) *Service {
	t.Helper()

	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_test",
		repo,
		WithStripeClient(sc),
		WithSuccessURL("https://example.com/success"),
		WithCancelURL("https://example.com/cancel"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return svc
}

func TestNew_Validation(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	tests := []struct {
		name          string
		secretKey     string
		webhookSecret string
		priceID       string
		wantErr       string
	}{
		{
			name:          "empty secret key",
			secretKey:     "",
			webhookSecret: "whsec_test",
			priceID:       "price_test",
			wantErr:       "stripe secret key cannot be empty",
		},
		{
			name:          "empty webhook secret",
			secretKey:     "sk_test",
			webhookSecret: "",
			priceID:       "price_test",
			wantErr:       "stripe webhook secret cannot be empty",
		},
		{
			name:          "empty price ID",
			secretKey:     "sk_test",
			webhookSecret: "whsec_test",
			priceID:       "",
			wantErr:       "stripe price ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(tt.secretKey, tt.webhookSecret, tt.priceID, repo)
			if err == nil {
				t.Fatal("New() error = nil; want error")
			}

			if err.Error() != tt.wantErr {
				t.Errorf("error = %q; want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNew_WithOptions(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_test",
		repo,
		WithSuccessURL("https://example.com/success"),
		WithCancelURL("https://example.com/cancel"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.successURL != "https://example.com/success" {
		t.Errorf("successURL = %q; want %q", svc.successURL, "https://example.com/success")
	}

	if svc.cancelURL != "https://example.com/cancel" {
		t.Errorf("cancelURL = %q; want %q", svc.cancelURL, "https://example.com/cancel")
	}
}

func TestNew_Valid(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	svc, err := New("sk_test_key", "whsec_test", "price_test", repo)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.webhookSecret != "whsec_test" {
		t.Errorf("webhookSecret = %q; want %q", svc.webhookSecret, "whsec_test")
	}

	if svc.priceID != "price_test" {
		t.Errorf("priceID = %q; want %q", svc.priceID, "price_test")
	}
}

func TestWebhookSecret(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	svc, err := New("sk_test_key", "whsec_test_secret", "price_test", repo)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := svc.WebhookSecret(); got != "whsec_test_secret" {
		t.Errorf("WebhookSecret() = %q; want %q", got, "whsec_test_secret")
	}
}

func TestHandleCheckout_Unauthenticated(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", nil)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "unauthorized" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "unauthorized")
	}

	if resp.Error.Message != "Authentication required" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Authentication required")
	}
}

func TestHandleCheckout_Success(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		checkoutSession: &stripe.CheckoutSession{URL: "https://checkout.stripe.com/session123"},
	}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["url"] != "https://checkout.stripe.com/session123" {
		t.Errorf("url = %q; want %q", resp["url"], "https://checkout.stripe.com/session123")
	}
}

func TestHandleCheckout_StripeError(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		checkoutErr: errors.New("stripe API error"),
	}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "internal_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "internal_error")
	}

	if resp.Error.Message != "Failed to create checkout session" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to create checkout session")
	}
}

func TestHandlePortal_Unauthenticated(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	req := httptest.NewRequest(http.MethodPost, "/billing/portal", nil)
	rec := httptest.NewRecorder()

	svc.HandlePortal().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "unauthorized" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "unauthorized")
	}

	if resp.Error.Message != "Authentication required" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Authentication required")
	}
}

func TestHandlePortal_NoSubscription(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{
		subscriptionErr: model.ErrNotFound,
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	req := httptest.NewRequest(http.MethodPost, "/billing/portal", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandlePortal().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "not_found" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "not_found")
	}

	if resp.Error.Message != "No active subscription found" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "No active subscription found")
	}
}

func TestHandlePortal_SubscriptionDBError(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{
		subscriptionErr: errors.New("database connection failed"),
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	req := httptest.NewRequest(http.MethodPost, "/billing/portal", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandlePortal().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "internal_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "internal_error")
	}
}

func TestHandlePortal_Success(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		portalSession: &stripe.BillingPortalSession{URL: "https://billing.stripe.com/portal123"},
	}
	repo := &mockUserRepo{
		subscription: &model.Subscription{
			StripeCustomerID: "cus_test123",
		},
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "pro"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	req := httptest.NewRequest(http.MethodPost, "/billing/portal", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandlePortal().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["url"] != "https://billing.stripe.com/portal123" {
		t.Errorf("url = %q; want %q", resp["url"], "https://billing.stripe.com/portal123")
	}
}

func TestHandlePortal_StripeError(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		portalErr: errors.New("stripe API error"),
	}
	repo := &mockUserRepo{
		subscription: &model.Subscription{
			StripeCustomerID: "cus_test123",
		},
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "pro"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	req := httptest.NewRequest(http.MethodPost, "/billing/portal", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandlePortal().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "internal_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "internal_error")
	}

	if resp.Error.Message != "Failed to create portal session" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to create portal session")
	}
}
