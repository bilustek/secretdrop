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

	if cfg.DatabaseURL() != "file:secretdrop.db?_journal_mode=WAL" {
		t.Errorf("DatabaseURL() = %q; want default", cfg.DatabaseURL())
	}

	if cfg.BaseURL() != "http://localhost:3000" {
		t.Errorf("BaseURL() = %q; want default", cfg.BaseURL())
	}

	if cfg.FromEmail() != "SecretDrop <noreply@secretdrop.app>" {
		t.Errorf("FromEmail() = %q; want default", cfg.FromEmail())
	}

	if cfg.SecretExpiry() != 10*time.Minute {
		t.Errorf("SecretExpiry() = %v; want 10m", cfg.SecretExpiry())
	}

	if cfg.CleanupInterval() != 1*time.Minute {
		t.Errorf("CleanupInterval() = %v; want 1m", cfg.CleanupInterval())
	}
}

func TestLoadWithAllEnvVars(t *testing.T) {
	t.Setenv("GOLANG_ENV", "production")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "file:test.db")
	t.Setenv("RESEND_API_KEY", "re_test_key")
	t.Setenv("BASE_URL", "https://example.com")
	t.Setenv("FROM_EMAIL", "test@example.com")
	t.Setenv("SECRET_EXPIRY", "30m")
	t.Setenv("CLEANUP_INTERVAL", "5m")

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

	if cfg.BaseURL() != "https://example.com" {
		t.Errorf("BaseURL() = %q; want %q", cfg.BaseURL(), "https://example.com")
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

func TestWithBaseURL(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := config.Load(config.WithBaseURL("https://custom.com"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BaseURL() != "https://custom.com" {
		t.Errorf("BaseURL() = %q; want %q", cfg.BaseURL(), "https://custom.com")
	}
}

func TestWithBaseURLEmpty(t *testing.T) {
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("RESEND_API_KEY", "")

	_, err := config.Load(config.WithBaseURL(""))
	if err == nil {
		t.Fatal("WithBaseURL('') should fail")
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
