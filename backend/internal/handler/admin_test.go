package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bilustek/secretdrop/internal/handler"
	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/user"
	usersqlite "github.com/bilustek/secretdrop/internal/user/sqlite"
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

func TestAdminListUsers_FilterByProvider(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "google@example.com", Name: "Google User",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "github", ProviderID: "gh1",
		Email: "github@example.com", Name: "GitHub User",
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?provider=github", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 1 {
		t.Errorf("total = %d; want 1", resp.Total)
	}

	if len(resp.Users) == 1 && resp.Users[0].Provider != "github" {
		t.Errorf("provider = %q; want %q", resp.Users[0].Provider, "github")
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

// --- mockAdminRepo for error-path testing ---

type mockAdminRepo struct {
	listUsersFunc              func(ctx context.Context, opts ...user.ListOption) ([]*model.User, error)
	countUsersFunc             func(ctx context.Context, opts ...user.ListOption) (int64, error)
	listSubscriptionsFunc      func(ctx context.Context, opts ...user.ListOption) ([]*user.SubscriptionWithUser, error)
	countSubscriptionsFunc     func(ctx context.Context, opts ...user.ListOption) (int64, error)
	listLimitsFunc             func(ctx context.Context) ([]*user.TierLimits, error)
	upsertLimitsFunc           func(ctx context.Context, tl *user.TierLimits) error
	deleteLimitsFunc           func(ctx context.Context, tier string) error
	updateTierFunc             func(ctx context.Context, id int64, tier string) error
	tierExistsFunc             func(ctx context.Context, tier string) (bool, error)
	updateSecretsLimitFunc     func(ctx context.Context, id int64, limit *int) error
	updateRecipientsLimitFunc  func(ctx context.Context, id int64, limit *int) error
	findSubscriptionByUserFunc func(ctx context.Context, id int64) (*model.Subscription, error)
	updateSubStatusFunc        func(ctx context.Context, stripeSubID, status string) error
}

func (m *mockAdminRepo) Upsert(_ context.Context, _ *model.User) (*model.User, error) {
	return nil, nil
}

func (m *mockAdminRepo) FindByID(_ context.Context, _ int64) (*model.User, error) {
	return nil, nil
}

func (m *mockAdminRepo) FindByProvider(_ context.Context, _, _ string) (*model.User, error) {
	return nil, nil
}

func (m *mockAdminRepo) IncrementSecretsUsed(_ context.Context, _ int64) error { return nil }
func (m *mockAdminRepo) ResetSecretsUsed(_ context.Context, _ int64) error     { return nil }

func (m *mockAdminRepo) UpdateTier(ctx context.Context, id int64, tier string) error {
	if m.updateTierFunc != nil {
		return m.updateTierFunc(ctx, id, tier)
	}

	return nil
}

func (m *mockAdminRepo) UpdateTimezone(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockAdminRepo) DeleteUser(_ context.Context, _ int64) error               { return nil }

func (m *mockAdminRepo) UpsertSubscription(_ context.Context, _ *model.Subscription) error {
	return nil
}

func (m *mockAdminRepo) FindSubscriptionByUserID(ctx context.Context, userID int64) (*model.Subscription, error) {
	if m.findSubscriptionByUserFunc != nil {
		return m.findSubscriptionByUserFunc(ctx, userID)
	}

	return nil, model.ErrNotFound
}

func (m *mockAdminRepo) FindUserByStripeCustomerID(_ context.Context, _ string) (*model.User, error) {
	return nil, nil
}

func (m *mockAdminRepo) UpdateSubscriptionStatus(ctx context.Context, stripeSubID, status string) error {
	if m.updateSubStatusFunc != nil {
		return m.updateSubStatusFunc(ctx, stripeSubID, status)
	}

	return nil
}

func (m *mockAdminRepo) UpdateSubscriptionPeriod(_ context.Context, _ string, _, _ time.Time) error {
	return nil
}

func (m *mockAdminRepo) ListUsers(ctx context.Context, opts ...user.ListOption) ([]*model.User, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx, opts...)
	}

	return nil, nil
}

func (m *mockAdminRepo) CountUsers(ctx context.Context, opts ...user.ListOption) (int64, error) {
	if m.countUsersFunc != nil {
		return m.countUsersFunc(ctx, opts...)
	}

	return 0, nil
}

func (m *mockAdminRepo) ListSubscriptions(
	ctx context.Context,
	opts ...user.ListOption,
) ([]*user.SubscriptionWithUser, error) {
	if m.listSubscriptionsFunc != nil {
		return m.listSubscriptionsFunc(ctx, opts...)
	}

	return nil, nil
}

func (m *mockAdminRepo) CountSubscriptions(ctx context.Context, opts ...user.ListOption) (int64, error) {
	if m.countSubscriptionsFunc != nil {
		return m.countSubscriptionsFunc(ctx, opts...)
	}

	return 0, nil
}

func (m *mockAdminRepo) ListLimits(ctx context.Context) ([]*user.TierLimits, error) {
	if m.listLimitsFunc != nil {
		return m.listLimitsFunc(ctx)
	}

	return nil, nil
}

func (m *mockAdminRepo) UpsertLimits(ctx context.Context, tl *user.TierLimits) error {
	if m.upsertLimitsFunc != nil {
		return m.upsertLimitsFunc(ctx, tl)
	}

	return nil
}

func (m *mockAdminRepo) DeleteLimits(ctx context.Context, tier string) error {
	if m.deleteLimitsFunc != nil {
		return m.deleteLimitsFunc(ctx, tier)
	}

	return nil
}

func (m *mockAdminRepo) UpdateSecretsLimitOverride(ctx context.Context, id int64, limit *int) error {
	if m.updateSecretsLimitFunc != nil {
		return m.updateSecretsLimitFunc(ctx, id, limit)
	}

	return nil
}

func (m *mockAdminRepo) UpdateRecipientsLimitOverride(ctx context.Context, id int64, limit *int) error {
	if m.updateRecipientsLimitFunc != nil {
		return m.updateRecipientsLimitFunc(ctx, id, limit)
	}

	return nil
}

func (m *mockAdminRepo) TierExists(ctx context.Context, tier string) (bool, error) {
	if m.tierExistsFunc != nil {
		return m.tierExistsFunc(ctx, tier)
	}

	return true, nil
}

func (m *mockAdminRepo) GetLimits(_ context.Context, _ string) (*user.TierLimits, error) {
	return nil, model.ErrNotFound
}

func (m *mockAdminRepo) FindTierByPriceID(_ context.Context, _ string) (string, error) {
	return "", nil
}

// --- ListUsers error paths ---

func TestAdminListUsers_ListError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminListUsers_CountError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 0, fmt.Errorf("count error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminListUsers_BuildLimitsMapError(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "a@b.com", Name: "A", Provider: "google", Tier: model.TierFree, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return nil, fmt.Errorf("limits error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	// Should still succeed; buildLimitsMap error is not fatal, falls back to hardcoded.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// With nil limitsMap, computeEffectiveLimit falls back to FreeTierLimit.
	if resp.Users[0].SecretsLimit != model.FreeTierLimit {
		t.Errorf("secrets_limit = %d; want %d", resp.Users[0].SecretsLimit, model.FreeTierLimit)
	}
}

func TestAdminListUsers_EffectiveLimit_ProTierFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "pro@b.com", Name: "Pro", Provider: "google", Tier: model.TierPro, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			// Return empty map so no tier match, triggers pro fallback.
			return []*user.TierLimits{}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Users[0].SecretsLimit != model.ProTierLimit {
		t.Errorf("secrets_limit = %d; want %d", resp.Users[0].SecretsLimit, model.ProTierLimit)
	}
}

func TestAdminListUsers_EffectiveLimit_Override(t *testing.T) {
	t.Parallel()

	now := time.Now()
	overrideVal := 999
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{
					ID: 1, Email: "override@b.com", Name: "Override",
					Provider: "google", Tier: model.TierFree,
					SecretsLimitOverride: &overrideVal, CreatedAt: now,
				},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{
				{Tier: model.TierFree, SecretsLimit: 5, RecipientsLimit: 1},
			}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Users[0].SecretsLimit != 999 {
		t.Errorf("secrets_limit = %d; want 999", resp.Users[0].SecretsLimit)
	}
}

func TestAdminListUsers_EffectiveLimit_TierFromLimitsMap(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "pro@b.com", Name: "Pro", Provider: "google", Tier: model.TierPro, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{
				{Tier: model.TierPro, SecretsLimit: 200, RecipientsLimit: 10},
			}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	// Should use the value from limitsMap, not the hardcoded ProTierLimit.
	if resp.Users[0].SecretsLimit != 200 {
		t.Errorf("secrets_limit = %d; want 200", resp.Users[0].SecretsLimit)
	}
}

// --- UpdateUser error paths ---

func TestAdminUpdateUser_TierExistsError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		tierExistsFunc: func(_ context.Context, _ string) (bool, error) {
			return false, fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminUpdateUser_UpdateTierInternalError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		tierExistsFunc: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
		updateTierFunc: func(_ context.Context, _ int64, _ string) error {
			return fmt.Errorf("db write error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminUpdateUser_NegativeSecretsLimitOverride(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "neg@example.com", Name: "Neg User",
	})

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"secrets_limit_override":-5}`)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpdateUser_ZeroSecretsLimitOverride(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "zero@example.com", Name: "Zero User",
	})

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"secrets_limit_override":0}`)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpdateUser_OverrideNotFound(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		updateSecretsLimitFunc: func(_ context.Context, _ int64, _ *int) error {
			return model.ErrNotFound
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"secrets_limit_override":100}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/9999", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminUpdateUser_OverrideInternalError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		updateSecretsLimitFunc: func(_ context.Context, _ int64, _ *int) error {
			return fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"secrets_limit_override":100}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminUpdateUser_ClearSecretsLimitNotFound(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		updateSecretsLimitFunc: func(_ context.Context, _ int64, _ *int) error {
			return model.ErrNotFound
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"clear_secrets_limit":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/9999", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminUpdateUser_ClearSecretsLimitInternalError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		updateSecretsLimitFunc: func(_ context.Context, _ int64, _ *int) error {
			return fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)

	body := strings.NewReader(`{"clear_secrets_limit":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- ListSubscriptions error paths ---

func TestAdminListSubscriptions_Empty(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminSubscriptionsListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 0 {
		t.Errorf("total = %d; want 0", resp.Total)
	}
}

func TestAdminListSubscriptions_Pagination(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions?page=2&per_page=5", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminSubscriptionsListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Page != 2 {
		t.Errorf("page = %d; want 2", resp.Page)
	}

	if resp.PerPage != 5 {
		t.Errorf("per_page = %d; want 5", resp.PerPage)
	}
}

func TestAdminListSubscriptions_ListError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listSubscriptionsFunc: func(_ context.Context, _ ...user.ListOption) ([]*user.SubscriptionWithUser, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminListSubscriptions_CountError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listSubscriptionsFunc: func(_ context.Context, _ ...user.ListOption) ([]*user.SubscriptionWithUser, error) {
			return []*user.SubscriptionWithUser{}, nil
		},
		countSubscriptionsFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 0, fmt.Errorf("count error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- ListLimits error path ---

func TestAdminListLimits_Error(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/limits", nil)
	rec := httptest.NewRecorder()

	h.ListLimits(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- UpsertLimits error paths ---

func TestAdminUpsertLimits_InvalidRecipientsLimit(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)

	body := strings.NewReader(`{"secrets_limit":10,"recipients_limit":0}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/limits/vip", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpsertLimits_NegativeRecipientsLimit(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)

	body := strings.NewReader(`{"secrets_limit":10,"recipients_limit":-1}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/limits/vip", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminUpsertLimits_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		upsertLimitsFunc: func(_ context.Context, _ *user.TierLimits) error {
			return fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)

	body := strings.NewReader(`{"secrets_limit":10,"recipients_limit":5}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/limits/vip", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- DeleteLimits error path ---

func TestAdminDeleteLimits_InternalError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		deleteLimitsFunc: func(_ context.Context, _ string) error {
			return fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/limits/{tier}", h.DeleteLimits)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/limits/vip", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- CancelSubscription error paths ---

func TestAdminCancelSubscription_FindInternalError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		findSubscriptionByUserFunc: func(_ context.Context, _ int64) (*model.Subscription, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/subscriptions/1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAdminCancelSubscription_UpdateTierError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		findSubscriptionByUserFunc: func(_ context.Context, _ int64) (*model.Subscription, error) {
			return &model.Subscription{
				StripeSubscriptionID: "sub_1",
				UserID:               1,
			}, nil
		},
		updateTierFunc: func(_ context.Context, _ int64, _ string) error {
			return fmt.Errorf("tier update error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/subscriptions/1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	// UpdateTier error is logged but not fatal; should still return 204.
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAdminCancelSubscription_UpdateStatusError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		findSubscriptionByUserFunc: func(_ context.Context, _ int64) (*model.Subscription, error) {
			return &model.Subscription{
				StripeSubscriptionID: "sub_1",
				UserID:               1,
			}, nil
		},
		updateSubStatusFunc: func(_ context.Context, _, _ string) error {
			return fmt.Errorf("db error")
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/subscriptions/1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- NewPlansHandler tests ---

func TestNewPlansHandler_ListPlans(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{
				{Tier: model.TierPro, SecretsLimit: 100, RecipientsLimit: 5, PriceCents: 999, Currency: "usd"},
				{Tier: model.TierFree, SecretsLimit: 5, RecipientsLimit: 1, PriceCents: 0, Currency: "usd"},
				{Tier: model.TierTeam, SecretsLimit: 1000, RecipientsLimit: 15, PriceCents: 2999, Currency: "usd"},
			}, nil
		},
	}

	h := handler.NewPlansHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var plans []model.PlanResponse
	if err := json.NewDecoder(rec.Body).Decode(&plans); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(plans) != 3 {
		t.Fatalf("len(plans) = %d; want 3", len(plans))
	}

	// Plans should be sorted by price_cents ascending.
	if plans[0].Tier != model.TierFree {
		t.Errorf("plans[0].tier = %q; want %q", plans[0].Tier, model.TierFree)
	}

	if plans[1].Tier != model.TierPro {
		t.Errorf("plans[1].tier = %q; want %q", plans[1].Tier, model.TierPro)
	}

	if plans[2].Tier != model.TierTeam {
		t.Errorf("plans[2].tier = %q; want %q", plans[2].Tier, model.TierTeam)
	}

	// Verify max_text_length is populated correctly for each tier.
	if plans[0].MaxTextLength != model.FreeMaxTextLength {
		t.Errorf("plans[0].max_text_length = %d; want %d", plans[0].MaxTextLength, model.FreeMaxTextLength)
	}

	if plans[1].MaxTextLength != model.ProMaxTextLength {
		t.Errorf("plans[1].max_text_length = %d; want %d", plans[1].MaxTextLength, model.ProMaxTextLength)
	}

	if plans[2].MaxTextLength != model.TeamMaxTextLength {
		t.Errorf("plans[2].max_text_length = %d; want %d", plans[2].MaxTextLength, model.TeamMaxTextLength)
	}

	// Verify pricing fields.
	if plans[1].PriceCents != 999 {
		t.Errorf("plans[1].price_cents = %d; want 999", plans[1].PriceCents)
	}

	if plans[2].PriceCents != 2999 {
		t.Errorf("plans[2].price_cents = %d; want 2999", plans[2].PriceCents)
	}
}

func TestNewPlansHandler_EmptyList(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{}, nil
		},
	}

	h := handler.NewPlansHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var plans []model.PlanResponse
	if err := json.NewDecoder(rec.Body).Decode(&plans); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(plans) != 0 {
		t.Errorf("len(plans) = %d; want 0", len(plans))
	}
}

func TestNewPlansHandler_DBError(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	h := handler.NewPlansHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestNewPlansHandler_UnknownTierMaxTextLength(t *testing.T) {
	t.Parallel()

	repo := &mockAdminRepo{
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{
				{Tier: "enterprise", SecretsLimit: 5000, RecipientsLimit: 50, PriceCents: 9999, Currency: "usd"},
			}, nil
		},
	}

	h := handler.NewPlansHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var plans []model.PlanResponse
	if err := json.NewDecoder(rec.Body).Decode(&plans); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("len(plans) = %d; want 1", len(plans))
	}

	// Unknown tier should fall back to FreeMaxTextLength.
	if plans[0].MaxTextLength != model.FreeMaxTextLength {
		t.Errorf("max_text_length = %d; want %d (free fallback)", plans[0].MaxTextLength, model.FreeMaxTextLength)
	}
}

// --- computeEffectiveLimit / computeEffectiveRecipientsLimit with team tier ---

func TestAdminListUsers_EffectiveLimit_TeamTierFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "team@b.com", Name: "Team", Provider: "google", Tier: model.TierTeam, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			// Return empty map so no tier match, triggers team fallback.
			return []*user.TierLimits{}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Users[0].SecretsLimit != model.TeamTierLimit {
		t.Errorf("secrets_limit = %d; want %d", resp.Users[0].SecretsLimit, model.TeamTierLimit)
	}
}

func TestAdminListUsers_EffectiveRecipientsLimit_TeamTierFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "team@b.com", Name: "Team", Provider: "google", Tier: model.TierTeam, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Users[0].RecipientsLimit != model.TeamMaxRecipients {
		t.Errorf("recipients_limit = %d; want %d", resp.Users[0].RecipientsLimit, model.TeamMaxRecipients)
	}
}

func TestAdminListUsers_EffectiveRecipientsLimit_ProTierFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "pro@b.com", Name: "Pro", Provider: "google", Tier: model.TierPro, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Users[0].RecipientsLimit != model.ProMaxRecipients {
		t.Errorf("recipients_limit = %d; want %d", resp.Users[0].RecipientsLimit, model.ProMaxRecipients)
	}
}

func TestAdminListUsers_EffectiveRecipientsLimit_FreeTierFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "free@b.com", Name: "Free", Provider: "google", Tier: model.TierFree, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Users[0].RecipientsLimit != model.FreeMaxRecipients {
		t.Errorf("recipients_limit = %d; want %d", resp.Users[0].RecipientsLimit, model.FreeMaxRecipients)
	}
}

func TestAdminListUsers_EffectiveRecipientsLimit_Override(t *testing.T) {
	t.Parallel()

	now := time.Now()
	overrideVal := 42
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{
					ID: 1, Email: "override@b.com", Name: "Override",
					Provider: "google", Tier: model.TierFree,
					RecipientsLimitOverride: &overrideVal, CreatedAt: now,
				},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{
				{Tier: model.TierFree, SecretsLimit: 5, RecipientsLimit: 1},
			}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Users[0].RecipientsLimit != 42 {
		t.Errorf("recipients_limit = %d; want 42", resp.Users[0].RecipientsLimit)
	}
}

func TestAdminListUsers_EffectiveRecipientsLimit_TierFromLimitsMap(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "pro@b.com", Name: "Pro", Provider: "google", Tier: model.TierPro, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{
				{Tier: model.TierPro, SecretsLimit: 200, RecipientsLimit: 25},
			}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	// Should use the value from limitsMap, not the hardcoded ProMaxRecipients.
	if resp.Users[0].RecipientsLimit != 25 {
		t.Errorf("recipients_limit = %d; want 25", resp.Users[0].RecipientsLimit)
	}
}

func TestAdminListUsers_EffectiveLimit_TeamTierFromLimitsMap(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockAdminRepo{
		listUsersFunc: func(_ context.Context, _ ...user.ListOption) ([]*model.User, error) {
			return []*model.User{
				{ID: 1, Email: "team@b.com", Name: "Team", Provider: "google", Tier: model.TierTeam, CreatedAt: now},
			}, nil
		},
		countUsersFunc: func(_ context.Context, _ ...user.ListOption) (int64, error) {
			return 1, nil
		},
		listLimitsFunc: func(_ context.Context) ([]*user.TierLimits, error) {
			return []*user.TierLimits{
				{Tier: model.TierTeam, SecretsLimit: 2000, RecipientsLimit: 30},
			}, nil
		},
	}
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	// Should use the value from limitsMap for team tier.
	if resp.Users[0].SecretsLimit != 2000 {
		t.Errorf("secrets_limit = %d; want 2000", resp.Users[0].SecretsLimit)
	}

	if resp.Users[0].RecipientsLimit != 30 {
		t.Errorf("recipients_limit = %d; want 30", resp.Users[0].RecipientsLimit)
	}
}
