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
	defaultEnv             = "production"
)

// Config holds all application configuration derived from environment variables.
type Config struct {
	env             string
	port            string
	databaseURL     string
	resendAPIKey    string
	baseURL         string
	fromEmail       string
	secretExpiry    time.Duration
	cleanupInterval time.Duration
}

// Option configures a Config value.
type Option func(*Config) error

// WithEnv sets the application environment.
func WithEnv(env string) Option {
	return func(c *Config) error {
		if env == "" {
			return errors.New("env cannot be empty")
		}

		c.env = env

		return nil
	}
}

// WithPort sets the server port.
func WithPort(port string) Option {
	return func(c *Config) error {
		if port == "" {
			return errors.New("port cannot be empty")
		}

		c.port = port

		return nil
	}
}

// WithDatabaseURL sets the database connection URL.
func WithDatabaseURL(url string) Option {
	return func(c *Config) error {
		if url == "" {
			return errors.New("database URL cannot be empty")
		}

		c.databaseURL = url

		return nil
	}
}

// WithResendAPIKey sets the Resend API key.
func WithResendAPIKey(key string) Option {
	return func(c *Config) error {
		if key == "" {
			return errors.New("resend API key cannot be empty")
		}

		c.resendAPIKey = key

		return nil
	}
}

// WithBaseURL sets the base URL for generated links.
func WithBaseURL(url string) Option {
	return func(c *Config) error {
		if url == "" {
			return errors.New("base URL cannot be empty")
		}

		c.baseURL = url

		return nil
	}
}

// WithFromEmail sets the sender email address.
func WithFromEmail(from string) Option {
	return func(c *Config) error {
		if from == "" {
			return errors.New("from email cannot be empty")
		}

		c.fromEmail = from

		return nil
	}
}

// WithSecretExpiry sets the secret expiration duration.
func WithSecretExpiry(d time.Duration) Option {
	return func(c *Config) error {
		if d <= 0 {
			return errors.New("secret expiry must be positive")
		}

		c.secretExpiry = d

		return nil
	}
}

// WithCleanupInterval sets the cleanup interval duration.
func WithCleanupInterval(d time.Duration) Option {
	return func(c *Config) error {
		if d <= 0 {
			return errors.New("cleanup interval must be positive")
		}

		c.cleanupInterval = d

		return nil
	}
}

// Env returns the application environment.
func (c *Config) Env() string { return c.env }

// Port returns the server port.
func (c *Config) Port() string { return c.port }

// DatabaseURL returns the database connection URL.
func (c *Config) DatabaseURL() string { return c.databaseURL }

// ResendAPIKey returns the Resend API key.
func (c *Config) ResendAPIKey() string { return c.resendAPIKey }

// BaseURL returns the base URL for generated links.
func (c *Config) BaseURL() string { return c.baseURL }

// FromEmail returns the sender email address.
func (c *Config) FromEmail() string { return c.fromEmail }

// SecretExpiry returns the secret expiration duration.
func (c *Config) SecretExpiry() time.Duration { return c.secretExpiry }

// CleanupInterval returns the cleanup interval duration.
func (c *Config) CleanupInterval() time.Duration { return c.cleanupInterval }

// IsDev returns true when the application is running in development mode.
func (c *Config) IsDev() bool { return c.env == "development" }

// Load reads environment variables and returns a validated Config.
func Load(opts ...Option) (*Config, error) {
	c := &Config{
		env:             envOrDefault("GOLANG_ENV", defaultEnv),
		port:            envOrDefault("PORT", defaultPort),
		databaseURL:     envOrDefault("DATABASE_URL", defaultDatabaseURL),
		resendAPIKey:    os.Getenv("RESEND_API_KEY"),
		baseURL:         envOrDefault("BASE_URL", defaultBaseURL),
		fromEmail:       envOrDefault("FROM_EMAIL", defaultFromEmail),
		secretExpiry:    defaultSecretExpiry,
		cleanupInterval: defaultCleanupInterval,
	}

	if v := os.Getenv("SECRET_EXPIRY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse SECRET_EXPIRY: %w", err)
		}

		c.secretExpiry = d
	}

	if v := os.Getenv("CLEANUP_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse CLEANUP_INTERVAL: %w", err)
		}

		c.cleanupInterval = d
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	if !c.IsDev() && c.resendAPIKey == "" {
		return nil, errors.New("RESEND_API_KEY environment variable is required")
	}

	return c, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
