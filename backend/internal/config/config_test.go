package config_test

import (
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Env() != "development" {
		t.Errorf("Env() = %q; want %q", cfg.Env(), "development")
	}

	if cfg.Port() != "8080" {
		t.Errorf("Port() = %q; want %q", cfg.Port(), "8080")
	}

	if cfg.DatabaseURL() != "file:db/secretdrop.db?_journal_mode=WAL" {
		t.Errorf("DatabaseURL() = %q; want default", cfg.DatabaseURL())
	}

	if cfg.APIBaseURL() != "http://localhost:8080" {
		t.Errorf("APIBaseURL() = %q; want default", cfg.APIBaseURL())
	}

	if cfg.FrontendBaseURL() != "http://localhost:3000" {
		t.Errorf("FrontendBaseURL() = %q; want default", cfg.FrontendBaseURL())
	}

	if cfg.FromEmail() != "SecretDrop <noreply@secretdrop.us>" {
		t.Errorf("FromEmail() = %q; want default", cfg.FromEmail())
	}

	if cfg.SecretExpiry() != 10*time.Minute {
		t.Errorf("SecretExpiry() = %v; want 10m", cfg.SecretExpiry())
	}

	if cfg.CleanupInterval() != 1*time.Minute {
		t.Errorf("CleanupInterval() = %v; want 1m", cfg.CleanupInterval())
	}

	if cfg.GoogleClientID() != "" {
		t.Errorf("GoogleClientID() = %q; want empty", cfg.GoogleClientID())
	}

	if cfg.GoogleClientSecret() != "" {
		t.Errorf("GoogleClientSecret() = %q; want empty", cfg.GoogleClientSecret())
	}

	if cfg.GithubClientID() != "" {
		t.Errorf("GithubClientID() = %q; want empty", cfg.GithubClientID())
	}

	if cfg.GithubClientSecret() != "" {
		t.Errorf("GithubClientSecret() = %q; want empty", cfg.GithubClientSecret())
	}

	if cfg.JWTSecret() != "" {
		t.Errorf("JWTSecret() = %q; want empty", cfg.JWTSecret())
	}

	if cfg.StripeSecretKey() != "" {
		t.Errorf("StripeSecretKey() = %q; want empty", cfg.StripeSecretKey())
	}

	if cfg.StripeWebhookSecret() != "" {
		t.Errorf("StripeWebhookSecret() = %q; want empty", cfg.StripeWebhookSecret())
	}

	if cfg.StripePriceID() != "" {
		t.Errorf("StripePriceID() = %q; want empty", cfg.StripePriceID())
	}
}

func TestLoadWithAllEnvVars(t *testing.T) {
	t.Setenv("GOLANG_ENV", "production")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "file:test.db")
	t.Setenv("RESEND_API_KEY", "re_test_key")
	t.Setenv("API_BASE_URL", "https://api.example.com")
	t.Setenv("FRONTEND_BASE_URL", "https://example.com")
	t.Setenv("FROM_EMAIL", "test@example.com")
	t.Setenv("SECRET_EXPIRY", "30m")
	t.Setenv("CLEANUP_INTERVAL", "5m")
	t.Setenv("GOOGLE_CLIENT_ID", "google-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("GITHUB_CLIENT_ID", "github-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "github-secret")
	t.Setenv("JWT_SECRET", "jwt-secret-key")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_PRICE_ID", "price_123")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port() != "9090" {
		t.Errorf("Port() = %q; want %q", cfg.Port(), "9090")
	}

	if cfg.DatabaseURL() != "file:test.db" {
		t.Errorf("DatabaseURL() = %q; want %q", cfg.DatabaseURL(), "file:test.db")
	}

	if cfg.ResendAPIKey() != "re_test_key" {
		t.Errorf("ResendAPIKey() = %q; want %q", cfg.ResendAPIKey(), "re_test_key")
	}

	if cfg.APIBaseURL() != "https://api.example.com" {
		t.Errorf("APIBaseURL() = %q; want %q", cfg.APIBaseURL(), "https://api.example.com")
	}

	if cfg.FrontendBaseURL() != "https://example.com" {
		t.Errorf("FrontendBaseURL() = %q; want %q", cfg.FrontendBaseURL(), "https://example.com")
	}

	if cfg.FromEmail() != "test@example.com" {
		t.Errorf("FromEmail() = %q; want %q", cfg.FromEmail(), "test@example.com")
	}

	if cfg.SecretExpiry() != 30*time.Minute {
		t.Errorf("SecretExpiry() = %v; want 30m", cfg.SecretExpiry())
	}

	if cfg.CleanupInterval() != 5*time.Minute {
		t.Errorf("CleanupInterval() = %v; want 5m", cfg.CleanupInterval())
	}

	if cfg.GoogleClientID() != "google-id" {
		t.Errorf("GoogleClientID() = %q; want %q", cfg.GoogleClientID(), "google-id")
	}

	if cfg.GoogleClientSecret() != "google-secret" {
		t.Errorf("GoogleClientSecret() = %q; want %q", cfg.GoogleClientSecret(), "google-secret")
	}

	if cfg.GithubClientID() != "github-id" {
		t.Errorf("GithubClientID() = %q; want %q", cfg.GithubClientID(), "github-id")
	}

	if cfg.GithubClientSecret() != "github-secret" {
		t.Errorf("GithubClientSecret() = %q; want %q", cfg.GithubClientSecret(), "github-secret")
	}

	if cfg.JWTSecret() != "jwt-secret-key" {
		t.Errorf("JWTSecret() = %q; want %q", cfg.JWTSecret(), "jwt-secret-key")
	}

	if cfg.StripeSecretKey() != "sk_test_123" {
		t.Errorf("StripeSecretKey() = %q; want %q", cfg.StripeSecretKey(), "sk_test_123")
	}

	if cfg.StripeWebhookSecret() != "whsec_123" {
		t.Errorf("StripeWebhookSecret() = %q; want %q", cfg.StripeWebhookSecret(), "whsec_123")
	}

	if cfg.StripePriceID() != "price_123" {
		t.Errorf("StripePriceID() = %q; want %q", cfg.StripePriceID(), "price_123")
	}
}

func TestLoadRequiresResendAPIKeyInProduction(t *testing.T) {
	t.Setenv("GOLANG_ENV", "production")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() should fail without RESEND_API_KEY in production")
	}
}

func TestLoadSkipsResendAPIKeyInDevelopment(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ResendAPIKey() != "" {
		t.Errorf("ResendAPIKey() = %q; want empty in dev", cfg.ResendAPIKey())
	}
}

func TestIsDev(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.IsDev() {
		t.Error("IsDev() = false; want true")
	}
}

func TestIsDevFalseInProduction(t *testing.T) {
	t.Setenv("GOLANG_ENV", "production")
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("GOOGLE_CLIENT_ID", "google-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("GITHUB_CLIENT_ID", "github-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "github-secret")
	t.Setenv("JWT_SECRET", "jwt-secret-key")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_PRICE_ID", "price_123")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.IsDev() {
		t.Error("IsDev() = true; want false")
	}
}

func TestLoadInvalidSecretExpiry(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("SECRET_EXPIRY", "notaduration")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() should fail with invalid SECRET_EXPIRY")
	}
}

func TestLoadInvalidCleanupInterval(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("CLEANUP_INTERVAL", "notaduration")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() should fail with invalid CLEANUP_INTERVAL")
	}
}

func TestWithEnv(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("GOOGLE_CLIENT_ID", "google-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("GITHUB_CLIENT_ID", "github-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "github-secret")
	t.Setenv("JWT_SECRET", "jwt-secret-key")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_PRICE_ID", "price_123")

	cfg, err := config.Load(config.WithEnv("staging"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Env() != "staging" {
		t.Errorf("Env() = %q; want %q", cfg.Env(), "staging")
	}
}

func TestWithEnvEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithEnv(""))
	if err == nil {
		t.Fatal("WithEnv('') should fail")
	}
}

func TestWithPort(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithPort("3000"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port() != "3000" {
		t.Errorf("Port() = %q; want %q", cfg.Port(), "3000")
	}
}

func TestWithPortEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithPort(""))
	if err == nil {
		t.Fatal("WithPort('') should fail")
	}
}

func TestWithDatabaseURL(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithDatabaseURL("file:custom.db"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL() != "file:custom.db" {
		t.Errorf("DatabaseURL() = %q; want %q", cfg.DatabaseURL(), "file:custom.db")
	}
}

func TestWithDatabaseURLEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithDatabaseURL(""))
	if err == nil {
		t.Fatal("WithDatabaseURL('') should fail")
	}
}

func TestWithResendAPIKey(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithResendAPIKey("re_custom"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ResendAPIKey() != "re_custom" {
		t.Errorf("ResendAPIKey() = %q; want %q", cfg.ResendAPIKey(), "re_custom")
	}
}

func TestWithResendAPIKeyEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithResendAPIKey(""))
	if err == nil {
		t.Fatal("WithResendAPIKey('') should fail")
	}
}

func TestWithAPIBaseURL(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithAPIBaseURL("https://api.custom.com"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.APIBaseURL() != "https://api.custom.com" {
		t.Errorf("APIBaseURL() = %q; want %q", cfg.APIBaseURL(), "https://api.custom.com")
	}
}

func TestWithAPIBaseURLEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithAPIBaseURL(""))
	if err == nil {
		t.Fatal("WithAPIBaseURL('') should fail")
	}
}

func TestWithFrontendBaseURL(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithFrontendBaseURL("https://custom.com"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.FrontendBaseURL() != "https://custom.com" {
		t.Errorf("FrontendBaseURL() = %q; want %q", cfg.FrontendBaseURL(), "https://custom.com")
	}
}

func TestWithFrontendBaseURLEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithFrontendBaseURL(""))
	if err == nil {
		t.Fatal("WithFrontendBaseURL('') should fail")
	}
}

func TestWithFromEmail(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithFromEmail("custom@test.com"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.FromEmail() != "custom@test.com" {
		t.Errorf("FromEmail() = %q; want %q", cfg.FromEmail(), "custom@test.com")
	}
}

func TestWithFromEmailEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithFromEmail(""))
	if err == nil {
		t.Fatal("WithFromEmail('') should fail")
	}
}

func TestWithSecretExpiry(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithSecretExpiry(1 * time.Hour))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SecretExpiry() != 1*time.Hour {
		t.Errorf("SecretExpiry() = %v; want 1h", cfg.SecretExpiry())
	}
}

func TestWithSecretExpiryInvalid(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithSecretExpiry(-1 * time.Second))
	if err == nil {
		t.Fatal("WithSecretExpiry(-1s) should fail")
	}
}

func TestWithCleanupInterval(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithCleanupInterval(30 * time.Second))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.CleanupInterval() != 30*time.Second {
		t.Errorf("CleanupInterval() = %v; want 30s", cfg.CleanupInterval())
	}
}

func TestWithCleanupIntervalInvalid(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithCleanupInterval(0))
	if err == nil {
		t.Fatal("WithCleanupInterval(0) should fail")
	}
}

func TestLoadProductionRequiresAuthVars(t *testing.T) {
	t.Setenv("GOLANG_ENV", "production")
	t.Setenv("RESEND_API_KEY", "re_test_key")

	// Auth and billing vars are not set, should fail
	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() should fail without auth vars in production")
	}
}

func TestLoadProductionWithAllVars(t *testing.T) {
	t.Setenv("GOLANG_ENV", "production")
	t.Setenv("RESEND_API_KEY", "re_test_key")
	t.Setenv("GOOGLE_CLIENT_ID", "google-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("GITHUB_CLIENT_ID", "github-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "github-secret")
	t.Setenv("JWT_SECRET", "jwt-secret-key")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_PRICE_ID", "price_123")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GoogleClientID() != "google-id" {
		t.Errorf("GoogleClientID() = %q; want %q", cfg.GoogleClientID(), "google-id")
	}

	if cfg.GoogleClientSecret() != "google-secret" {
		t.Errorf("GoogleClientSecret() = %q; want %q", cfg.GoogleClientSecret(), "google-secret")
	}

	if cfg.GithubClientID() != "github-id" {
		t.Errorf("GithubClientID() = %q; want %q", cfg.GithubClientID(), "github-id")
	}

	if cfg.GithubClientSecret() != "github-secret" {
		t.Errorf("GithubClientSecret() = %q; want %q", cfg.GithubClientSecret(), "github-secret")
	}

	if cfg.JWTSecret() != "jwt-secret-key" {
		t.Errorf("JWTSecret() = %q; want %q", cfg.JWTSecret(), "jwt-secret-key")
	}

	if cfg.StripeSecretKey() != "sk_test_123" {
		t.Errorf("StripeSecretKey() = %q; want %q", cfg.StripeSecretKey(), "sk_test_123")
	}

	if cfg.StripeWebhookSecret() != "whsec_123" {
		t.Errorf("StripeWebhookSecret() = %q; want %q", cfg.StripeWebhookSecret(), "whsec_123")
	}

	if cfg.StripePriceID() != "price_123" {
		t.Errorf("StripePriceID() = %q; want %q", cfg.StripePriceID(), "price_123")
	}
}
