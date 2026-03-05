package sqlite_test

import (
	"context"
	"fmt"
	"testing"

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

func seedUsers(t *testing.T, repo *usersqlite.Repository, count int) {
	t.Helper()

	ctx := context.Background()

	for i := range count {
		tier := model.TierFree

		u, err := repo.Upsert(ctx, &model.User{
			Provider:   "google",
			ProviderID: fmt.Sprintf("gid-%d", i),
			Email:      fmt.Sprintf("user%d@example.com", i),
			Name:       fmt.Sprintf("User %d", i),
		})
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}

		if i%3 == 0 {
			tier = model.TierPro
			if err := repo.UpdateTier(ctx, u.ID, tier); err != nil {
				t.Fatalf("UpdateTier() error = %v", err)
			}
		}
	}
}

func TestListUsers_Pagination(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	seedUsers(t, repo, 5)

	ctx := context.Background()

	users, err := repo.ListUsers(ctx, user.WithPage(1, 2))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 2 {
		t.Errorf("len(users) = %d; want 2", len(users))
	}

	count, err := repo.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}

	if count != 5 {
		t.Errorf("count = %d; want 5", count)
	}
}

func TestListUsers_SearchByEmail(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "bob@example.com", Name: "Bob",
	})

	users, err := repo.ListUsers(ctx, user.WithSearch("alice"))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(users))
	}
}

func TestListUsers_FilterByTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "free@example.com", Name: "Free",
	})
	pro, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "pro@example.com", Name: "Pro",
	})
	_ = repo.UpdateTier(ctx, pro.ID, model.TierPro)

	users, err := repo.ListUsers(ctx, user.WithTier(model.TierPro))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(users))
	}
}

func TestListUsers_FilterByProvider(t *testing.T) {
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

	users, err := repo.ListUsers(ctx, user.WithProvider("github"))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(users))
	}

	if len(users) > 0 && users[0].Provider != "github" {
		t.Errorf("provider = %q; want %q", users[0].Provider, "github")
	}
}

func TestCountUsers_FilterByProvider(t *testing.T) {
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

	count, err := repo.CountUsers(ctx, user.WithProvider("github"))
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestListUsers_Sort(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "b@example.com", Name: "Bravo",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "a@example.com", Name: "Alpha",
	})

	users, err := repo.ListUsers(ctx, user.WithSort("email", "asc"))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) < 2 {
		t.Fatalf("len(users) = %d; want >= 2", len(users))
	}

	if users[0].Email != "a@example.com" {
		t.Errorf("first user email = %q; want %q", users[0].Email, "a@example.com")
	}
}

func TestCountUsers_WithFilter(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "free@example.com", Name: "Free",
	})
	u2, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "pro@example.com", Name: "Pro",
	})
	_ = repo.UpdateTier(ctx, u2.ID, model.TierPro)

	count, err := repo.CountUsers(ctx, user.WithTier(model.TierPro))
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestListSubscriptions_Basic(t *testing.T) {
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

	subs, err := repo.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) != 1 {
		t.Fatalf("len(subs) = %d; want 1", len(subs))
	}

	if subs[0].UserEmail != "sub@example.com" {
		t.Errorf("UserEmail = %q; want %q", subs[0].UserEmail, "sub@example.com")
	}

	if subs[0].UserName != "Sub User" {
		t.Errorf("UserName = %q; want %q", subs[0].UserName, "Sub User")
	}
}

func TestListSubscriptions_FilterByStatus(t *testing.T) {
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
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_2",
		Status:               model.SubscriptionCanceled,
	})

	subs, err := repo.ListSubscriptions(ctx, user.WithStatus(model.SubscriptionActive))
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) != 1 {
		t.Errorf("len(subs) = %d; want 1", len(subs))
	}
}

func TestCountSubscriptions(t *testing.T) {
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

	count, err := repo.CountSubscriptions(ctx)
	if err != nil {
		t.Fatalf("CountSubscriptions() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestCountSubscriptions_WithStatusFilter(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "countsub@example.com", Name: "Count Sub",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_cs1",
		StripeSubscriptionID: "sub_cs1",
		Status:               model.SubscriptionActive,
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_cs1",
		StripeSubscriptionID: "sub_cs2",
		Status:               model.SubscriptionCanceled,
	})

	count, err := repo.CountSubscriptions(ctx, user.WithStatus(model.SubscriptionActive))
	if err != nil {
		t.Fatalf("CountSubscriptions() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestCountSubscriptions_WithSearchFilter(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u1, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-cs1",
		Email: "alice-cs@example.com", Name: "Alice CS",
	})
	u2, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-cs2",
		Email: "bob-cs@example.com", Name: "Bob CS",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u1.ID,
		StripeCustomerID:     "cus_alice_cs",
		StripeSubscriptionID: "sub_alice_cs",
		Status:               model.SubscriptionActive,
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u2.ID,
		StripeCustomerID:     "cus_bob_cs",
		StripeSubscriptionID: "sub_bob_cs",
		Status:               model.SubscriptionActive,
	})

	count, err := repo.CountSubscriptions(ctx, user.WithSearch("alice"))
	if err != nil {
		t.Fatalf("CountSubscriptions() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestCountSubscriptions_WithStatusAndSearch(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-both",
		Email: "both@example.com", Name: "Both Filters",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_both",
		StripeSubscriptionID: "sub_both1",
		Status:               model.SubscriptionActive,
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_both",
		StripeSubscriptionID: "sub_both2",
		Status:               model.SubscriptionCanceled,
	})

	count, err := repo.CountSubscriptions(ctx, user.WithStatus(model.SubscriptionActive), user.WithSearch("both"))
	if err != nil {
		t.Fatalf("CountSubscriptions() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestListSubscriptions_WithSearch(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u1, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-ls1",
		Email: "alice-ls@example.com", Name: "Alice LS",
	})
	u2, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-ls2",
		Email: "bob-ls@example.com", Name: "Bob LS",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u1.ID,
		StripeCustomerID:     "cus_als",
		StripeSubscriptionID: "sub_als",
		Status:               model.SubscriptionActive,
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u2.ID,
		StripeCustomerID:     "cus_bls",
		StripeSubscriptionID: "sub_bls",
		Status:               model.SubscriptionActive,
	})

	subs, err := repo.ListSubscriptions(ctx, user.WithSearch("alice"))
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) != 1 {
		t.Errorf("len(subs) = %d; want 1", len(subs))
	}
}

func TestListSubscriptions_Sort(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-sort",
		Email: "sort@example.com", Name: "Sort User",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_sort",
		StripeSubscriptionID: "sub_sort1",
		Status:               model.SubscriptionActive,
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_sort",
		StripeSubscriptionID: "sub_sort2",
		Status:               model.SubscriptionCanceled,
	})

	subs, err := repo.ListSubscriptions(ctx, user.WithSort("status", "asc"))
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) < 2 {
		t.Fatalf("len(subs) = %d; want >= 2", len(subs))
	}

	// "active" < "canceled" in ASC order.
	if subs[0].Status != model.SubscriptionActive {
		t.Errorf("first sub status = %q; want %q", subs[0].Status, model.SubscriptionActive)
	}
}

func TestListSubscriptions_Pagination(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-page",
		Email: "page@example.com", Name: "Page User",
	})

	for i := range 5 {
		_ = repo.UpsertSubscription(ctx, &model.Subscription{
			UserID:               u.ID,
			StripeCustomerID:     "cus_page",
			StripeSubscriptionID: fmt.Sprintf("sub_page_%d", i),
			Status:               model.SubscriptionActive,
		})
	}

	subs, err := repo.ListSubscriptions(ctx, user.WithPage(1, 2))
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) != 2 {
		t.Errorf("len(subs) = %d; want 2", len(subs))
	}
}

func TestListUsers_NoPagination(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	seedUsers(t, repo, 3)

	// Without pagination options, all users should be returned.
	users, err := repo.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 3 {
		t.Errorf("len(users) = %d; want 3", len(users))
	}
}

func TestListUsers_SearchAndTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-st1",
		Email: "alice-st@example.com", Name: "Alice ST",
	})
	u2, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-st2",
		Email: "bob-st@example.com", Name: "Bob ST",
	})
	_ = repo.UpdateTier(ctx, u2.ID, model.TierPro)

	// Search for "bob" AND filter by tier "pro" — should return 1 user.
	users, err := repo.ListUsers(ctx, user.WithSearch("bob"), user.WithTier(model.TierPro))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(users))
	}
}

func TestListUsers_InvalidSortFallback(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	seedUsers(t, repo, 2)

	// Invalid sort column should fallback to "created_at" without error.
	users, err := repo.ListUsers(ctx, user.WithSort("nonexistent_column", "desc"))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 2 {
		t.Errorf("len(users) = %d; want 2", len(users))
	}
}

func TestListSubscriptions_InvalidSortFallback(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-isf",
		Email: "isf@example.com", Name: "ISF User",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_isf",
		StripeSubscriptionID: "sub_isf",
		Status:               model.SubscriptionActive,
	})

	// Invalid sort column should fallback to "s.created_at".
	subs, err := repo.ListSubscriptions(ctx, user.WithSort("nonexistent", "desc"))
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) != 1 {
		t.Errorf("len(subs) = %d; want 1", len(subs))
	}
}

func TestCountUsers_WithSearch(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-cus1",
		Email: "alice-cus@example.com", Name: "Alice CUS",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-cus2",
		Email: "bob-cus@example.com", Name: "Bob CUS",
	})

	count, err := repo.CountUsers(ctx, user.WithSearch("alice"))
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestCountUsers_WithSearchAndTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-cust1",
		Email: "alice-cust@example.com", Name: "Alice CUST",
	})
	u2, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g-cust2",
		Email: "bob-cust@example.com", Name: "Bob CUST",
	})
	_ = repo.UpdateTier(ctx, u2.ID, model.TierPro)

	count, err := repo.CountUsers(ctx, user.WithSearch("bob"), user.WithTier(model.TierPro))
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestListUsers_AfterClose(t *testing.T) {
	t.Parallel()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	_ = repo.Close()

	ctx := context.Background()

	_, err = repo.ListUsers(ctx)
	if err == nil {
		t.Error("ListUsers() after Close() should return error")
	}
}

func TestCountUsers_AfterClose(t *testing.T) {
	t.Parallel()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	_ = repo.Close()

	ctx := context.Background()

	_, err = repo.CountUsers(ctx)
	if err == nil {
		t.Error("CountUsers() after Close() should return error")
	}
}

func TestListSubscriptions_AfterClose(t *testing.T) {
	t.Parallel()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	_ = repo.Close()

	ctx := context.Background()

	_, err = repo.ListSubscriptions(ctx)
	if err == nil {
		t.Error("ListSubscriptions() after Close() should return error")
	}
}

func TestCountSubscriptions_AfterClose(t *testing.T) {
	t.Parallel()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	_ = repo.Close()

	ctx := context.Background()

	_, err = repo.CountSubscriptions(ctx)
	if err == nil {
		t.Error("CountSubscriptions() after Close() should return error")
	}
}
