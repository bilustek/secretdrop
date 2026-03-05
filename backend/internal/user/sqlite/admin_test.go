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
