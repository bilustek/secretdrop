package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultPort                   = "8080"
	defaultDatabaseURL            = "file:db/secretdrop.db?_journal_mode=WAL"
	defaultAPIBaseURL             = "http://localhost:8080"
	defaultFrontendBaseURL        = "http://localhost:3000"
	defaultFromEmail              = "SecretDrop <hello@secretdrop.us>"
	defaultReplyToEmail           = "support@bilustek.com"
	defaultSecretExpiry           = 10 * time.Minute
	defaultCleanupInterval        = 1 * time.Minute
	defaultEnv                    = "production"
	defaultSentryTracesSampleRate = 1.0
)

// Config holds all application configuration derived from environment variables.
type Config struct {
	env             string
	port            string
	databaseURL     string
	resendAPIKey    string
	apiBaseURL      string
	frontendBaseURL string
	fromEmail       string
	replyToEmail    string
	secretExpiry    time.Duration
	secretExpiryRaw string
	cleanupInterval time.Duration

	googleClientID      string
	googleClientSecret  string
	githubClientID      string
	githubClientSecret  string
	jwtSecret           string
	stripeSecretKey     string
	stripeWebhookSecret string
	stripePriceID       string

	slackWebhookSubscriptions string
	slackWebhookNotifications string

	adminUsername string
	adminPassword string

	sentryDSN              string
	sentryTracesSampleRate float64

	appleClientID   string
	appleTeamID     string
	appleKeyID      string
	applePrivateKey string

	stripeProjectMetaKey   string
	stripeProjectMetaValue string
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

// WithAPIBaseURL sets the API base URL for OAuth callbacks.
func WithAPIBaseURL(url string) Option {
	return func(c *Config) error {
		if url == "" {
			return errors.New("API base URL cannot be empty")
		}

		c.apiBaseURL = url

		return nil
	}
}

// WithFrontendBaseURL sets the frontend base URL for generated links.
func WithFrontendBaseURL(url string) Option {
	return func(c *Config) error {
		if url == "" {
			return errors.New("frontend base URL cannot be empty")
		}

		c.frontendBaseURL = url

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

// WithSentryDSN sets the Sentry DSN.
func WithSentryDSN(dsn string) Option {
	return func(c *Config) error {
		c.sentryDSN = dsn

		return nil
	}
}

// WithSentryTracesSampleRate sets the Sentry traces sample rate.
func WithSentryTracesSampleRate(rate float64) Option {
	return func(c *Config) error {
		if rate < 0 || rate > 1 {
			return errors.New("sentry traces sample rate must be between 0 and 1")
		}

		c.sentryTracesSampleRate = rate

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

// APIBaseURL returns the API base URL for OAuth callbacks.
func (c *Config) APIBaseURL() string { return c.apiBaseURL }

// FrontendBaseURL returns the frontend base URL for generated links.
func (c *Config) FrontendBaseURL() string { return c.frontendBaseURL }

// FromEmail returns the sender email address.
func (c *Config) FromEmail() string { return c.fromEmail }

// ReplyToEmail returns the Reply-To email address.
func (c *Config) ReplyToEmail() string { return c.replyToEmail }

// SecretExpiry returns the secret expiration duration.
func (c *Config) SecretExpiry() time.Duration { return c.secretExpiry }

// SecretExpiryRaw returns the original SECRET_EXPIRY string (e.g. "10m", "1h").
// Falls back to "10m" when SECRET_EXPIRY was not set.
func (c *Config) SecretExpiryRaw() string {
	if c.secretExpiryRaw != "" {
		return c.secretExpiryRaw
	}

	return "10m"
}

// CleanupInterval returns the cleanup interval duration.
func (c *Config) CleanupInterval() time.Duration { return c.cleanupInterval }

// GoogleClientID returns the Google OAuth client ID.
func (c *Config) GoogleClientID() string { return c.googleClientID }

// GoogleClientSecret returns the Google OAuth client secret.
func (c *Config) GoogleClientSecret() string { return c.googleClientSecret }

// GithubClientID returns the GitHub OAuth client ID.
func (c *Config) GithubClientID() string { return c.githubClientID }

// GithubClientSecret returns the GitHub OAuth client secret.
func (c *Config) GithubClientSecret() string { return c.githubClientSecret }

// JWTSecret returns the JWT signing secret.
func (c *Config) JWTSecret() string { return c.jwtSecret }

// StripeSecretKey returns the Stripe secret key.
func (c *Config) StripeSecretKey() string { return c.stripeSecretKey }

// StripeWebhookSecret returns the Stripe webhook signing secret.
func (c *Config) StripeWebhookSecret() string { return c.stripeWebhookSecret }

// StripePriceID returns the Stripe price ID for the subscription plan.
func (c *Config) StripePriceID() string { return c.stripePriceID }

// SlackWebhookSubscriptions returns the Slack webhook URL for subscription events.
func (c *Config) SlackWebhookSubscriptions() string { return c.slackWebhookSubscriptions }

// SlackWebhookNotifications returns the Slack webhook URL for error notifications.
func (c *Config) SlackWebhookNotifications() string { return c.slackWebhookNotifications }

// AdminUsername returns the admin Basic Auth username.
func (c *Config) AdminUsername() string { return c.adminUsername }

// AdminPassword returns the admin Basic Auth password.
func (c *Config) AdminPassword() string { return c.adminPassword }

// SentryDSN returns the Sentry DSN. Empty means Sentry is disabled.
func (c *Config) SentryDSN() string { return c.sentryDSN }

// SentryTracesSampleRate returns the Sentry traces sample rate.
func (c *Config) SentryTracesSampleRate() float64 { return c.sentryTracesSampleRate }

// AppleClientID returns the Apple Sign-In client ID (Services ID).
func (c *Config) AppleClientID() string { return c.appleClientID }

// AppleTeamID returns the Apple Developer Team ID.
func (c *Config) AppleTeamID() string { return c.appleTeamID }

// AppleKeyID returns the Apple Sign-In key ID.
func (c *Config) AppleKeyID() string { return c.appleKeyID }

// ApplePrivateKey returns the Apple Sign-In private key in PEM format.
func (c *Config) ApplePrivateKey() string { return c.applePrivateKey }

// StripeProjectMetaKey returns the Stripe metadata key for project filtering.
func (c *Config) StripeProjectMetaKey() string { return c.stripeProjectMetaKey }

// StripeProjectMetaValue returns the Stripe metadata value for project filtering.
func (c *Config) StripeProjectMetaValue() string { return c.stripeProjectMetaValue }

// IsDev returns true when the application is running in development mode.
func (c *Config) IsDev() bool { return c.env == "development" }

// Load reads environment variables and returns a validated Config.
func Load(opts ...Option) (*Config, error) {
	c := &Config{
		env:             envOrDefault("GOLANG_ENV", defaultEnv),
		port:            envOrDefault("PORT", defaultPort),
		databaseURL:     envOrDefault("DATABASE_URL", defaultDatabaseURL),
		resendAPIKey:    os.Getenv("RESEND_API_KEY"),
		apiBaseURL:      envOrDefault("API_BASE_URL", defaultAPIBaseURL),
		frontendBaseURL: envOrDefault("FRONTEND_BASE_URL", defaultFrontendBaseURL),
		fromEmail:       envOrDefault("FROM_EMAIL", defaultFromEmail),
		replyToEmail:    envOrDefault("REPLY_TO_EMAIL", defaultReplyToEmail),
		secretExpiry:    defaultSecretExpiry,
		cleanupInterval: defaultCleanupInterval,
	}

	if v := os.Getenv("SECRET_EXPIRY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse SECRET_EXPIRY: %w", err)
		}

		c.secretExpiry = d
		c.secretExpiryRaw = v
	}

	if v := os.Getenv("CLEANUP_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse CLEANUP_INTERVAL: %w", err)
		}

		c.cleanupInterval = d
	}

	c.googleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	c.googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	c.githubClientID = os.Getenv("GITHUB_CLIENT_ID")
	c.githubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	c.jwtSecret = os.Getenv("JWT_SECRET")
	c.stripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
	c.stripeWebhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	c.stripePriceID = os.Getenv("STRIPE_PRICE_ID")

	c.slackWebhookSubscriptions = os.Getenv("SLACK_WEBHOOK_SUBSCRIPTIONS")
	c.slackWebhookNotifications = os.Getenv("SLACK_WEBHOOK_NOTIFICATIONS")

	c.adminUsername = os.Getenv("ADMIN_USERNAME")
	c.adminPassword = os.Getenv("ADMIN_PASSWORD")

	c.sentryDSN = os.Getenv("SENTRY_DSN")
	c.sentryTracesSampleRate = defaultSentryTracesSampleRate

	if v := os.Getenv("SENTRY_TRACES_SAMPLE_RATE"); v != "" {
		rate, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("parse SENTRY_TRACES_SAMPLE_RATE: %w", err)
		}

		c.sentryTracesSampleRate = rate
	}

	c.appleClientID = os.Getenv("APPLE_CLIENT_ID")
	c.appleTeamID = os.Getenv("APPLE_TEAM_ID")
	c.appleKeyID = os.Getenv("APPLE_KEY_ID")
	c.applePrivateKey = os.Getenv("APPLE_PRIVATE_KEY")

	c.stripeProjectMetaKey = os.Getenv("STRIPE_PROJECT_METAKEY")
	c.stripeProjectMetaValue = os.Getenv("STRIPE_PROJECT_METADATA")

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	if !c.IsDev() {
		for _, kv := range []struct{ name, val string }{
			{"RESEND_API_KEY", c.resendAPIKey},
			{"GOOGLE_CLIENT_ID", c.googleClientID},
			{"GOOGLE_CLIENT_SECRET", c.googleClientSecret},
			{"GITHUB_CLIENT_ID", c.githubClientID},
			{"GITHUB_CLIENT_SECRET", c.githubClientSecret},
			{"JWT_SECRET", c.jwtSecret},
			{"STRIPE_SECRET_KEY", c.stripeSecretKey},
			{"STRIPE_WEBHOOK_SECRET", c.stripeWebhookSecret},
			{"STRIPE_PRICE_ID", c.stripePriceID},
		} {
			if kv.val == "" {
				return nil, fmt.Errorf("%s environment variable is required", kv.name)
			}
		}
	}

	return c, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
