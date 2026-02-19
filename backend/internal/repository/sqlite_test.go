package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/repository"
)

func newTestRepo(t *testing.T) *repository.SQLite {
	t.Helper()

	repo, err := repository.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	return repo
}

func TestStoreAndFind(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	secret := &model.Secret{
		Token:         "test-token-123",
		EncryptedBlob: []byte("encrypted-data"),
		Nonce:         []byte("nonce-data"),
		RecipientHash: "abc123hash",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	if err := repo.Store(ctx, secret); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	if secret.ID == 0 {
		t.Error("Store() should set ID")
	}

	found, err := repo.FindByTokenAndHash(ctx, "test-token-123", "abc123hash")
	if err != nil {
		t.Fatalf("FindByTokenAndHash() error = %v", err)
	}

	if found.Token != secret.Token {
		t.Errorf("Token = %q; want %q", found.Token, secret.Token)
	}

	if string(found.EncryptedBlob) != string(secret.EncryptedBlob) {
		t.Errorf("EncryptedBlob mismatch")
	}

	if found.Viewed {
		t.Error("Viewed should be false initially")
	}
}

func TestFindNotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.FindByTokenAndHash(ctx, "nonexistent", "nohash")
	if err != model.ErrNotFound {
		t.Errorf("FindByTokenAndHash() error = %v; want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	secret := &model.Secret{
		Token:         "delete-me",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	if err := repo.Store(ctx, secret); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	if err := repo.Delete(ctx, secret.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := repo.FindByTokenAndHash(ctx, "delete-me", "hash")
	if err != model.ErrNotFound {
		t.Errorf("after Delete(), FindByTokenAndHash() error = %v; want ErrNotFound", err)
	}
}

func TestDeleteExpired(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	expired := &model.Secret{
		Token:         "expired",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash1",
		ExpiresAt:     time.Now().Add(-1 * time.Minute).UTC(),
	}

	active := &model.Secret{
		Token:         "active",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash2",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	if err := repo.Store(ctx, expired); err != nil {
		t.Fatalf("Store(expired) error = %v", err)
	}

	if err := repo.Store(ctx, active); err != nil {
		t.Fatalf("Store(active) error = %v", err)
	}

	count, err := repo.DeleteExpired(ctx, time.Now())
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}

	if count != 1 {
		t.Errorf("DeleteExpired() count = %d; want 1", count)
	}

	_, err = repo.FindByTokenAndHash(ctx, "expired", "hash1")
	if err != model.ErrNotFound {
		t.Errorf("expired secret should be deleted, got error = %v", err)
	}

	found, err := repo.FindByTokenAndHash(ctx, "active", "hash2")
	if err != nil {
		t.Fatalf("active secret should still exist, got error = %v", err)
	}

	if found.Token != "active" {
		t.Errorf("Token = %q; want %q", found.Token, "active")
	}
}
