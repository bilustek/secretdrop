package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/email/noop"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/repository/sqlite"
	"github.com/bilusteknoloji/secretdrop/internal/service"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func newTestService(t *testing.T) (*service.SecretService, *sqlite.Repository, *noop.Sender) {
	t.Helper()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	sender := noop.New()

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL("http://localhost:3000"),
		service.WithFromEmail("noreply@test.com"),
		service.WithExpiry(10*time.Minute),
	)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}

	return svc, repo, sender
}

func parseLinkTokenAndKey(t *testing.T, link string) (string, string) {
	t.Helper()

	parts := strings.SplitN(link, "#", 2)
	if len(parts) != 2 {
		t.Fatalf("link should contain # fragment: %s", link)
	}

	tokenPart := parts[0]
	token := tokenPart[strings.LastIndex(tokenPart, "/")+1:]

	return token, parts[1]
}

func TestCreateAndRevealSecret(t *testing.T) {
	t.Parallel()

	svc, _, sender := newTestService(t)
	ctx := context.Background()

	req := &model.CreateRequest{
		Text: "DB_PASSWORD=secret123",
		To:   []string{"alice@example.com"},
	}

	resp, err := svc.Create(ctx, 0, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if resp.ID == "" {
		t.Error("Create() should return batch ID")
	}

	if len(resp.Recipients) != 1 {
		t.Fatalf("Recipients count = %d; want 1", len(resp.Recipients))
	}

	if len(sender.Calls) != 1 {
		t.Fatalf("email Calls count = %d; want 1", len(sender.Calls))
	}

	if sender.Calls[0].To != "alice@example.com" {
		t.Errorf("email To = %q; want %q", sender.Calls[0].To, "alice@example.com")
	}

	token, encodedKey := parseLinkTokenAndKey(t, resp.Recipients[0].Link)

	revealResp, err := svc.Reveal(ctx, token, &model.RevealRequest{
		Email: "alice@example.com",
		Key:   encodedKey,
	})
	if err != nil {
		t.Fatalf("Reveal() error = %v", err)
	}

	if revealResp.Text != "DB_PASSWORD=secret123" {
		t.Errorf("Reveal() text = %q; want %q", revealResp.Text, "DB_PASSWORD=secret123")
	}
}

func TestRevealDeletesSecret(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.Create(ctx, 0, &model.CreateRequest{
		Text: "one-time-secret",
		To:   []string{"bob@example.com"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	token, key := parseLinkTokenAndKey(t, resp.Recipients[0].Link)

	_, err = svc.Reveal(ctx, token, &model.RevealRequest{
		Email: "bob@example.com", Key: key,
	})
	if err != nil {
		t.Fatalf("first Reveal() error = %v", err)
	}

	_, err = svc.Reveal(ctx, token, &model.RevealRequest{
		Email: "bob@example.com", Key: key,
	})
	if err == nil {
		t.Error("second Reveal() should fail")
	}

	if appErr, ok := err.(*model.AppError); ok {
		if appErr.Type != "not_found" {
			t.Errorf("error type = %q; want %q", appErr.Type, "not_found")
		}
	}
}

func TestRevealWithWrongEmail(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.Create(ctx, 0, &model.CreateRequest{
		Text: "classified",
		To:   []string{"correct@example.com"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	token, key := parseLinkTokenAndKey(t, resp.Recipients[0].Link)

	_, err = svc.Reveal(ctx, token, &model.RevealRequest{
		Email: "wrong@example.com", Key: key,
	})
	if err == nil {
		t.Error("Reveal() with wrong email should fail")
	}
}

func TestCreateMultipleRecipients(t *testing.T) {
	t.Parallel()

	svc, _, sender := newTestService(t)
	ctx := context.Background()

	req := &model.CreateRequest{
		Text: "shared-secret",
		To:   []string{"a@example.com", "b@example.com", "c@example.com"},
	}

	resp, err := svc.Create(ctx, 0, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if len(resp.Recipients) != 3 {
		t.Errorf("Recipients count = %d; want 3", len(resp.Recipients))
	}

	if len(sender.Calls) != 3 {
		t.Errorf("email Calls = %d; want 3", len(sender.Calls))
	}

	for i, r := range resp.Recipients {
		token, key := parseLinkTokenAndKey(t, r.Link)

		revealResp, revealErr := svc.Reveal(ctx, token, &model.RevealRequest{
			Email: req.To[i], Key: key,
		})
		if revealErr != nil {
			t.Fatalf("Reveal() for %s error = %v", req.To[i], revealErr)
		}

		if revealResp.Text != "shared-secret" {
			t.Errorf("text for %s = %q; want %q", req.To[i], revealResp.Text, "shared-secret")
		}
	}
}

func TestCreateValidation(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestService(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *model.CreateRequest
		wantType string
	}{
		{
			name:     "empty text",
			req:      &model.CreateRequest{Text: "", To: []string{"a@b.com"}},
			wantType: "validation_error",
		},
		{
			name: "text too long",
			req: &model.CreateRequest{
				Text: strings.Repeat("x", model.MaxTextLength+1),
				To:   []string{"a@b.com"},
			},
			wantType: "text_too_long",
		},
		{
			name:     "no recipients",
			req:      &model.CreateRequest{Text: "secret", To: []string{}},
			wantType: "validation_error",
		},
		{
			name: "too many recipients",
			req: &model.CreateRequest{
				Text: "secret",
				To: []string{
					"a@b.com", "b@b.com", "c@b.com",
					"d@b.com", "e@b.com", "f@b.com",
				},
			},
			wantType: "too_many_recipients",
		},
		{
			name:     "invalid email",
			req:      &model.CreateRequest{Text: "secret", To: []string{"not-an-email"}},
			wantType: "invalid_email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, createErr := svc.Create(ctx, 0, tt.req)
			if createErr == nil {
				t.Fatal("Create() should return error")
			}

			appErr, ok := createErr.(*model.AppError)
			if !ok {
				t.Fatalf("error should be *model.AppError, got %T", createErr)
			}

			if appErr.Type != tt.wantType {
				t.Errorf("error type = %q; want %q", appErr.Type, tt.wantType)
			}
		})
	}
}

func TestRevealExpiredSecret(t *testing.T) {
	t.Parallel()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	sender := noop.New()

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL("http://localhost:3000"),
		service.WithFromEmail("noreply@test.com"),
		service.WithExpiry(1*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}

	ctx := context.Background()

	resp, err := svc.Create(ctx, 0, &model.CreateRequest{
		Text: "will-expire",
		To:   []string{"test@example.com"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	token, key := parseLinkTokenAndKey(t, resp.Recipients[0].Link)

	time.Sleep(10 * time.Millisecond)

	_, err = svc.Reveal(ctx, token, &model.RevealRequest{
		Email: "test@example.com", Key: key,
	})
	if err == nil {
		t.Fatal("Reveal() of expired secret should fail")
	}

	if appErr, ok := err.(*model.AppError); ok {
		if appErr.Type != "expired" {
			t.Errorf("error type = %q; want %q", appErr.Type, "expired")
		}
	}
}

func newTestServiceWithUserRepo(t *testing.T) (*service.SecretService, *usersqlite.Repository) {
	t.Helper()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	t.Cleanup(func() { userRepo.Close() })

	sender := noop.New()

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL("http://localhost:3000"),
		service.WithFromEmail("noreply@test.com"),
		service.WithExpiry(10*time.Minute),
		service.WithUserRepo(userRepo),
	)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}

	return svc, userRepo
}

func TestCreate_FreeTierLimitReached(t *testing.T) {
	t.Parallel()

	svc, userRepo := newTestServiceWithUserRepo(t)
	ctx := context.Background()

	u, err := userRepo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "free-user-1",
		Email:      "free@example.com",
		Name:       "Free User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Set secrets_used to the free tier limit (1)
	if err := userRepo.IncrementSecretsUsed(ctx, u.ID); err != nil {
		t.Fatalf("IncrementSecretsUsed() error = %v", err)
	}

	_, createErr := svc.Create(ctx, u.ID, &model.CreateRequest{
		Text: "secret",
		To:   []string{"recipient@example.com"},
	})
	if createErr == nil {
		t.Fatal("Create() should return error when limit reached")
	}

	appErr, ok := createErr.(*model.AppError)
	if !ok {
		t.Fatalf("error should be *model.AppError, got %T", createErr)
	}

	if appErr.Type != "limit_reached" {
		t.Errorf("error type = %q; want %q", appErr.Type, "limit_reached")
	}

	if appErr.StatusCode != 403 {
		t.Errorf("status code = %d; want 403", appErr.StatusCode)
	}
}

func TestCreate_ProTierLimitReached(t *testing.T) {
	t.Parallel()

	svc, userRepo := newTestServiceWithUserRepo(t)
	ctx := context.Background()

	u, err := userRepo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "pro-user-1",
		Email:      "pro@example.com",
		Name:       "Pro User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if err := userRepo.UpdateTier(ctx, u.ID, model.TierPro); err != nil {
		t.Fatalf("UpdateTier() error = %v", err)
	}

	// Set secrets_used to the pro tier limit (100)
	for range model.ProTierLimit {
		if err := userRepo.IncrementSecretsUsed(ctx, u.ID); err != nil {
			t.Fatalf("IncrementSecretsUsed() error = %v", err)
		}
	}

	_, createErr := svc.Create(ctx, u.ID, &model.CreateRequest{
		Text: "secret",
		To:   []string{"recipient@example.com"},
	})
	if createErr == nil {
		t.Fatal("Create() should return error when limit reached")
	}

	appErr, ok := createErr.(*model.AppError)
	if !ok {
		t.Fatalf("error should be *model.AppError, got %T", createErr)
	}

	if appErr.Type != "limit_reached" {
		t.Errorf("error type = %q; want %q", appErr.Type, "limit_reached")
	}

	if appErr.StatusCode != 403 {
		t.Errorf("status code = %d; want 403", appErr.StatusCode)
	}
}

func TestCreate_IncrementsUsage(t *testing.T) {
	t.Parallel()

	svc, userRepo := newTestServiceWithUserRepo(t)
	ctx := context.Background()

	u, err := userRepo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "usage-user-1",
		Email:      "usage@example.com",
		Name:       "Usage User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	_, createErr := svc.Create(ctx, u.ID, &model.CreateRequest{
		Text: "secret",
		To:   []string{"recipient@example.com"},
	})
	if createErr != nil {
		t.Fatalf("Create() error = %v", createErr)
	}

	updated, err := userRepo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updated.SecretsUsed != 1 {
		t.Errorf("SecretsUsed = %d; want 1", updated.SecretsUsed)
	}
}
