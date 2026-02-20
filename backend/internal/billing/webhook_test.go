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

	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/bilusteknoloji/secretdrop/internal/model"
)

// webhookUserRepo is a test double that tracks all calls for webhook tests.
type webhookUserRepo struct {
	// findByID
	findByIDUser *model.User
	findByIDErr  error

	// findUserByStripeCustomerID
	findByStripeUser *model.User
	findByStripeErr  error

	// upsertSubscription
	upsertSubCalled bool
	upsertSubArg    *model.Subscription
	upsertSubErr    error

	// updateSubscriptionStatus
	updateSubStatusCalled bool
	updateSubStatusSubID  string
	updateSubStatusStatus string
	updateSubStatusErr    error

	// updateTier
	updateTierCalled bool
	updateTierUserID int64
	updateTierTier   string
	updateTierErr    error

	// resetSecretsUsed
	resetSecretsUsedCalled bool
	resetSecretsUsedUserID int64
	resetSecretsUsedErr    error

	// subscription lookup
	subscription    *model.Subscription
	subscriptionErr error
}

func (m *webhookUserRepo) Upsert(_ context.Context, _ *model.User) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *webhookUserRepo) FindByID(_ context.Context, id int64) (*model.User, error) {
	return m.findByIDUser, m.findByIDErr
}

func (m *webhookUserRepo) FindByProvider(_ context.Context, _, _ string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *webhookUserRepo) IncrementSecretsUsed(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *webhookUserRepo) ResetSecretsUsed(_ context.Context, id int64) error {
	m.resetSecretsUsedCalled = true
	m.resetSecretsUsedUserID = id

	return m.resetSecretsUsedErr
}

func (m *webhookUserRepo) UpdateTier(_ context.Context, id int64, tier string) error {
	m.updateTierCalled = true
	m.updateTierUserID = id
	m.updateTierTier = tier

	return m.updateTierErr
}

func (m *webhookUserRepo) UpsertSubscription(_ context.Context, sub *model.Subscription) error {
	m.upsertSubCalled = true
	m.upsertSubArg = sub

	return m.upsertSubErr
}

func (m *webhookUserRepo) FindSubscriptionByUserID(_ context.Context, _ int64) (*model.Subscription, error) {
	return m.subscription, m.subscriptionErr
}

func (m *webhookUserRepo) FindUserByStripeCustomerID(_ context.Context, _ string) (*model.User, error) {
	return m.findByStripeUser, m.findByStripeErr
}

func (m *webhookUserRepo) UpdateSubscriptionStatus(_ context.Context, subID, status string) error {
	m.updateSubStatusCalled = true
	m.updateSubStatusSubID = subID
	m.updateSubStatusStatus = status

	return m.updateSubStatusErr
}

const (
	testWebhookSecret   = "whsec_test_secret"
	testEventCreatedVal = 1_234_567_890
)

func buildEventPayload(t *testing.T, eventType string, dataObj any) []byte {
	t.Helper()

	raw, err := json.Marshal(dataObj)
	if err != nil {
		t.Fatalf("marshal data object: %v", err)
	}

	event := map[string]any{
		"id":      "evt_test",
		"object":  "event",
		"type":    eventType,
		"created": testEventCreatedVal,
		"data": map[string]any{
			"object": json.RawMessage(raw),
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	return payload
}

func newWebhookTestService(t *testing.T, repo *webhookUserRepo) *Service {
	t.Helper()

	svc, err := New(
		"sk_test_key",
		testWebhookSecret,
		"price_test",
		repo,
		WithStripeClient(&mockStripeClient{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return svc
}

func signPayload(t *testing.T, payload []byte) string {
	t.Helper()

	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    testWebhookSecret,
		Timestamp: time.Now(),
	})

	return signed.Header
}

func TestHandleWebhook_CheckoutCompleted(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"customer_email":      "user@example.com",
		"client_reference_id": "42",
	}

	payload := buildEventPayload(t, "checkout.session.completed", dataObj)

	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if !repo.upsertSubCalled {
		t.Fatal("UpsertSubscription was not called")
	}

	if repo.upsertSubArg.UserID != 42 {
		t.Errorf("subscription user ID = %d; want %d", repo.upsertSubArg.UserID, 42)
	}

	if repo.upsertSubArg.StripeCustomerID != "cus_test_123" {
		t.Errorf("subscription customer ID = %q; want %q", repo.upsertSubArg.StripeCustomerID, "cus_test_123")
	}

	if repo.upsertSubArg.StripeSubscriptionID != "sub_test_123" {
		t.Errorf("subscription ID = %q; want %q", repo.upsertSubArg.StripeSubscriptionID, "sub_test_123")
	}

	if repo.upsertSubArg.Status != model.SubscriptionActive {
		t.Errorf("subscription status = %q; want %q", repo.upsertSubArg.Status, model.SubscriptionActive)
	}

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier was not called")
	}

	if repo.updateTierUserID != 42 {
		t.Errorf("tier update user ID = %d; want %d", repo.updateTierUserID, 42)
	}

	if repo.updateTierTier != model.TierPro {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierPro)
	}
}

func TestHandleWebhook_InvoicePaid(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro, SecretsUsed: 5},
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "in_test_123",
		"object":   "invoice",
		"customer": "cus_test_123",
	}

	payload := buildEventPayload(t, "invoice.paid", dataObj)

	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if !repo.resetSecretsUsedCalled {
		t.Fatal("ResetSecretsUsed was not called")
	}

	if repo.resetSecretsUsedUserID != 42 {
		t.Errorf("reset secrets user ID = %d; want %d", repo.resetSecretsUsedUserID, 42)
	}
}

func TestHandleWebhook_SubscriptionDeleted(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro},
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "sub_test_123",
		"object":   "subscription",
		"customer": "cus_test_123",
		"status":   "canceled",
	}

	payload := buildEventPayload(t, "customer.subscription.deleted", dataObj)

	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if !repo.updateSubStatusCalled {
		t.Fatal("UpdateSubscriptionStatus was not called")
	}

	if repo.updateSubStatusSubID != "sub_test_123" {
		t.Errorf("subscription ID = %q; want %q", repo.updateSubStatusSubID, "sub_test_123")
	}

	if repo.updateSubStatusStatus != model.SubscriptionCanceled {
		t.Errorf("subscription status = %q; want %q", repo.updateSubStatusStatus, model.SubscriptionCanceled)
	}

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier was not called")
	}

	if repo.updateTierUserID != 42 {
		t.Errorf("tier update user ID = %d; want %d", repo.updateTierUserID, 42)
	}

	if repo.updateTierTier != model.TierFree {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierFree)
	}
}

func TestHandleWebhook_SubscriptionUpdated_PastDue(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro},
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "sub_test_123",
		"object":   "subscription",
		"customer": "cus_test_123",
		"status":   "past_due",
	}

	payload := buildEventPayload(t, "customer.subscription.updated", dataObj)

	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if !repo.updateSubStatusCalled {
		t.Fatal("UpdateSubscriptionStatus was not called")
	}

	if repo.updateSubStatusStatus != model.SubscriptionPastDue {
		t.Errorf("subscription status = %q; want %q", repo.updateSubStatusStatus, model.SubscriptionPastDue)
	}

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier was not called")
	}

	if repo.updateTierTier != model.TierFree {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierFree)
	}
}

func TestHandleWebhook_SubscriptionUpdated_Active(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierFree},
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "sub_test_123",
		"object":   "subscription",
		"customer": "cus_test_123",
		"status":   "active",
	}

	payload := buildEventPayload(t, "customer.subscription.updated", dataObj)

	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier was not called")
	}

	if repo.updateTierTier != model.TierPro {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierPro)
	}
}

func TestHandleWebhook_InvalidSignature(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	payload := []byte(`{"id":"evt_test","type":"checkout.session.completed"}`)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", "t=123,v1=invalidsignature")

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	var resp errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "invalid_signature" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_signature")
	}
}

func TestHandleWebhook_UnhandledEvent(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	payload := buildEventPayload(t, "charge.succeeded", map[string]any{
		"id":     "ch_test_123",
		"object": "charge",
	})

	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	// Verify no repo methods were called.
	if repo.upsertSubCalled {
		t.Error("UpsertSubscription should not be called for unhandled events")
	}

	if repo.updateTierCalled {
		t.Error("UpdateTier should not be called for unhandled events")
	}

	if repo.resetSecretsUsedCalled {
		t.Error("ResetSecretsUsed should not be called for unhandled events")
	}

	if repo.updateSubStatusCalled {
		t.Error("UpdateSubscriptionStatus should not be called for unhandled events")
	}
}
