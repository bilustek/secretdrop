package billing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"

	"github.com/bilustek/secretdrop/internal/auth"
	"github.com/bilustek/secretdrop/internal/middleware"
	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/slack"
	"github.com/bilustek/secretdrop/internal/user"
)

// mockNotifier is a test double for slack.Notifier.
type mockNotifier struct {
	called bool
	event  slack.Event
	err    error
}

func (m *mockNotifier) Notify(_ context.Context, ev slack.Event) error {
	m.called = true
	m.event = ev

	return m.err
}

// mockStripeClient is a test double for StripeClient.
type mockStripeClient struct {
	checkoutSession *stripe.CheckoutSession
	checkoutErr     error
	checkoutParams  *stripe.CheckoutSessionCreateParams
	portalSession   *stripe.BillingPortalSession
	portalErr       error
	portalParams    *stripe.BillingPortalSessionCreateParams
	cancelErr       error
	cancelCalledID  string
}

func (m *mockStripeClient) CreateCheckoutSession(
	_ context.Context,
	params *stripe.CheckoutSessionCreateParams,
) (*stripe.CheckoutSession, error) {
	m.checkoutParams = params
	return m.checkoutSession, m.checkoutErr
}

func (m *mockStripeClient) CreatePortalSession(
	_ context.Context,
	params *stripe.BillingPortalSessionCreateParams,
) (*stripe.BillingPortalSession, error) {
	m.portalParams = params
	return m.portalSession, m.portalErr
}

func (m *mockStripeClient) CancelSubscription(_ context.Context, id string) error {
	m.cancelCalledID = id
	return m.cancelErr
}

// mockUserRepo is a minimal test double for user.Repository.
type mockUserRepo struct {
	subscription    *model.Subscription
	subscriptionErr error
	tierLimits      *user.TierLimits
	tierLimitsErr   error
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

func (m *mockUserRepo) UpdateTimezone(_ context.Context, _ int64, _ string) error {
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

func (m *mockUserRepo) DeleteUser(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) GetLimits(_ context.Context, _ string) (*user.TierLimits, error) {
	if m.tierLimitsErr != nil {
		return nil, m.tierLimitsErr
	}

	if m.tierLimits != nil {
		return m.tierLimits, nil
	}

	return nil, model.ErrNotFound
}

func (m *mockUserRepo) FindTierByPriceID(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockUserRepo) ListLimits(_ context.Context) ([]*user.TierLimits, error) {
	return nil, nil
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

func TestNew_OptionError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}
	failOpt := func(_ *Service) error {
		return errors.New("option failed")
	}

	_, err := New("sk_test_key", "whsec_test", "price_test", repo, failOpt)
	if err == nil {
		t.Fatal("New() error = nil; want error")
	}

	if !strings.Contains(err.Error(), "apply option") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "apply option")
	}
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
	repo := &mockUserRepo{
		tierLimits: &user.TierLimits{
			Tier:          "pro",
			StripePriceID: "price_from_db",
		},
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
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

	if sc.checkoutParams.LineItems[0].Price == nil || *sc.checkoutParams.LineItems[0].Price != "price_from_db" {
		t.Errorf("price = %v; want %q", sc.checkoutParams.LineItems[0].Price, "price_from_db")
	}
}

func TestHandleCheckout_StripeError(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		checkoutErr: errors.New("stripe API error"),
	}
	repo := &mockUserRepo{
		tierLimits: &user.TierLimits{
			Tier:          "pro",
			StripePriceID: "price_from_db",
		},
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
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

func TestWithPortalReturnURL(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_test",
		repo,
		WithPortalReturnURL("https://example.com/dashboard"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.portalReturnURL != "https://example.com/dashboard" {
		t.Errorf("portalReturnURL = %q; want %q", svc.portalReturnURL, "https://example.com/dashboard")
	}
}

func TestWithNotifier(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}
	notifier := &mockNotifier{}

	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_test",
		repo,
		WithNotifier(notifier),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.notifier == nil {
		t.Fatal("notifier should not be nil")
	}
}

func TestCancelSubscription_Success(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	err := svc.CancelSubscription(context.Background(), "sub_test_123")
	if err != nil {
		t.Fatalf("CancelSubscription() error = %v; want nil", err)
	}

	if sc.cancelCalledID != "sub_test_123" {
		t.Errorf("cancel called with ID = %q; want %q", sc.cancelCalledID, "sub_test_123")
	}
}

func TestCancelSubscription_Error(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		cancelErr: errors.New("stripe cancel error"),
	}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	err := svc.CancelSubscription(context.Background(), "sub_test_123")
	if err == nil {
		t.Fatal("CancelSubscription() error = nil; want error")
	}
}

func TestHandleCheckout_WithProjectMetadata(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		checkoutSession: &stripe.CheckoutSession{URL: "https://checkout.stripe.com/session123"},
	}
	repo := &mockUserRepo{
		tierLimits: &user.TierLimits{
			Tier:          "pro",
			StripePriceID: "price_from_db",
		},
	}

	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_test",
		repo,
		WithStripeClient(sc),
		WithSuccessURL("https://example.com/success"),
		WithCancelURL("https://example.com/cancel"),
		WithProjectMetadata("project", "secretdrop"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if sc.checkoutParams == nil {
		t.Fatal("checkout params should not be nil")
	}

	if sc.checkoutParams.Metadata == nil {
		t.Fatal("metadata should not be nil when project metadata is set")
	}

	if sc.checkoutParams.Metadata["project"] != "secretdrop" {
		t.Errorf("metadata[project] = %q; want %q", sc.checkoutParams.Metadata["project"], "secretdrop")
	}

	if sc.checkoutParams.SubscriptionData == nil {
		t.Fatal("subscription data should not be nil when project metadata is set")
	}
}

func TestHandleCheckout_InvalidRequestBody(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "invalid_request" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_request")
	}

	if resp.Error.Message != "Invalid request body" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Invalid request body")
	}
}

func TestHandleCheckout_EmptyTier(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":""}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "invalid_request" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_request")
	}

	if resp.Error.Message != "Invalid tier" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Invalid tier")
	}
}

func TestHandleCheckout_FreeTier(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"free"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "invalid_request" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_request")
	}

	if resp.Error.Message != "Invalid tier" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Invalid tier")
	}
}

func TestHandleCheckout_TierNotFound(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{
		tierLimitsErr: model.ErrNotFound,
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"enterprise"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

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

	if resp.Error.Message != "Plan not found" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Plan not found")
	}
}

func TestHandleCheckout_GetLimitsDBError(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{
		tierLimitsErr: errors.New("database connection failed"),
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
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

	if resp.Error.Message != "Failed to look up plan" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to look up plan")
	}
}

func TestHandleCheckout_FallbackToLegacyPriceID(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		checkoutSession: &stripe.CheckoutSession{URL: "https://checkout.stripe.com/legacy"},
	}
	repo := &mockUserRepo{
		tierLimits: &user.TierLimits{
			Tier:          "pro",
			StripePriceID: "", // no price ID in DB
		},
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["url"] != "https://checkout.stripe.com/legacy" {
		t.Errorf("url = %q; want %q", resp["url"], "https://checkout.stripe.com/legacy")
	}

	// Should fall back to the legacy priceID ("price_test") from newTestService.
	if sc.checkoutParams.LineItems[0].Price == nil || *sc.checkoutParams.LineItems[0].Price != "price_test" {
		t.Errorf("price = %v; want %q", sc.checkoutParams.LineItems[0].Price, "price_test")
	}
}

func TestHandleCheckout_NoPriceIDAnywhere(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{}
	repo := &mockUserRepo{
		tierLimits: &user.TierLimits{
			Tier:          "pro",
			StripePriceID: "", // no price ID in DB
		},
	}

	// Create service with empty legacy priceID — must use New directly with a
	// dummy priceID then clear it, since New() validates priceID is non-empty.
	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_placeholder",
		repo,
		WithStripeClient(sc),
		WithSuccessURL("https://example.com/success"),
		WithCancelURL("https://example.com/cancel"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Clear the legacy priceID to simulate no fallback available.
	svc.priceID = ""

	claims := &auth.Claims{UserID: 42, Email: "user@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "configuration_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "configuration_error")
	}

	if resp.Error.Message != "Plan not configured for billing" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Plan not configured for billing")
	}
}

func TestHandleCheckout_TierMetadataIncluded(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		checkoutSession: &stripe.CheckoutSession{URL: "https://checkout.stripe.com/session"},
	}
	repo := &mockUserRepo{
		tierLimits: &user.TierLimits{
			Tier:          "team",
			StripePriceID: "price_team",
		},
	}
	svc := newTestService(t, sc, repo)

	claims := &auth.Claims{UserID: 99, Email: "team@example.com", Tier: "free"}
	ctx := middleware.ContextWithUser(context.Background(), claims)

	body := strings.NewReader(`{"tier":"team"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/checkout", body).WithContext(ctx)
	rec := httptest.NewRecorder()

	svc.HandleCheckout().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if sc.checkoutParams.Metadata == nil {
		t.Fatal("metadata should not be nil")
	}

	if sc.checkoutParams.Metadata["tier"] != "team" {
		t.Errorf("metadata[tier] = %q; want %q", sc.checkoutParams.Metadata["tier"], "team")
	}

	if sc.checkoutParams.SubscriptionData == nil {
		t.Fatal("subscription data should not be nil")
	}

	if sc.checkoutParams.SubscriptionData.Metadata["tier"] != "team" {
		t.Errorf("subscription metadata[tier] = %q; want %q", sc.checkoutParams.SubscriptionData.Metadata["tier"], "team")
	}
}

func TestWithPortalConfigID(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}

	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_test",
		repo,
		WithPortalConfigID("bpc_test_config"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.portalConfigID != "bpc_test_config" {
		t.Errorf("portalConfigID = %q; want %q", svc.portalConfigID, "bpc_test_config")
	}
}

func TestHandlePortal_WithPortalConfigID(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		portalSession: &stripe.BillingPortalSession{URL: "https://billing.stripe.com/portal_config"},
	}
	repo := &mockUserRepo{
		subscription: &model.Subscription{
			StripeCustomerID: "cus_test456",
		},
	}

	svc, err := New(
		"sk_test_key",
		"whsec_test",
		"price_test",
		repo,
		WithStripeClient(sc),
		WithSuccessURL("https://example.com/success"),
		WithCancelURL("https://example.com/cancel"),
		WithPortalReturnURL("https://example.com/dashboard"),
		WithPortalConfigID("bpc_test_config"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

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

	if resp["url"] != "https://billing.stripe.com/portal_config" {
		t.Errorf("url = %q; want %q", resp["url"], "https://billing.stripe.com/portal_config")
	}

	if sc.portalParams == nil {
		t.Fatal("portal params should not be nil")
	}

	if sc.portalParams.Configuration == nil || *sc.portalParams.Configuration != "bpc_test_config" {
		t.Errorf("portal configuration = %v; want %q", sc.portalParams.Configuration, "bpc_test_config")
	}

	if sc.portalParams.ReturnURL == nil || *sc.portalParams.ReturnURL != "https://example.com/dashboard" {
		t.Errorf("portal return URL = %v; want %q", sc.portalParams.ReturnURL, "https://example.com/dashboard")
	}
}

func TestHandlePortal_WithoutPortalConfigID(t *testing.T) {
	t.Parallel()

	sc := &mockStripeClient{
		portalSession: &stripe.BillingPortalSession{URL: "https://billing.stripe.com/portal_no_config"},
	}
	repo := &mockUserRepo{
		subscription: &model.Subscription{
			StripeCustomerID: "cus_test789",
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

	if sc.portalParams == nil {
		t.Fatal("portal params should not be nil")
	}

	if sc.portalParams.Configuration != nil {
		t.Errorf("portal configuration = %v; want nil (no config ID set)", sc.portalParams.Configuration)
	}
}
