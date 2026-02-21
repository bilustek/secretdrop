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
