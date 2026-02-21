package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/handler"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func newAdminTestRepo(t *testing.T) *usersqlite.Repository {
	t.Helper()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	t.Cleanup(func() { _ = repo.Close() })

	return repo
}

// stubCanceller is a fake SubscriptionCanceller for testing.
type stubCanceller struct {
	called bool
	err    error
}

func (s *stubCanceller) CancelSubscription(_ context.Context, _ string) error {
	s.called = true

	return s.err
}

func TestAdminListUsers_Empty(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("total = %d; want 0", resp.Total)
	}
}

func TestAdminListUsers_WithData(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "github", ProviderID: "gh1",
		Email: "bob@example.com", Name: "Bob",
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?per_page=1&page=1", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("total = %d; want 2", resp.Total)
	}

	if len(resp.Users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(resp.Users))
	}
}

func TestAdminListUsers_Search(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "github", ProviderID: "gh1",
		Email: "bob@example.com", Name: "Bob",
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?q=alice", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 1 {
		t.Errorf("total = %d; want 1", resp.Total)
	}
}

func TestAdminListUsers_FilterByTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "free@example.com", Name: "Free User",
	})
	pro, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "pro@example.com", Name: "Pro User",
	})
	_ = repo.UpdateTier(ctx, pro.ID, model.TierPro)

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?tier=pro", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 1 {
		t.Errorf("total = %d; want 1", resp.Total)
	}
}

func TestAdminListUsers_Sort(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "bravo@example.com", Name: "Bravo",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "alpha@example.com", Name: "Alpha",
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?sort=email&order=asc", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Users) < 2 {
		t.Fatalf("len(users) = %d; want >= 2", len(resp.Users))
	}

	if resp.Users[0].Email != "alpha@example.com" {
		t.Errorf("first user email = %q; want %q", resp.Users[0].Email, "alpha@example.com")
	}
}

func TestAdminListUsers_PaginationDefaults(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Page != 1 {
		t.Errorf("page = %d; want 1", resp.Page)
	}

	if resp.PerPage != 20 {
		t.Errorf("per_page = %d; want 20", resp.PerPage)
	}
}

func TestAdminListUsers_PerPageCap(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?per_page=999", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.PerPage != 100 {
		t.Errorf("per_page = %d; want 100 (capped)", resp.PerPage)
	}
}

func TestAdminListUsers_InvalidPaginationParams(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=abc&per_page=xyz", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Page != 1 {
		t.Errorf("page = %d; want 1 (default)", resp.Page)
	}

	if resp.PerPage != 20 {
		t.Errorf("per_page = %d; want 20 (default)", resp.PerPage)
	}
}

func TestAdminUpdateTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	updated, _ := repo.FindByID(ctx, u.ID)
	if updated.Tier != model.TierPro {
		t.Errorf("tier = %q; want %q", updated.Tier, model.TierPro)
	}
}

func TestAdminUpdateTier_InvalidTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)

	body := strings.NewReader(`{"tier":"enterprise"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpdateTier_InvalidID(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/abc", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpdateTier_InvalidJSON(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpdateTier_UserNotFound(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/9999", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminListSubscriptions(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "sub@example.com", Name: "Sub User",
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminSubscriptionsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("total = %d; want 1", resp.Total)
	}

	if resp.Subscriptions[0].UserEmail != "sub@example.com" {
		t.Errorf("user_email = %q; want %q", resp.Subscriptions[0].UserEmail, "sub@example.com")
	}
}

func TestAdminListSubscriptions_FilterByStatus(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "sub@example.com", Name: "Sub User",
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_active",
		Status:               model.SubscriptionActive,
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_canceled",
		Status:               model.SubscriptionCanceled,
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions?status=active", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	var resp model.AdminSubscriptionsListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 1 {
		t.Errorf("total = %d; want 1", resp.Total)
	}
}

func TestAdminListSubscriptions_WithSearch(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "searchme@example.com", Name: "Search User",
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions?q=searchme", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestAdminListSubscriptions_WithSort(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "sub@example.com", Name: "Sub User",
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions?sort=created_at&order=asc", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestAdminCancelSubscription(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "cancel@example.com", Name: "Cancel User",
	})
	_ = repo.UpdateTier(ctx, u.ID, model.TierPro)
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_cancel",
		StripeSubscriptionID: "sub_cancel",
		Status:               model.SubscriptionActive,
	})

	canceller := &stubCanceller{}
	h := handler.NewAdminHandler(repo, canceller)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/subscriptions/%d", u.ID), nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}

	if !canceller.called {
		t.Error("expected canceller to be called")
	}

	// Verify subscription status updated
	sub, _ := repo.FindSubscriptionByUserID(ctx, u.ID)
	if sub.Status != model.SubscriptionCanceled {
		t.Errorf("subscription status = %q; want %q", sub.Status, model.SubscriptionCanceled)
	}

	// Verify tier downgraded
	updated, _ := repo.FindByID(ctx, u.ID)
	if updated.Tier != model.TierFree {
		t.Errorf("tier = %q; want %q", updated.Tier, model.TierFree)
	}
}

func TestAdminCancelSubscription_NilCanceller(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "cancel@example.com", Name: "Cancel User",
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/subscriptions/%d", u.ID), nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAdminCancelSubscription_InvalidID(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/subscriptions/abc", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminCancelSubscription_NotFound(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/subscriptions/9999", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminCancelSubscription_CancellerError(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "cancel@example.com", Name: "Cancel User",
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	canceller := &stubCanceller{err: fmt.Errorf("stripe error")}
	h := handler.NewAdminHandler(repo, canceller)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/subscriptions/%d", u.ID), nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminRegister(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	h.Register(mux)

	// Verify all routes are registered by making requests
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodGet, "/api/v1/admin/subscriptions"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s %s: status = %d; want %d", tt.method, tt.path, rec.Code, http.StatusOK)
		}
	}
}

func TestAdminListLimits(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/limits", nil)
	rec := httptest.NewRecorder()

	h.ListLimits(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp []model.AdminLimitsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp) < 2 {
		t.Errorf("len(limits) = %d; want >= 2 (free + pro seeded)", len(resp))
	}
}

func TestAdminUpsertLimits(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)

	body := strings.NewReader(`{"secrets_limit":1000,"recipients_limit":20}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/limits/vip", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminLimitsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Tier != "vip" {
		t.Errorf("tier = %q; want %q", resp.Tier, "vip")
	}

	if resp.SecretsLimit != 1000 {
		t.Errorf("secrets_limit = %d; want 1000", resp.SecretsLimit)
	}

	if resp.RecipientsLimit != 20 {
		t.Errorf("recipients_limit = %d; want 20", resp.RecipientsLimit)
	}
}

func TestAdminUpsertLimits_InvalidBody(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/limits/vip", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpsertLimits_InvalidLimits(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)

	body := strings.NewReader(`{"secrets_limit":0,"recipients_limit":5}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/limits/vip", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminDeleteLimits(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)
	mux.HandleFunc("DELETE /api/v1/admin/limits/{tier}", h.DeleteLimits)

	// First, upsert a "vip" tier
	upsertBody := strings.NewReader(`{"secrets_limit":500,"recipients_limit":10}`)
	upsertReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/limits/vip", upsertBody)
	upsertRec := httptest.NewRecorder()

	mux.ServeHTTP(upsertRec, upsertReq)

	if upsertRec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d; want %d", upsertRec.Code, http.StatusOK)
	}

	// Now delete it
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/limits/vip", nil)
	deleteRec := httptest.NewRecorder()

	mux.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", deleteRec.Code, http.StatusNoContent)
	}
}

func TestAdminDeleteLimits_FreeTierProtected(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/limits/{tier}", h.DeleteLimits)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/limits/free", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminDeleteLimits_NotFound(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/limits/{tier}", h.DeleteLimits)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/limits/nonexistent", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminUpdateUser_SecretsLimitOverride(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "override@example.com", Name: "Override User",
	})

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"secrets_limit_override":500}`)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	updated, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}

	if updated.SecretsLimitOverride == nil {
		t.Fatal("SecretsLimitOverride = nil; want 500")
	}

	if *updated.SecretsLimitOverride != 500 {
		t.Errorf("SecretsLimitOverride = %d; want 500", *updated.SecretsLimitOverride)
	}
}

func TestAdminUpdateUser_ClearSecretsLimit(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "clear@example.com", Name: "Clear User",
	})

	// First set an override
	overrideVal := 500
	_ = repo.UpdateSecretsLimitOverride(ctx, u.ID, &overrideVal)

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"clear_secrets_limit":true}`)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	updated, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}

	if updated.SecretsLimitOverride != nil {
		t.Errorf("SecretsLimitOverride = %d; want nil", *updated.SecretsLimitOverride)
	}
}

func TestAdminUpdateUser_NoFields(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "nofields@example.com", Name: "No Fields",
	})

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminListUsers_WithEffectiveLimit(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "effective@example.com", Name: "Effective User",
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Users) != 1 {
		t.Fatalf("len(users) = %d; want 1", len(resp.Users))
	}

	// The user has tier "free" and no override, so effective limit should match
	// the seeded free tier default (5) from the limits table.
	if resp.Users[0].SecretsLimit != model.FreeTierLimit {
		t.Errorf("secrets_limit = %d; want %d", resp.Users[0].SecretsLimit, model.FreeTierLimit)
	}
}
