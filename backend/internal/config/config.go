package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	defaultPort            = "8080"
	defaultDatabaseURL     = "file:secretdrop.db?_journal_mode=WAL"
	defaultBaseURL         = "http://localhost:3000"
	defaultFromEmail       = "SecretDrop <noreply@secretdrop.app>"
	defaultSecretExpiry    = 10 * time.Minute
	defaultCleanupInterval = 1 * time.Minute
)

// Config holds all application configuration derived from environment variables.
type Config struct {
	Port            string
	DatabaseURL     string
	ResendAPIKey    string
	BaseURL         string
	FromEmail       string
	SecretExpiry    time.Duration
	CleanupInterval time.Duration
}

// Load reads environment variables and returns a validated Config.
func Load() (*Config, error) {
	cfg := &Config{
		Port:            envOrDefault("PORT", defaultPort),
		DatabaseURL:     envOrDefault("DATABASE_URL", defaultDatabaseURL),
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		BaseURL:         envOrDefault("BASE_URL", defaultBaseURL),
		FromEmail:       envOrDefault("FROM_EMAIL", defaultFromEmail),
		SecretExpiry:    defaultSecretExpiry,
		CleanupInterval: defaultCleanupInterval,
	}

	if cfg.ResendAPIKey == "" {
		return nil, errors.New("RESEND_API_KEY environment variable is required")
	}

	if v := os.Getenv("SECRET_EXPIRY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse SECRET_EXPIRY: %w", err)
		}

		cfg.SecretExpiry = d
	}

	if v := os.Getenv("CLEANUP_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse CLEANUP_INTERVAL: %w", err)
		}

		cfg.CleanupInterval = d
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
