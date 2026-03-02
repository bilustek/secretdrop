package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
	"github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func newTestRepo(t *testing.T) *sqlite.Repository {
	t.Helper()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}

	t.Cleanup(func() { _ = repo.Close() })

	return repo
}

func TestRepositoryImplementsInterface(t *testing.T) {
	t.Parallel()

	var _ user.Repository = (*sqlite.Repository)(nil)
}

func TestUpsert_NewUser(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u := &model.User{
		Provider:   "google",
		ProviderID: "google-123",
		Email:      "alice@example.com",
		Name:       "Alice",
		AvatarURL:  "https://example.com/alice.png",
	}

	got, err := repo.Upsert(ctx, u)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if got.ID == 0 {
		t.Error("Upsert() should set ID")
	}

	if got.Provider != "google" {
		t.Errorf("Provider = %q; want %q", got.Provider, "google")
	}

	if got.ProviderID != "google-123" {
		t.Errorf("ProviderID = %q; want %q", got.ProviderID, "google-123")
	}

	if got.Email != "alice@example.com" {
		t.Errorf("Email = %q; want %q", got.Email, "alice@example.com")
	}

	if got.Name != "Alice" {
		t.Errorf("Name = %q; want %q", got.Name, "Alice")
	}

	if got.AvatarURL != "https://example.com/alice.png" {
		t.Errorf("AvatarURL = %q; want %q", got.AvatarURL, "https://example.com/alice.png")
	}

	if got.Tier != model.TierFree {
		t.Errorf("Tier = %q; want %q", got.Tier, model.TierFree)
	}

	if got.SecretsUsed != 0 {
		t.Errorf("SecretsUsed = %d; want 0", got.SecretsUsed)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestUpsert_SameEmailDifferentProvider(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	// First login via Google.
	first, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "google-456",
		Email:      "bob@example.com",
		Name:       "Bob Google",
		AvatarURL:  "https://example.com/bob-google.png",
	})
	if err != nil {
		t.Fatalf("Upsert() first call error = %v", err)
	}

	// Modify tier and secrets_used to verify they are preserved on conflict.
	if err := repo.UpdateTier(ctx, first.ID, model.TierPro); err != nil {
		t.Fatalf("UpdateTier() error = %v", err)
	}

	if err := repo.IncrementSecretsUsed(ctx, first.ID); err != nil {
		t.Fatalf("IncrementSecretsUsed() error = %v", err)
	}

	// Second login via GitHub with same email — should find existing user.
	second, err := repo.Upsert(ctx, &model.User{
		Provider:   "github",
		ProviderID: "gh-789",
		Email:      "bob@example.com",
		Name:       "Bob GitHub",
		AvatarURL:  "https://example.com/bob-github.png",
	})
	if err != nil {
		t.Fatalf("Upsert() second call error = %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("ID = %d; want %d (same user by email)", second.ID, first.ID)
	}

	// Provider and provider_id should be updated to the latest login.
	if second.Provider != "github" {
		t.Errorf("Provider = %q; want %q", second.Provider, "github")
	}

	if second.ProviderID != "gh-789" {
		t.Errorf("ProviderID = %q; want %q", second.ProviderID, "gh-789")
	}

	if second.Name != "Bob GitHub" {
		t.Errorf("Name = %q; want %q", second.Name, "Bob GitHub")
	}

	if second.AvatarURL != "https://example.com/bob-github.png" {
		t.Errorf("AvatarURL = %q; want %q", second.AvatarURL, "https://example.com/bob-github.png")
	}

	// Tier and secrets_used must be preserved.
	if second.Tier != model.TierPro {
		t.Errorf("Tier = %q; want %q (should be preserved)", second.Tier, model.TierPro)
	}

	if second.SecretsUsed != 1 {
		t.Errorf("SecretsUsed = %d; want 1 (should be preserved)", second.SecretsUsed)
	}
}

func TestUpsert_EmptyNamePreservesExisting(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	// First login — Apple sends name on first consent.
	first, err := repo.Upsert(ctx, &model.User{
		Provider:   "apple",
		ProviderID: "apple-001",
		Email:      "eve@example.com",
		Name:       "Eve Apple",
		AvatarURL:  "",
	})
	if err != nil {
		t.Fatalf("Upsert() first call error = %v", err)
	}

	if first.Name != "Eve Apple" {
		t.Fatalf("Name = %q; want %q", first.Name, "Eve Apple")
	}

	// Second login — Apple does NOT send name on subsequent logins.
	second, err := repo.Upsert(ctx, &model.User{
		Provider:   "apple",
		ProviderID: "apple-001",
		Email:      "eve@example.com",
		Name:       "",
		AvatarURL:  "",
	})
	if err != nil {
		t.Fatalf("Upsert() second call error = %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("ID = %d; want %d (same user by email)", second.ID, first.ID)
	}

	// Name must be preserved when empty string is passed.
	if second.Name != "Eve Apple" {
		t.Errorf("Name = %q; want %q (should be preserved when empty)", second.Name, "Eve Apple")
	}
}

func TestUpsert_EmptyAvatarPreservesExisting(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	// First login with avatar.
	if _, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "google-avatar-001",
		Email:      "frank@example.com",
		Name:       "Frank",
		AvatarURL:  "https://example.com/frank.png",
	}); err != nil {
		t.Fatalf("Upsert() first call error = %v", err)
	}

	// Second login with empty avatar should preserve existing.
	second, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "google-avatar-001",
		Email:      "frank@example.com",
		Name:       "Frank Updated",
		AvatarURL:  "",
	})
	if err != nil {
		t.Fatalf("Upsert() second call error = %v", err)
	}

	if second.AvatarURL != "https://example.com/frank.png" {
		t.Errorf(
			"AvatarURL = %q; want %q (should be preserved when empty)",
			second.AvatarURL,
			"https://example.com/frank.png",
		)
	}

	// Non-empty name should still be updated.
	if second.Name != "Frank Updated" {
		t.Errorf("Name = %q; want %q", second.Name, "Frank Updated")
	}
}

func TestFindByID(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u := &model.User{
		Provider:   "google",
		ProviderID: "g-789",
		Email:      "carol@example.com",
		Name:       "Carol",
		AvatarURL:  "https://example.com/carol.png",
	}

	created, err := repo.Upsert(ctx, u)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID = %d; want %d", found.ID, created.ID)
	}

	if found.Email != "carol@example.com" {
		t.Errorf("Email = %q; want %q", found.Email, "carol@example.com")
	}

	if found.Tier != model.TierFree {
		t.Errorf("Tier = %q; want %q", found.Tier, model.TierFree)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, 99999)
	if err != model.ErrNotFound {
		t.Errorf("FindByID() error = %v; want model.ErrNotFound", err)
	}
}

func TestFindByProvider(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u := &model.User{
		Provider:   "github",
		ProviderID: "gh-101",
		Email:      "dave@example.com",
		Name:       "Dave",
		AvatarURL:  "https://example.com/dave.png",
	}

	created, err := repo.Upsert(ctx, u)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	found, err := repo.FindByProvider(ctx, "github", "gh-101")
	if err != nil {
		t.Fatalf("FindByProvider() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID = %d; want %d", found.ID, created.ID)
	}

	if found.Provider != "github" {
		t.Errorf("Provider = %q; want %q", found.Provider, "github")
	}

	if found.ProviderID != "gh-101" {
		t.Errorf("ProviderID = %q; want %q", found.ProviderID, "gh-101")
	}
}

func TestFindByProvider_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.FindByProvider(ctx, "nonexistent", "no-id")
	if err != model.ErrNotFound {
		t.Errorf("FindByProvider() error = %v; want model.ErrNotFound", err)
	}
}

func TestIncrementSecretsUsed(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u := &model.User{
		Provider:   "google",
		ProviderID: "g-inc",
		Email:      "eve@example.com",
		Name:       "Eve",
	}

	created, err := repo.Upsert(ctx, u)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if err := repo.IncrementSecretsUsed(ctx, created.ID); err != nil {
		t.Fatalf("IncrementSecretsUsed() first call error = %v", err)
	}

	if err := repo.IncrementSecretsUsed(ctx, created.ID); err != nil {
		t.Fatalf("IncrementSecretsUsed() second call error = %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.SecretsUsed != 2 {
		t.Errorf("SecretsUsed = %d; want 2", found.SecretsUsed)
	}
}

func TestResetSecretsUsed(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u := &model.User{
		Provider:   "google",
		ProviderID: "g-reset",
		Email:      "frank@example.com",
		Name:       "Frank",
	}

	created, err := repo.Upsert(ctx, u)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if err := repo.IncrementSecretsUsed(ctx, created.ID); err != nil {
		t.Fatalf("IncrementSecretsUsed() error = %v", err)
	}

	if err := repo.ResetSecretsUsed(ctx, created.ID); err != nil {
		t.Fatalf("ResetSecretsUsed() error = %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.SecretsUsed != 0 {
		t.Errorf("SecretsUsed = %d; want 0", found.SecretsUsed)
	}
}

func TestIncrementSecretsUsed_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.IncrementSecretsUsed(ctx, 99999)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("IncrementSecretsUsed() error = %v; want model.ErrNotFound", err)
	}
}

func TestResetSecretsUsed_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.ResetSecretsUsed(ctx, 99999)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("ResetSecretsUsed() error = %v; want model.ErrNotFound", err)
	}
}

func TestUpdateTier_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.UpdateTier(ctx, 99999, model.TierPro)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("UpdateTier() error = %v; want model.ErrNotFound", err)
	}
}

func TestUpdateTier(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u := &model.User{
		Provider:   "google",
		ProviderID: "g-tier",
		Email:      "grace@example.com",
		Name:       "Grace",
	}

	created, err := repo.Upsert(ctx, u)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if created.Tier != model.TierFree {
		t.Fatalf("initial Tier = %q; want %q", created.Tier, model.TierFree)
	}

	if err := repo.UpdateTier(ctx, created.ID, model.TierPro); err != nil {
		t.Fatalf("UpdateTier() error = %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.Tier != model.TierPro {
		t.Errorf("Tier = %q; want %q", found.Tier, model.TierPro)
	}
}

func createTestUserAndSubscription(t *testing.T, repo *sqlite.Repository) (*model.User, *model.Subscription) {
	t.Helper()

	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "github",
		ProviderID: "gh-sub-1",
		Email:      "sub@example.com",
		Name:       "Sub User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	sub := &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_test123",
		StripeSubscriptionID: "sub_test456",
		Status:               model.SubscriptionActive,
		CurrentPeriodStart:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrentPeriodEnd:     time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := repo.UpsertSubscription(ctx, sub); err != nil {
		t.Fatalf("UpsertSubscription() error = %v", err)
	}

	return u, sub
}

func TestUpsertSubscription(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "github",
		ProviderID: "gh-upsub",
		Email:      "upsub@example.com",
		Name:       "Upsub User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	sub := &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_abc",
		StripeSubscriptionID: "sub_xyz",
		Status:               model.SubscriptionActive,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
	}

	if err := repo.UpsertSubscription(ctx, sub); err != nil {
		t.Fatalf("UpsertSubscription() error = %v", err)
	}

	found, err := repo.FindSubscriptionByUserID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindSubscriptionByUserID() error = %v", err)
	}

	if found.ID == 0 {
		t.Error("ID should not be zero")
	}

	if found.UserID != u.ID {
		t.Errorf("UserID = %d; want %d", found.UserID, u.ID)
	}

	if found.StripeCustomerID != "cus_abc" {
		t.Errorf("StripeCustomerID = %q; want %q", found.StripeCustomerID, "cus_abc")
	}

	if found.StripeSubscriptionID != "sub_xyz" {
		t.Errorf("StripeSubscriptionID = %q; want %q", found.StripeSubscriptionID, "sub_xyz")
	}

	if found.Status != model.SubscriptionActive {
		t.Errorf("Status = %q; want %q", found.Status, model.SubscriptionActive)
	}

	if !found.CurrentPeriodStart.Equal(periodStart) {
		t.Errorf("CurrentPeriodStart = %v; want %v", found.CurrentPeriodStart, periodStart)
	}

	if !found.CurrentPeriodEnd.Equal(periodEnd) {
		t.Errorf("CurrentPeriodEnd = %v; want %v", found.CurrentPeriodEnd, periodEnd)
	}

	if found.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestUpsertSubscription_Update(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "github",
		ProviderID: "gh-updup",
		Email:      "updup@example.com",
		Name:       "Updup User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Initial insert.
	sub := &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_dup",
		StripeSubscriptionID: "sub_dup",
		Status:               model.SubscriptionActive,
		CurrentPeriodStart:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrentPeriodEnd:     time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := repo.UpsertSubscription(ctx, sub); err != nil {
		t.Fatalf("UpsertSubscription() first call error = %v", err)
	}

	// Upsert with same stripe_subscription_id but updated fields.
	newPeriodStart := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	newPeriodEnd := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	updated := &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_dup",
		StripeSubscriptionID: "sub_dup",
		Status:               model.SubscriptionPastDue,
		CurrentPeriodStart:   newPeriodStart,
		CurrentPeriodEnd:     newPeriodEnd,
	}

	if err := repo.UpsertSubscription(ctx, updated); err != nil {
		t.Fatalf("UpsertSubscription() second call error = %v", err)
	}

	found, err := repo.FindSubscriptionByUserID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindSubscriptionByUserID() error = %v", err)
	}

	if found.Status != model.SubscriptionPastDue {
		t.Errorf("Status = %q; want %q", found.Status, model.SubscriptionPastDue)
	}

	if !found.CurrentPeriodStart.Equal(newPeriodStart) {
		t.Errorf("CurrentPeriodStart = %v; want %v", found.CurrentPeriodStart, newPeriodStart)
	}

	if !found.CurrentPeriodEnd.Equal(newPeriodEnd) {
		t.Errorf("CurrentPeriodEnd = %v; want %v", found.CurrentPeriodEnd, newPeriodEnd)
	}
}

func TestFindSubscriptionByUserID(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	u, sub := createTestUserAndSubscription(t, repo)
	ctx := context.Background()

	found, err := repo.FindSubscriptionByUserID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindSubscriptionByUserID() error = %v", err)
	}

	if found.UserID != u.ID {
		t.Errorf("UserID = %d; want %d", found.UserID, u.ID)
	}

	if found.StripeCustomerID != sub.StripeCustomerID {
		t.Errorf("StripeCustomerID = %q; want %q", found.StripeCustomerID, sub.StripeCustomerID)
	}

	if found.StripeSubscriptionID != sub.StripeSubscriptionID {
		t.Errorf("StripeSubscriptionID = %q; want %q", found.StripeSubscriptionID, sub.StripeSubscriptionID)
	}

	if found.Status != model.SubscriptionActive {
		t.Errorf("Status = %q; want %q", found.Status, model.SubscriptionActive)
	}
}

func TestFindSubscriptionByUserID_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.FindSubscriptionByUserID(ctx, 99999)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("FindSubscriptionByUserID() error = %v; want model.ErrNotFound", err)
	}
}

func TestFindUserByStripeCustomerID(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	u, sub := createTestUserAndSubscription(t, repo)
	ctx := context.Background()

	found, err := repo.FindUserByStripeCustomerID(ctx, sub.StripeCustomerID)
	if err != nil {
		t.Fatalf("FindUserByStripeCustomerID() error = %v", err)
	}

	if found.ID != u.ID {
		t.Errorf("ID = %d; want %d", found.ID, u.ID)
	}

	if found.Email != u.Email {
		t.Errorf("Email = %q; want %q", found.Email, u.Email)
	}

	if found.Provider != u.Provider {
		t.Errorf("Provider = %q; want %q", found.Provider, u.Provider)
	}
}

func TestFindUserByStripeCustomerID_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.FindUserByStripeCustomerID(ctx, "cus_nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("FindUserByStripeCustomerID() error = %v; want model.ErrNotFound", err)
	}
}

func TestUpdateSubscriptionStatus(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	_, sub := createTestUserAndSubscription(t, repo)
	ctx := context.Background()

	if err := repo.UpdateSubscriptionStatus(ctx, sub.StripeSubscriptionID, model.SubscriptionCanceled); err != nil {
		t.Fatalf("UpdateSubscriptionStatus() error = %v", err)
	}

	found, err := repo.FindSubscriptionByUserID(ctx, sub.UserID)
	if err != nil {
		t.Fatalf("FindSubscriptionByUserID() error = %v", err)
	}

	if found.Status != model.SubscriptionCanceled {
		t.Errorf("Status = %q; want %q", found.Status, model.SubscriptionCanceled)
	}
}

func TestUpdateSubscriptionStatus_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.UpdateSubscriptionStatus(ctx, "sub_nonexistent", model.SubscriptionCanceled)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("UpdateSubscriptionStatus() error = %v; want model.ErrNotFound", err)
	}
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, sub := createTestUserAndSubscription(t, repo)

	if err := repo.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	// User should be gone.
	_, err := repo.FindByID(ctx, u.ID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("FindByID() after delete: error = %v; want model.ErrNotFound", err)
	}

	// Subscription should be gone too.
	_, err = repo.FindSubscriptionByUserID(ctx, sub.UserID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("FindSubscriptionByUserID() after delete: error = %v; want model.ErrNotFound", err)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.DeleteUser(ctx, 99999)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("DeleteUser() error = %v; want model.ErrNotFound", err)
	}
}

func TestDeleteUser_WithoutSubscription(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "g-delnosub",
		Email:      "delnosub@example.com",
		Name:       "No Sub",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if err := repo.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	_, findErr := repo.FindByID(ctx, u.ID)
	if !errors.Is(findErr, model.ErrNotFound) {
		t.Errorf("FindByID() after delete: error = %v; want model.ErrNotFound", findErr)
	}
}

func TestGetLimits(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	got, err := repo.GetLimits(ctx, "free")
	if err != nil {
		t.Fatalf("GetLimits() error = %v", err)
	}

	if got.Tier != "free" {
		t.Errorf("Tier = %q; want %q", got.Tier, "free")
	}

	if got.SecretsLimit != 5 {
		t.Errorf("SecretsLimit = %d; want 5", got.SecretsLimit)
	}

	if got.RecipientsLimit != 1 {
		t.Errorf("RecipientsLimit = %d; want 1", got.RecipientsLimit)
	}
}

func TestGetLimits_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.GetLimits(ctx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("GetLimits() error = %v; want model.ErrNotFound", err)
	}
}

func TestListLimits(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	limits, err := repo.ListLimits(ctx)
	if err != nil {
		t.Fatalf("ListLimits() error = %v", err)
	}

	if len(limits) < 2 {
		t.Fatalf("ListLimits() returned %d items; want at least 2", len(limits))
	}

	// Should be ordered by tier name: "free" before "pro".
	if limits[0].Tier != "free" {
		t.Errorf("limits[0].Tier = %q; want %q", limits[0].Tier, "free")
	}

	if limits[1].Tier != "pro" {
		t.Errorf("limits[1].Tier = %q; want %q", limits[1].Tier, "pro")
	}
}

func TestUpsertLimits_Insert(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	tl := &user.TierLimits{
		Tier:            "vip",
		SecretsLimit:    1000,
		RecipientsLimit: 20,
	}

	if err := repo.UpsertLimits(ctx, tl); err != nil {
		t.Fatalf("UpsertLimits() error = %v", err)
	}

	got, err := repo.GetLimits(ctx, "vip")
	if err != nil {
		t.Fatalf("GetLimits() error = %v", err)
	}

	if got.Tier != "vip" {
		t.Errorf("Tier = %q; want %q", got.Tier, "vip")
	}

	if got.SecretsLimit != 1000 {
		t.Errorf("SecretsLimit = %d; want 1000", got.SecretsLimit)
	}

	if got.RecipientsLimit != 20 {
		t.Errorf("RecipientsLimit = %d; want 20", got.RecipientsLimit)
	}
}

func TestUpsertLimits_Update(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	tl := &user.TierLimits{
		Tier:            "pro",
		SecretsLimit:    200,
		RecipientsLimit: 50,
	}

	if err := repo.UpsertLimits(ctx, tl); err != nil {
		t.Fatalf("UpsertLimits() error = %v", err)
	}

	got, err := repo.GetLimits(ctx, "pro")
	if err != nil {
		t.Fatalf("GetLimits() error = %v", err)
	}

	if got.SecretsLimit != 200 {
		t.Errorf("SecretsLimit = %d; want 200", got.SecretsLimit)
	}

	if got.RecipientsLimit != 50 {
		t.Errorf("RecipientsLimit = %d; want 50", got.RecipientsLimit)
	}
}

func TestDeleteLimits(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.DeleteLimits(ctx, "pro"); err != nil {
		t.Fatalf("DeleteLimits() error = %v", err)
	}

	_, err := repo.GetLimits(ctx, "pro")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("GetLimits() after delete: error = %v; want model.ErrNotFound", err)
	}
}

func TestDeleteLimits_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.DeleteLimits(ctx, "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("DeleteLimits() error = %v; want model.ErrNotFound", err)
	}
}

func TestTierExists(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	exists, err := repo.TierExists(ctx, "free")
	if err != nil {
		t.Fatalf("TierExists() error = %v", err)
	}

	if !exists {
		t.Error("TierExists(\"free\") = false; want true")
	}

	exists, err = repo.TierExists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("TierExists() error = %v", err)
	}

	if exists {
		t.Error("TierExists(\"nonexistent\") = true; want false")
	}
}

func TestUpdateSecretsLimitOverride_Set(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "g-override-set",
		Email:      "override-set@example.com",
		Name:       "Override Set",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	limit := 500
	if err := repo.UpdateSecretsLimitOverride(ctx, u.ID, &limit); err != nil {
		t.Fatalf("UpdateSecretsLimitOverride() error = %v", err)
	}

	found, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.SecretsLimitOverride == nil {
		t.Fatal("SecretsLimitOverride = nil; want 500")
	}

	if *found.SecretsLimitOverride != 500 {
		t.Errorf("SecretsLimitOverride = %d; want 500", *found.SecretsLimitOverride)
	}
}

func TestUpdateSecretsLimitOverride_Clear(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "g-override-clear",
		Email:      "override-clear@example.com",
		Name:       "Override Clear",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Set override first.
	limit := 500
	if err := repo.UpdateSecretsLimitOverride(ctx, u.ID, &limit); err != nil {
		t.Fatalf("UpdateSecretsLimitOverride() set error = %v", err)
	}

	// Clear override.
	if err := repo.UpdateSecretsLimitOverride(ctx, u.ID, nil); err != nil {
		t.Fatalf("UpdateSecretsLimitOverride() clear error = %v", err)
	}

	found, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.SecretsLimitOverride != nil {
		t.Errorf("SecretsLimitOverride = %v; want nil", *found.SecretsLimitOverride)
	}
}

func TestUpdateSecretsLimitOverride_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	limit := 500
	err := repo.UpdateSecretsLimitOverride(ctx, 99999, &limit)
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("UpdateSecretsLimitOverride() error = %v; want model.ErrNotFound", err)
	}
}
