package billing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/slack"
	"github.com/bilustek/secretdrop/internal/user"
)

// syncNotifier is a test double for slack.Notifier that supports waiting
// for the goroutine to complete in tests. Call expect() before the handler
// runs and wait() after to synchronize with the background goroutine.
type syncNotifier struct {
	mu     sync.Mutex
	wg     sync.WaitGroup
	called bool
	event  slack.Event
	err    error
}

func (n *syncNotifier) Notify(_ context.Context, ev slack.Event) error {
	defer n.wg.Done()

	n.mu.Lock()
	n.called = true
	n.event = ev
	n.mu.Unlock()

	return n.err
}

// expect registers that we expect one notification call.
func (n *syncNotifier) expect() {
	n.wg.Add(1)
}

func (n *syncNotifier) wait() {
	n.wg.Wait()
}

type findByStripeResult struct {
	user *model.User
	err  error
}

// webhookUserRepo is a test double that tracks all calls for webhook tests.
type webhookUserRepo struct {
	// findByID
	findByIDUser *model.User
	findByIDErr  error

	// findUserByStripeCustomerID
	findByStripeUser *model.User
	findByStripeErr  error

	// sequential results for retry tests (takes priority when non-nil)
	findByStripeResults []findByStripeResult
	findByStripeCalls   int

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

	// updateSubscriptionPeriod
	updateSubPeriodCalled bool
	updateSubPeriodSubID  string
	updateSubPeriodStart  time.Time
	updateSubPeriodEnd    time.Time
	updateSubPeriodErr    error

	// subscription lookup
	subscription    *model.Subscription
	subscriptionErr error

	// findTierByPriceID
	findTierByPriceIDTier string
	findTierByPriceIDErr  error
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

func (m *webhookUserRepo) UpdateTimezone(_ context.Context, _ int64, _ string) error {
	return nil
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
	if m.findByStripeResults != nil {
		i := m.findByStripeCalls
		m.findByStripeCalls++

		if i < len(m.findByStripeResults) {
			return m.findByStripeResults[i].user, m.findByStripeResults[i].err
		}

		last := m.findByStripeResults[len(m.findByStripeResults)-1]

		return last.user, last.err
	}

	return m.findByStripeUser, m.findByStripeErr
}

func (m *webhookUserRepo) UpdateSubscriptionStatus(_ context.Context, subID, status string) error {
	m.updateSubStatusCalled = true
	m.updateSubStatusSubID = subID
	m.updateSubStatusStatus = status

	return m.updateSubStatusErr
}

func (m *webhookUserRepo) UpdateSubscriptionPeriod(_ context.Context, subID string, start, end time.Time) error {
	m.updateSubPeriodCalled = true
	m.updateSubPeriodSubID = subID
	m.updateSubPeriodStart = start
	m.updateSubPeriodEnd = end

	return m.updateSubPeriodErr
}

func (m *webhookUserRepo) DeleteUser(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *webhookUserRepo) GetLimits(_ context.Context, _ string) (*user.TierLimits, error) {
	return nil, model.ErrNotFound
}

func (m *webhookUserRepo) FindTierByPriceID(_ context.Context, _ string) (string, error) {
	return m.findTierByPriceIDTier, m.findTierByPriceIDErr
}

func (m *webhookUserRepo) ListLimits(_ context.Context) ([]*user.TierLimits, error) {
	return nil, nil
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

func newWebhookTestService(t *testing.T, repo *webhookUserRepo, opts ...Option) *Service {
	t.Helper()

	baseOpts := []Option{WithStripeClient(&mockStripeClient{})}
	baseOpts = append(baseOpts, opts...)

	svc, err := New(
		"sk_test_key",
		testWebhookSecret,
		"price_test",
		repo,
		baseOpts...,
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
		"metadata":            map[string]string{"tier": "pro"},
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

	// No price items → fallback to pro
	if repo.updateTierTier != model.TierPro {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierPro)
	}
}

func TestHandleWebhook_SubscriptionUpdated_Active_WithPriceID(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser:      &model.User{ID: 42, Tier: model.TierFree},
		findTierByPriceIDTier: model.TierTeam,
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "sub_test_123",
		"object":   "subscription",
		"customer": "cus_test_123",
		"status":   "active",
		"items": map[string]any{
			"data": []map[string]any{
				{
					"price": map[string]any{
						"id": "price_team_123",
					},
					"current_period_start": 1700000000,
					"current_period_end":   1702592000,
				},
			},
		},
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

	if repo.updateTierTier != model.TierTeam {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierTeam)
	}
}

func TestHandleWebhook_SubscriptionUpdated_Active_PriceLookupFails(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser:     &model.User{ID: 42, Tier: model.TierFree},
		findTierByPriceIDErr: errors.New("not found"),
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "sub_test_123",
		"object":   "subscription",
		"customer": "cus_test_123",
		"status":   "active",
		"items": map[string]any{
			"data": []map[string]any{
				{
					"price": map[string]any{
						"id": "price_unknown",
					},
					"current_period_start": 1700000000,
					"current_period_end":   1702592000,
				},
			},
		},
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

	// Price lookup fails → fallback to pro
	if repo.updateTierTier != model.TierPro {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierPro)
	}
}

func TestHandleWebhook_CheckoutCompleted_TierFromMetadata(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"client_reference_id": "42",
		"metadata":            map[string]string{"tier": "team"},
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

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier was not called")
	}

	if repo.updateTierTier != model.TierTeam {
		t.Errorf("tier = %q; want %q", repo.updateTierTier, model.TierTeam)
	}
}

func TestHandleWebhook_CheckoutCompleted_NoTierMetadata_FallbackPro(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
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

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier was not called")
	}

	// No metadata → fallback to pro
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

func TestHandleWebhook_ProjectMetadata_Match(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo, WithProjectMetadata("project", "secretdrop"))

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"client_reference_id": "42",
		"metadata":            map[string]string{"project": "secretdrop"},
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
		t.Fatal("UpsertSubscription should be called when metadata matches")
	}
}

func TestHandleWebhook_ProjectMetadata_Mismatch(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo, WithProjectMetadata("project", "secretdrop"))

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"client_reference_id": "42",
		"metadata":            map[string]string{"project": "talentscore"},
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

	if repo.upsertSubCalled {
		t.Error("UpsertSubscription should NOT be called when metadata mismatches")
	}

	if repo.updateTierCalled {
		t.Error("UpdateTier should NOT be called when metadata mismatches")
	}
}

func TestHandleWebhook_ProjectMetadata_NoMetadata(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo, WithProjectMetadata("project", "secretdrop"))

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
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

	if repo.upsertSubCalled {
		t.Error("UpsertSubscription should NOT be called when metadata is missing")
	}
}

func TestHandleWebhook_NoProjectFilter_ProcessesAll(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo) // no WithProjectMetadata

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
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
		t.Fatal("UpsertSubscription should be called when no project filter is set")
	}
}

func TestFindUserWithRetry_SucceedsAfterRetries(t *testing.T) {
	t.Parallel()

	wantUser := &model.User{ID: 42, Email: "test@example.com"}
	repo := &webhookUserRepo{
		findByStripeResults: []findByStripeResult{
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
			{wantUser, nil},
		},
	}

	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond}

	u, err := svc.findUserWithRetry(context.Background(), "cus_test_123")
	if err != nil {
		t.Fatalf("findUserWithRetry() error = %v; want nil", err)
	}

	if u.ID != wantUser.ID {
		t.Errorf("user ID = %d; want %d", u.ID, wantUser.ID)
	}

	if repo.findByStripeCalls != 3 {
		t.Errorf("calls = %d; want 3", repo.findByStripeCalls)
	}
}

func TestFindUserWithRetry_AllRetriesExhausted(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeResults: []findByStripeResult{
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
		},
	}

	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond}

	_, err := svc.findUserWithRetry(context.Background(), "cus_test_123")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("findUserWithRetry() error = %v; want model.ErrNotFound", err)
	}

	// 1 initial + 4 retries = 5
	if repo.findByStripeCalls != 5 {
		t.Errorf("calls = %d; want 5", repo.findByStripeCalls)
	}
}

func TestFindUserWithRetry_NonNotFoundError_NoRetry(t *testing.T) {
	t.Parallel()

	dbErr := errors.New("database connection failed")
	repo := &webhookUserRepo{
		findByStripeResults: []findByStripeResult{
			{nil, dbErr},
		},
	}

	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond}

	_, err := svc.findUserWithRetry(context.Background(), "cus_test_123")
	if !errors.Is(err, dbErr) {
		t.Errorf("findUserWithRetry() error = %v; want %v", err, dbErr)
	}

	if repo.findByStripeCalls != 1 {
		t.Errorf("calls = %d; want 1 (no retry for non-ErrNotFound)", repo.findByStripeCalls)
	}
}

func TestHandleWebhook_CheckoutCompleted_InvalidClientReferenceID_Empty(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	// Empty client_reference_id will cause ParseInt to fail
	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"client_reference_id": "",
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

	if repo.upsertSubCalled {
		t.Error("UpsertSubscription should not be called when client_reference_id is empty")
	}
}

func TestHandleWebhook_CheckoutCompleted_InvalidClientReferenceID(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"client_reference_id": "not-a-number",
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

	if repo.upsertSubCalled {
		t.Error("UpsertSubscription should not be called when client_reference_id is invalid")
	}
}

func TestHandleWebhook_CheckoutCompleted_UpsertError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		upsertSubErr: errors.New("database error"),
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
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
		t.Fatal("UpsertSubscription should be called")
	}

	// updateTier should NOT be called when upsert fails (early return)
	if repo.updateTierCalled {
		t.Error("UpdateTier should not be called when upsert fails")
	}
}

func TestHandleWebhook_CheckoutCompleted_UpdateTierError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		updateTierErr: errors.New("tier update error"),
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
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

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier should be called even if it errors")
	}
}

func TestHandleWebhook_CheckoutCompleted_NilCustomerAndSubscription(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	// No "customer" or "subscription" fields -> nil pointers in the struct
	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
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
		t.Fatal("UpsertSubscription should be called")
	}

	if repo.upsertSubArg.StripeCustomerID != "" {
		t.Errorf("customer ID = %q; want empty", repo.upsertSubArg.StripeCustomerID)
	}

	if repo.upsertSubArg.StripeSubscriptionID != "" {
		t.Errorf("subscription ID = %q; want empty", repo.upsertSubArg.StripeSubscriptionID)
	}
}

func TestHandleWebhook_CheckoutCompleted_WithNotifier(t *testing.T) {
	t.Parallel()

	notifier := &syncNotifier{}
	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo, WithNotifier(notifier))

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"client_reference_id": "42",
	}

	payload := buildEventPayload(t, "checkout.session.completed", dataObj)
	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	notifier.expect()

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	notifier.wait()

	if !notifier.called {
		t.Fatal("notifier should be called")
	}
}

func TestHandleWebhook_InvoicePaid_NoCustomerID(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":     "in_test_123",
		"object": "invoice",
		// no customer field
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

	if repo.resetSecretsUsedCalled {
		t.Error("ResetSecretsUsed should not be called when customer ID is missing")
	}
}

func TestHandleWebhook_InvoicePaid_FindUserError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeErr: errors.New("database error"),
	}
	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = nil // no retries

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

	if repo.resetSecretsUsedCalled {
		t.Error("ResetSecretsUsed should not be called when find user fails")
	}
}

func TestHandleWebhook_InvoicePaid_ResetSecretsError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser:    &model.User{ID: 42, Tier: model.TierPro},
		resetSecretsUsedErr: errors.New("reset error"),
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
		t.Fatal("ResetSecretsUsed should be called")
	}
}

func TestHandleWebhook_InvoicePaid_WithLineItems(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro},
		subscription: &model.Subscription{
			StripeSubscriptionID: "sub_test_123",
		},
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "in_test_123",
		"object":   "invoice",
		"customer": "cus_test_123",
		"lines": map[string]any{
			"data": []map[string]any{
				{
					"period": map[string]any{
						"start": 1700000000,
						"end":   1702592000,
					},
				},
			},
		},
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
		t.Fatal("ResetSecretsUsed should be called")
	}

	if !repo.updateSubPeriodCalled {
		t.Fatal("UpdateSubscriptionPeriod should be called when line items exist")
	}

	if repo.updateSubPeriodSubID != "sub_test_123" {
		t.Errorf("period sub ID = %q; want %q", repo.updateSubPeriodSubID, "sub_test_123")
	}
}

func TestHandleWebhook_InvoicePaid_WithLineItems_PeriodUpdateError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro},
		subscription: &model.Subscription{
			StripeSubscriptionID: "sub_test_123",
		},
		updateSubPeriodErr: errors.New("period update error"),
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "in_test_123",
		"object":   "invoice",
		"customer": "cus_test_123",
		"lines": map[string]any{
			"data": []map[string]any{
				{
					"period": map[string]any{
						"start": 1700000000,
						"end":   1702592000,
					},
				},
			},
		},
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

	if !repo.updateSubPeriodCalled {
		t.Fatal("UpdateSubscriptionPeriod should be called")
	}
}

func TestHandleWebhook_InvoicePaid_WithLineItems_SubscriptionNotFound(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro},
		subscriptionErr:  model.ErrNotFound,
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "in_test_123",
		"object":   "invoice",
		"customer": "cus_test_123",
		"lines": map[string]any{
			"data": []map[string]any{
				{
					"period": map[string]any{
						"start": 1700000000,
						"end":   1702592000,
					},
				},
			},
		},
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

	// period should NOT be updated when subscription is not found
	if repo.updateSubPeriodCalled {
		t.Error("UpdateSubscriptionPeriod should not be called when subscription is not found")
	}
}

func TestHandleWebhook_SubscriptionDeleted_UpdateStatusError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser:   &model.User{ID: 42},
		updateSubStatusErr: errors.New("status update error"),
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
		t.Fatal("UpdateSubscriptionStatus should be called even if it errors")
	}

	// Should still proceed to update tier
	if !repo.updateTierCalled {
		t.Fatal("UpdateTier should be called even if status update errors")
	}
}

func TestHandleWebhook_SubscriptionDeleted_NoCustomerID(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":     "sub_test_123",
		"object": "subscription",
		"status": "canceled",
		// no customer field
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

	// updateTier should NOT be called because we can't find the user without customer ID
	if repo.updateTierCalled {
		t.Error("UpdateTier should not be called when customer ID is missing")
	}
}

func TestHandleWebhook_SubscriptionDeleted_FindUserError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeErr: errors.New("database error"),
	}
	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = nil

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

	if repo.updateTierCalled {
		t.Error("UpdateTier should not be called when find user fails")
	}
}

func TestHandleWebhook_SubscriptionDeleted_UpdateTierError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42},
		updateTierErr:    errors.New("tier update error"),
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

	if !repo.updateTierCalled {
		t.Fatal("UpdateTier should be called even if it errors")
	}
}

func TestHandleWebhook_SubscriptionDeleted_WithNotifier(t *testing.T) {
	t.Parallel()

	notifier := &syncNotifier{}
	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42},
	}
	svc := newWebhookTestService(t, repo, WithNotifier(notifier))

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

	notifier.expect()

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	notifier.wait()

	if !notifier.called {
		t.Fatal("notifier should be called for subscription deleted")
	}
}

func TestHandleWebhook_SubscriptionUpdated_UpdateStatusError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser:   &model.User{ID: 42},
		updateSubStatusErr: errors.New("status update error"),
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

	if !repo.updateSubStatusCalled {
		t.Fatal("UpdateSubscriptionStatus should be called even if it errors")
	}

	// Should still proceed
	if !repo.updateTierCalled {
		t.Fatal("UpdateTier should still be called after status update error")
	}
}

func TestHandleWebhook_SubscriptionUpdated_WithItems(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42},
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "sub_test_123",
		"object":   "subscription",
		"customer": "cus_test_123",
		"status":   "active",
		"items": map[string]any{
			"data": []map[string]any{
				{
					"id":                   "si_test_123",
					"current_period_start": 1700000000,
					"current_period_end":   1702592000,
				},
			},
		},
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

	if !repo.updateSubPeriodCalled {
		t.Fatal("UpdateSubscriptionPeriod should be called when items exist")
	}

	if repo.updateSubPeriodSubID != "sub_test_123" {
		t.Errorf("period sub ID = %q; want %q", repo.updateSubPeriodSubID, "sub_test_123")
	}
}

func TestHandleWebhook_SubscriptionUpdated_PeriodUpdateError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser:   &model.User{ID: 42},
		updateSubPeriodErr: errors.New("period update error"),
	}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":       "sub_test_123",
		"object":   "subscription",
		"customer": "cus_test_123",
		"status":   "active",
		"items": map[string]any{
			"data": []map[string]any{
				{
					"id":                   "si_test_123",
					"current_period_start": 1700000000,
					"current_period_end":   1702592000,
				},
			},
		},
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

	if !repo.updateSubPeriodCalled {
		t.Fatal("UpdateSubscriptionPeriod should be called even if it errors")
	}

	// Should still proceed to update tier
	if !repo.updateTierCalled {
		t.Fatal("UpdateTier should still be called after period update error")
	}
}

func TestHandleWebhook_SubscriptionUpdated_NoCustomerID(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	dataObj := map[string]any{
		"id":     "sub_test_123",
		"object": "subscription",
		"status": "active",
		// no customer field
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

	if repo.updateTierCalled {
		t.Error("UpdateTier should not be called when customer ID is missing")
	}
}

func TestHandleWebhook_SubscriptionUpdated_FindUserError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeErr: errors.New("database error"),
	}
	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = nil

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

	if repo.updateTierCalled {
		t.Error("UpdateTier should not be called when find user fails")
	}
}

func TestHandleWebhook_SubscriptionUpdated_UpdateTierError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42},
		updateTierErr:    errors.New("tier update error"),
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
		t.Fatal("UpdateTier should be called even if it errors")
	}
}

func TestHandleCheckoutCompleted_UnmarshalError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	// Call handleCheckoutCompleted directly with invalid raw data
	event := stripe.Event{
		Data: &stripe.EventData{
			Raw: json.RawMessage(`not valid json`),
		},
	}

	svc.handleCheckoutCompleted(context.Background(), event)

	if repo.upsertSubCalled {
		t.Error("UpsertSubscription should not be called when unmarshal fails")
	}
}

func TestHandleInvoicePaid_UnmarshalError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	event := stripe.Event{
		Data: &stripe.EventData{
			Raw: json.RawMessage(`not valid json`),
		},
	}

	svc.handleInvoicePaid(context.Background(), event)

	if repo.resetSecretsUsedCalled {
		t.Error("ResetSecretsUsed should not be called when unmarshal fails")
	}
}

func TestHandleSubscriptionDeleted_UnmarshalError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	event := stripe.Event{
		Data: &stripe.EventData{
			Raw: json.RawMessage(`not valid json`),
		},
	}

	svc.handleSubscriptionDeleted(context.Background(), event)

	if repo.updateSubStatusCalled {
		t.Error("UpdateSubscriptionStatus should not be called when unmarshal fails")
	}
}

func TestHandleSubscriptionUpdated_UnmarshalError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo)

	event := stripe.Event{
		Data: &stripe.EventData{
			Raw: json.RawMessage(`not valid json`),
		},
	}

	svc.handleSubscriptionUpdated(context.Background(), event)

	if repo.updateSubStatusCalled {
		t.Error("UpdateSubscriptionStatus should not be called when unmarshal fails")
	}
}

func TestHandleCheckoutCompleted_NotifierError(t *testing.T) {
	t.Parallel()

	notifier := &syncNotifier{err: errors.New("slack error")}
	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo, WithNotifier(notifier))

	dataObj := map[string]any{
		"id":                  "cs_test_123",
		"object":              "checkout.session",
		"customer":            "cus_test_123",
		"subscription":        "sub_test_123",
		"client_reference_id": "42",
	}

	payload := buildEventPayload(t, "checkout.session.completed", dataObj)
	sig := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", sig)

	notifier.expect()

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	notifier.wait()

	// Even with error, notifier was called
	if !notifier.called {
		t.Fatal("notifier should be called")
	}
}

func TestHandleSubscriptionDeleted_NotifierError(t *testing.T) {
	t.Parallel()

	notifier := &syncNotifier{err: errors.New("slack error")}
	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42},
	}
	svc := newWebhookTestService(t, repo, WithNotifier(notifier))

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

	notifier.expect()

	rec := httptest.NewRecorder()
	svc.HandleWebhook().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	notifier.wait()

	if !notifier.called {
		t.Fatal("notifier should be called even if it returns error")
	}
}

func TestMatchesProjectMetadata_UnmarshalError(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{}
	svc := newWebhookTestService(t, repo, WithProjectMetadata("project", "secretdrop"))

	event := stripe.Event{
		Data: &stripe.EventData{
			Raw: json.RawMessage(`not valid json`),
		},
	}

	if svc.matchesProjectMetadata(event) {
		t.Error("matchesProjectMetadata should return false when unmarshal fails")
	}
}

func TestMatchesProjectMetadata_NestedLocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "root metadata match",
			raw:  `{"metadata":{"project":"secretdrop"}}`,
			want: true,
		},
		{
			name: "subscription_details metadata match (invoice renewal)",
			raw:  `{"metadata":{},"subscription_details":{"metadata":{"project":"secretdrop"}}}`,
			want: true,
		},
		{
			name: "lines.data[0] metadata match (invoice line item)",
			raw:  `{"metadata":{},"lines":{"data":[{"metadata":{"project":"secretdrop"}}]}}`,
			want: true,
		},
		{
			name: "lines.data[1] metadata match (second line item)",
			raw:  `{"metadata":{},"lines":{"data":[{"metadata":{}},{"metadata":{"project":"secretdrop"}}]}}`,
			want: true,
		},
		{
			name: "no metadata anywhere",
			raw:  `{"metadata":{},"subscription_details":{"metadata":{}},"lines":{"data":[{"metadata":{}}]}}`,
			want: false,
		},
		{
			name: "wrong value everywhere",
			raw:  `{"metadata":{"project":"other"},"subscription_details":{"metadata":{"project":"other"}},"lines":{"data":[{"metadata":{"project":"other"}}]}}`,
			want: false,
		},
		{
			name: "subscription_details present but nil metadata",
			raw:  `{"metadata":{},"subscription_details":{}}`,
			want: false,
		},
		{
			name: "empty lines array",
			raw:  `{"metadata":{},"lines":{"data":[]}}`,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &webhookUserRepo{}
			svc := newWebhookTestService(t, repo, WithProjectMetadata("project", "secretdrop"))

			event := stripe.Event{
				Data: &stripe.EventData{Raw: json.RawMessage(tc.raw)},
			}

			if got := svc.matchesProjectMetadata(event); got != tc.want {
				t.Errorf("matchesProjectMetadata() = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestHandleWebhook_InvoicePaid_ProjectMatchViaSubscriptionDetails(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro},
		subscription: &model.Subscription{
			StripeSubscriptionID: "sub_test_123",
		},
	}
	svc := newWebhookTestService(t, repo, WithProjectMetadata("project", "secretdrop"))

	dataObj := map[string]any{
		"id":       "in_test_123",
		"object":   "invoice",
		"customer": "cus_test_123",
		"metadata": map[string]string{},
		"subscription_details": map[string]any{
			"metadata": map[string]string{"project": "secretdrop"},
		},
		"lines": map[string]any{
			"data": []map[string]any{
				{
					"period": map[string]any{
						"start": 1700000000,
						"end":   1702592000,
					},
				},
			},
		},
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
		t.Fatal("ResetSecretsUsed should be called when subscription_details metadata matches")
	}
}

func TestHandleWebhook_InvoicePaid_ProjectMismatchViaSubscriptionDetails(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeUser: &model.User{ID: 42, Tier: model.TierPro},
	}
	svc := newWebhookTestService(t, repo, WithProjectMetadata("project", "secretdrop"))

	dataObj := map[string]any{
		"id":       "in_test_123",
		"object":   "invoice",
		"customer": "cus_test_123",
		"metadata": map[string]string{},
		"subscription_details": map[string]any{
			"metadata": map[string]string{"project": "other-app"},
		},
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

	if repo.resetSecretsUsedCalled {
		t.Error("ResetSecretsUsed should NOT be called when no metadata matches this project")
	}
}

func TestFindUserWithRetry_NonNotFoundErrorOnRetry(t *testing.T) {
	t.Parallel()

	dbErr := errors.New("database connection failed")
	repo := &webhookUserRepo{
		findByStripeResults: []findByStripeResult{
			{nil, model.ErrNotFound},
			{nil, dbErr}, // non-NotFound error on retry
		},
	}

	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = []time.Duration{time.Millisecond, time.Millisecond}

	_, err := svc.findUserWithRetry(context.Background(), "cus_test_123")
	if !errors.Is(err, dbErr) {
		t.Errorf("findUserWithRetry() error = %v; want %v", err, dbErr)
	}

	if repo.findByStripeCalls != 2 {
		t.Errorf("calls = %d; want 2", repo.findByStripeCalls)
	}
}

func TestFindUserWithRetry_ContextCancelled(t *testing.T) {
	t.Parallel()

	repo := &webhookUserRepo{
		findByStripeResults: []findByStripeResult{
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
			{nil, model.ErrNotFound},
		},
	}

	svc := newWebhookTestService(t, repo)
	svc.retryBackoffs = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := svc.findUserWithRetry(ctx, "cus_test_123")
	if err == nil {
		t.Fatal("findUserWithRetry() error = nil; want context error")
	}
}
