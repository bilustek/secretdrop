package cleanup_test

import (
	"context"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/cleanup"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/repository"
)

func TestWorkerCleansExpiredSecrets(t *testing.T) {
	t.Parallel()

	repo, err := repository.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	ctx := context.Background()

	expired := &model.Secret{
		Token:         "expired-token",
		EncryptedBlob: []byte("data"),
		Nonce:         []byte("nonce"),
		RecipientHash: "hash1",
		ExpiresAt:     time.Now().Add(-5 * time.Minute).UTC(),
	}

	active := &model.Secret{
		Token:         "active-token",
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

	worker := cleanup.NewWorker(repo, 50*time.Millisecond)
	worker.Start()

	time.Sleep(200 * time.Millisecond)

	worker.Stop()

	_, err = repo.FindByTokenAndHash(ctx, "expired-token", "hash1")
	if err != model.ErrNotFound {
		t.Errorf("expired secret should be cleaned up, got error = %v", err)
	}

	found, err := repo.FindByTokenAndHash(ctx, "active-token", "hash2")
	if err != nil {
		t.Fatalf("active secret should still exist, got error = %v", err)
	}

	if found.Token != "active-token" {
		t.Errorf("Token = %q; want %q", found.Token, "active-token")
	}
}

func TestWorkerStopsGracefully(t *testing.T) {
	t.Parallel()

	repo, err := repository.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	worker := cleanup.NewWorker(repo, 1*time.Hour)
	worker.Start()

	done := make(chan struct{})

	go func() {
		worker.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Worker.Stop() did not return in time")
	}
}
