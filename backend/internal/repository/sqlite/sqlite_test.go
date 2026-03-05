package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/repository"
	"github.com/bilustek/secretdrop/internal/repository/sqlite"
)

func newTestRepo(t *testing.T) *sqlite.Repository {
	t.Helper()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	return repo
}

func TestRepositoryImplementsInterface(t *testing.T) {
	t.Parallel()

	var _ repository.Repository = (*sqlite.Repository)(nil)
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

func TestNewInvalidDSN(t *testing.T) {
	t.Parallel()

	_, err := sqlite.New("file:/nonexistent/path/db.sqlite?mode=ro")
	if err == nil {
		t.Fatal("New() with invalid DSN should return error")
	}
}

func TestNewWithFailingOption(t *testing.T) {
	t.Parallel()

	failOpt := func(r *sqlite.Repository) error {
		return fmt.Errorf("option failed")
	}

	_, err := sqlite.New(":memory:", failOpt)
	if err == nil {
		t.Fatal("New() with failing option should return error")
	}

	if !strings.Contains(err.Error(), "apply option") {
		t.Errorf("error = %v; want it to contain 'apply option'", err)
	}
}

func TestNewWithSuccessfulOption(t *testing.T) {
	t.Parallel()

	called := false
	okOpt := func(r *sqlite.Repository) error {
		called = true

		return nil
	}

	repo, err := sqlite.New(":memory:", okOpt)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	defer repo.Close()

	if !called {
		t.Error("option should have been called")
	}
}

// closedRepo creates a repo and immediately closes it, returning
// the closed repo for testing error paths.
func closedRepo(t *testing.T) *sqlite.Repository {
	t.Helper()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}

	if err := repo.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	return repo
}

func TestStoreOnClosedDB(t *testing.T) {
	t.Parallel()

	repo := closedRepo(t)
	ctx := context.Background()

	secret := &model.Secret{
		Token:         "tok",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	err := repo.Store(ctx, secret)
	if err == nil {
		t.Fatal("Store() on closed DB should return error")
	}
}

func TestStoreDuplicateToken(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	secret := &model.Secret{
		Token:         "dup-token",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	if err := repo.Store(ctx, secret); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	secret2 := &model.Secret{
		Token:         "dup-token",
		EncryptedBlob: []byte("data2"),
		Nonce:         []byte("nonce2"),
		RecipientHash: "hash2",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	err := repo.Store(ctx, secret2)
	if err == nil {
		t.Fatal("Store() with duplicate token should return error")
	}
}

func TestFindByTokenAndHashOnClosedDB(t *testing.T) {
	t.Parallel()

	repo := closedRepo(t)
	ctx := context.Background()

	_, err := repo.FindByTokenAndHash(ctx, "tok", "hash")
	if err == nil {
		t.Fatal("FindByTokenAndHash() on closed DB should return error")
	}

	if errors.Is(err, model.ErrNotFound) {
		t.Error("error should not be ErrNotFound, but a DB-level error")
	}
}

func TestDeleteOnClosedDB(t *testing.T) {
	t.Parallel()

	repo := closedRepo(t)
	ctx := context.Background()

	err := repo.Delete(ctx, 1)
	if err == nil {
		t.Fatal("Delete() on closed DB should return error")
	}
}

func TestDeleteExpiredOnClosedDB(t *testing.T) {
	t.Parallel()

	repo := closedRepo(t)
	ctx := context.Background()

	_, err := repo.DeleteExpired(ctx, time.Now())
	if err == nil {
		t.Fatal("DeleteExpired() on closed DB should return error")
	}
}

func TestDeleteExpiredNoExpired(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	active := &model.Secret{
		Token:         "still-active",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	if err := repo.Store(ctx, active); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	count, err := repo.DeleteExpired(ctx, time.Now())
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}

	if count != 0 {
		t.Errorf("DeleteExpired() count = %d; want 0", count)
	}
}

func TestStoreCancelledContext(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	secret := &model.Secret{
		Token:         "ctx-cancel",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash",
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC(),
	}

	err := repo.Store(ctx, secret)
	if err == nil {
		t.Fatal("Store() with cancelled context should return error")
	}
}

func TestDeleteCancelledContext(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repo.Delete(ctx, 1)
	if err == nil {
		t.Fatal("Delete() with cancelled context should return error")
	}
}

func TestDeleteExpiredCancelledContext(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.DeleteExpired(ctx, time.Now())
	if err == nil {
		t.Fatal("DeleteExpired() with cancelled context should return error")
	}
}

func TestFindByTokenAndHashCancelledContext(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.FindByTokenAndHash(ctx, "tok", "hash")
	if err == nil {
		t.Fatal("FindByTokenAndHash() with cancelled context should return error")
	}
}
