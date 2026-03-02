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
				Text: strings.Repeat("x", model.ProMaxTextLength+1),
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

	// Set secrets_used to the free tier limit
	for range model.FreeTierLimit {
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

func TestCreate_WithExpiresIn(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.Create(ctx, 0, &model.CreateRequest{
		Text:      "secret-with-expiry",
		To:        []string{"alice@example.com"},
		ExpiresIn: "1d",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// ExpiresAt should be ~24h from now, not 10m
	expectedMin := time.Now().Add(23 * time.Hour)
	if resp.ExpiresAt.Before(expectedMin) {
		t.Errorf("ExpiresAt = %v; expected at least ~24h from now", resp.ExpiresAt)
	}
}

func TestCreate_WithInvalidExpiresIn(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, 0, &model.CreateRequest{
		Text:      "secret",
		To:        []string{"alice@example.com"},
		ExpiresIn: "99h",
	})
	if err == nil {
		t.Fatal("Create() should return error for invalid expires_in")
	}

	appErr, ok := err.(*model.AppError)
	if !ok {
		t.Fatalf("error should be *model.AppError, got %T", err)
	}

	if appErr.Type != "validation_error" {
		t.Errorf("error type = %q; want %q", appErr.Type, "validation_error")
	}
}

func TestCreate_WithEmptyExpiresIn(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.Create(ctx, 0, &model.CreateRequest{
		Text: "secret-default-expiry",
		To:   []string{"alice@example.com"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// ExpiresAt should be ~10m from now (default)
	expectedMax := time.Now().Add(11 * time.Minute)
	if resp.ExpiresAt.After(expectedMax) {
		t.Errorf("ExpiresAt = %v; expected around 10m from now", resp.ExpiresAt)
	}
}

func TestCreate_FreeUserRecipientLimit(t *testing.T) {
	t.Parallel()

	svc, userRepo := newTestServiceWithUserRepo(t)
	ctx := context.Background()

	u, err := userRepo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "free-recip-1",
		Email:      "freerecip@example.com",
		Name:       "Free Recip User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Free user should be limited to 1 recipient.
	_, createErr := svc.Create(ctx, u.ID, &model.CreateRequest{
		Text: "secret",
		To:   []string{"a@b.com", "b@b.com"},
	})
	if createErr == nil {
		t.Fatal("Create() should return error for free user with 2 recipients")
	}

	appErr, ok := createErr.(*model.AppError)
	if !ok {
		t.Fatalf("error should be *model.AppError, got %T", createErr)
	}

	if appErr.Type != "too_many_recipients" {
		t.Errorf("error type = %q; want %q", appErr.Type, "too_many_recipients")
	}
}

func TestNormalizeExpiry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "allowed 10m", raw: "10m", want: "10m"},
		{name: "allowed 1h", raw: "1h", want: "1h"},
		{name: "allowed 1d", raw: "1d", want: "1d"},
		{name: "allowed 30d", raw: "30d", want: "30d"},
		{name: "empty string", raw: "", want: "10m"},
		{name: "unparseable", raw: "notaduration", want: "10m"},
		{name: "30m rounds to 10m", raw: "30m", want: "10m"},
		{name: "45m rounds to 1h", raw: "45m", want: "1h"},
		{name: "24h maps to 1d", raw: "24h", want: "1d"},
		{name: "120h maps to 5d", raw: "120h", want: "5d"},
		{name: "240h maps to 10d", raw: "240h", want: "10d"},
		{name: "720h maps to 30d", raw: "720h", want: "30d"},
		{name: "2h rounds to 1h", raw: "2h", want: "1h"},
		{name: "48h tie-break picks shorter 1d", raw: "48h", want: "1d"},
		{name: "72h tie-break picks shorter 1d", raw: "72h", want: "1d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := service.NormalizeExpiry(tt.raw)
			if got != tt.want {
				t.Errorf("NormalizeExpiry(%q) = %q; want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDefaultExpiry(t *testing.T) {
	t.Parallel()

	t.Run("returns fallback when not set", func(t *testing.T) {
		t.Parallel()

		svc, _, _ := newTestService(t)
		if got := svc.DefaultExpiry(); got != "10m" {
			t.Errorf("DefaultExpiry() = %q; want %q", got, "10m")
		}
	})

	t.Run("returns normalized value when set", func(t *testing.T) {
		t.Parallel()

		repo, err := sqlite.New(":memory:")
		if err != nil {
			t.Fatalf("sqlite.New() error = %v", err)
		}
		t.Cleanup(func() { repo.Close() })

		svc, err := service.New(
			repo, noop.New(),
			service.WithBaseURL("http://localhost:3000"),
			service.WithFromEmail("noreply@test.com"),
			service.WithExpiry(10*time.Minute),
			service.WithDefaultExpiry("1h"),
		)
		if err != nil {
			t.Fatalf("service.New() error = %v", err)
		}

		if got := svc.DefaultExpiry(); got != "1h" {
			t.Errorf("DefaultExpiry() = %q; want %q", got, "1h")
		}
	})

	t.Run("normalizes non-whitelisted value", func(t *testing.T) {
		t.Parallel()

		repo, err := sqlite.New(":memory:")
		if err != nil {
			t.Fatalf("sqlite.New() error = %v", err)
		}
		t.Cleanup(func() { repo.Close() })

		svc, err := service.New(
			repo, noop.New(),
			service.WithBaseURL("http://localhost:3000"),
			service.WithFromEmail("noreply@test.com"),
			service.WithExpiry(30*time.Minute),
			service.WithDefaultExpiry("30m"),
		)
		if err != nil {
			t.Fatalf("service.New() error = %v", err)
		}

		got := svc.DefaultExpiry()
		if _, ok := service.AllowedExpiries[got]; !ok {
			t.Errorf("DefaultExpiry() = %q; want a value from AllowedExpiries", got)
		}
	})

	t.Run("aligns server expiry with normalized default", func(t *testing.T) {
		t.Parallel()

		repo, err := sqlite.New(":memory:")
		if err != nil {
			t.Fatalf("sqlite.New() error = %v", err)
		}
		t.Cleanup(func() { repo.Close() })

		svc, err := service.New(
			repo, noop.New(),
			service.WithBaseURL("http://localhost:3000"),
			service.WithFromEmail("noreply@test.com"),
			service.WithExpiry(30*time.Minute),
			service.WithDefaultExpiry("30m"),
		)
		if err != nil {
			t.Fatalf("service.New() error = %v", err)
		}

		label := svc.DefaultExpiry()
		wantDur := service.AllowedExpiries[label]

		if got := svc.Expiry(); got != wantDur {
			t.Errorf("Expiry() = %v; want %v (matching DefaultExpiry %q)", got, wantDur, label)
		}
	})
}

func TestCreate_ProUserMultipleRecipients(t *testing.T) {
	t.Parallel()

	svc, userRepo := newTestServiceWithUserRepo(t)
	ctx := context.Background()

	u, err := userRepo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "pro-recip-1",
		Email:      "prorecip@example.com",
		Name:       "Pro Recip User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if err := userRepo.UpdateTier(ctx, u.ID, model.TierPro); err != nil {
		t.Fatalf("UpdateTier() error = %v", err)
	}

	// Pro user should be able to send to 5 recipients.
	_, createErr := svc.Create(ctx, u.ID, &model.CreateRequest{
		Text: "secret",
		To:   []string{"a@b.com", "b@b.com", "c@b.com", "d@b.com", "e@b.com"},
	})
	if createErr != nil {
		t.Fatalf("Create() error = %v; pro user should allow 5 recipients", createErr)
	}
}

func TestBuildNotificationEmail_WithTimezone(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 3, 2, 15, 4, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timezone string
		wantHas  string
		wantNot  string
	}{
		{
			name:     "UTC timezone shows single time",
			timezone: "UTC",
			wantHas:  "Mar 2, 2026 at 3:04 PM UTC",
			wantNot:  "(3:04 PM UTC)",
		},
		{
			name:     "non-UTC timezone shows dual format",
			timezone: "America/New_York",
			wantHas:  "(3:04 PM UTC)",
		},
		{
			name:     "Etc/UTC alias shows single time",
			timezone: "Etc/UTC",
			wantHas:  "Mar 2, 2026 at 3:04 PM UTC",
			wantNot:  "(3:04 PM UTC)",
		},
		{
			name:     "empty timezone falls back to UTC",
			timezone: "",
			wantHas:  "Mar 2, 2026 at 3:04 PM UTC",
			wantNot:  "(3:04 PM UTC)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			html := service.BuildNotificationEmail("Test User", "https://example.com/s/token#key", expiresAt, tt.timezone)

			if !strings.Contains(html, tt.wantHas) {
				t.Errorf("email should contain %q, got:\n%s", tt.wantHas, html)
			}

			if tt.wantNot != "" && strings.Contains(html, tt.wantNot) {
				t.Errorf("email should NOT contain %q for %s timezone", tt.wantNot, tt.timezone)
			}
		})
	}
}
