package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/bilusteknoloji/secretdrop/internal/handler"
	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

type mockCanceller struct {
	called bool
	subID  string
	err    error
}

func (m *mockCanceller) CancelSubscription(_ context.Context, id string) error {
	m.called = true
	m.subID = id

	return m.err
}

func newTestUserRepo(t *testing.T) *usersqlite.Repository {
	t.Helper()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	t.Cleanup(func() { _ = repo.Close() })

	return repo
}

func TestDeleteAccount_Unauthenticated(t *testing.T) {
	t.Parallel()

	repo := newTestUserRepo(t)
	h := handler.NewDeleteAccountHandler(repo, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDeleteAccount_Success(t *testing.T) {
	t.Parallel()

	repo := newTestUserRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "g-del",
		Email:      "delete@example.com",
		Name:       "Delete Me",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	claims := &auth.Claims{UserID: u.ID, Email: u.Email, Tier: model.TierFree}
	h := handler.NewDeleteAccountHandler(repo, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}

	// Verify user is deleted.
	_, findErr := repo.FindByID(ctx, u.ID)
	if !errors.Is(findErr, model.ErrNotFound) {
		t.Errorf("FindByID() after delete: error = %v; want model.ErrNotFound", findErr)
	}
}

func TestDeleteAccount_CancelsStripeSubscription(t *testing.T) {
	t.Parallel()

	repo := newTestUserRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "g-delsub",
		Email:      "delsub@example.com",
		Name:       "Delete Sub",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	sub := &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_del",
		StripeSubscriptionID: "sub_del",
		Status:               model.SubscriptionActive,
	}
	if err := repo.UpsertSubscription(ctx, sub); err != nil {
		t.Fatalf("UpsertSubscription() error = %v", err)
	}

	canceller := &mockCanceller{}
	claims := &auth.Claims{UserID: u.ID, Email: u.Email, Tier: model.TierPro}
	h := handler.NewDeleteAccountHandler(repo, canceller, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}

	if !canceller.called {
		t.Error("CancelSubscription was not called")
	}

	if canceller.subID != "sub_del" {
		t.Errorf("CancelSubscription subID = %q; want %q", canceller.subID, "sub_del")
	}

	// Verify user is deleted.
	_, findErr := repo.FindByID(ctx, u.ID)
	if !errors.Is(findErr, model.ErrNotFound) {
		t.Errorf("FindByID() after delete: error = %v; want model.ErrNotFound", findErr)
	}

	// Verify subscription is deleted.
	_, subErr := repo.FindSubscriptionByUserID(ctx, u.ID)
	if !errors.Is(subErr, model.ErrNotFound) {
		t.Errorf("FindSubscriptionByUserID() after delete: error = %v; want model.ErrNotFound", subErr)
	}
}

func TestDeleteAccount_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestUserRepo(t)
	claims := &auth.Claims{UserID: 99999, Email: "ghost@example.com", Tier: model.TierFree}
	h := handler.NewDeleteAccountHandler(repo, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}
