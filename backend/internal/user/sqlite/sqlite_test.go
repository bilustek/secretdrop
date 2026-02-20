package sqlite_test

import (
	"context"
	"errors"
	"testing"

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

func TestUpsert_ExistingUser(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	// Insert initial user.
	initial := &model.User{
		Provider:   "github",
		ProviderID: "gh-456",
		Email:      "bob@example.com",
		Name:       "Bob",
		AvatarURL:  "https://example.com/bob.png",
	}

	first, err := repo.Upsert(ctx, initial)
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

	// Upsert with same provider+providerID but different name/email/avatar.
	updated := &model.User{
		Provider:   "github",
		ProviderID: "gh-456",
		Email:      "bob-new@example.com",
		Name:       "Bob Updated",
		AvatarURL:  "https://example.com/bob-new.png",
	}

	second, err := repo.Upsert(ctx, updated)
	if err != nil {
		t.Fatalf("Upsert() second call error = %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("ID = %d; want %d (same user)", second.ID, first.ID)
	}

	if second.Email != "bob-new@example.com" {
		t.Errorf("Email = %q; want %q", second.Email, "bob-new@example.com")
	}

	if second.Name != "Bob Updated" {
		t.Errorf("Name = %q; want %q", second.Name, "Bob Updated")
	}

	if second.AvatarURL != "https://example.com/bob-new.png" {
		t.Errorf("AvatarURL = %q; want %q", second.AvatarURL, "https://example.com/bob-new.png")
	}

	// Tier and secrets_used must be preserved.
	if second.Tier != model.TierPro {
		t.Errorf("Tier = %q; want %q (should be preserved)", second.Tier, model.TierPro)
	}

	if second.SecretsUsed != 1 {
		t.Errorf("SecretsUsed = %d; want 1 (should be preserved)", second.SecretsUsed)
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
