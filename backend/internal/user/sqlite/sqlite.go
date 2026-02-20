package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"

	_ "modernc.org/sqlite" // SQLite driver
)

const migration = `
CREATE TABLE IF NOT EXISTS users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    provider     TEXT    NOT NULL,
    provider_id  TEXT    NOT NULL,
    email        TEXT    NOT NULL,
    name         TEXT    NOT NULL DEFAULT '',
    avatar_url   TEXT    NOT NULL DEFAULT '',
    tier         TEXT    NOT NULL DEFAULT 'free',
    secrets_used INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(email)
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE TABLE IF NOT EXISTS subscriptions (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                INTEGER NOT NULL REFERENCES users(id),
    stripe_customer_id     TEXT    NOT NULL,
    stripe_subscription_id TEXT    NOT NULL UNIQUE,
    status                 TEXT    NOT NULL DEFAULT 'active',
    current_period_start   DATETIME NOT NULL,
    current_period_end     DATETIME NOT NULL,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_id ON subscriptions(stripe_subscription_id);
`

const errFmtRowsAffected = "rows affected: %w"

// compile-time interface check.
var _ user.Repository = (*Repository)(nil)

// Repository implements user.Repository using a SQLite database.
type Repository struct {
	db *sql.DB
}

// New opens a SQLite database and runs migrations.
func New(dsn string) (*Repository, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(migration); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("run migration: %w", err)
	}

	return &Repository{db: db}, nil
}

// Upsert inserts a new user or updates an existing one matched by email.
// On conflict, provider, provider_id, name, and avatar_url are updated;
// tier and secrets_used are preserved.
func (r *Repository) Upsert(ctx context.Context, u *model.User) (*model.User, error) {
	const query = `
		INSERT INTO users (provider, provider_id, email, name, avatar_url)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			provider   = excluded.provider,
			provider_id = excluded.provider_id,
			name       = excluded.name,
			avatar_url = excluded.avatar_url,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, provider, provider_id, email, name, avatar_url, tier, secrets_used, created_at, updated_at
	`

	result := &model.User{}

	err := r.db.QueryRowContext(ctx, query,
		u.Provider,
		u.ProviderID,
		u.Email,
		u.Name,
		u.AvatarURL,
	).Scan(
		&result.ID,
		&result.Provider,
		&result.ProviderID,
		&result.Email,
		&result.Name,
		&result.AvatarURL,
		&result.Tier,
		&result.SecretsUsed,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	return result, nil
}

// FindByID retrieves a user by ID. Returns model.ErrNotFound if no user exists.
func (r *Repository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, provider, provider_id, email, name, avatar_url, tier, secrets_used, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.Provider,
		&u.ProviderID,
		&u.Email,
		&u.Name,
		&u.AvatarURL,
		&u.Tier,
		&u.SecretsUsed,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}

		return nil, fmt.Errorf("query user by id: %w", err)
	}

	return u, nil
}

// FindByProvider retrieves a user by provider and provider ID.
// Returns model.ErrNotFound if no user exists.
func (r *Repository) FindByProvider(ctx context.Context, provider, providerID string) (*model.User, error) {
	const query = `
		SELECT id, provider, provider_id, email, name, avatar_url, tier, secrets_used, created_at, updated_at
		FROM users
		WHERE provider = ? AND provider_id = ?
	`

	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, provider, providerID).Scan(
		&u.ID,
		&u.Provider,
		&u.ProviderID,
		&u.Email,
		&u.Name,
		&u.AvatarURL,
		&u.Tier,
		&u.SecretsUsed,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}

		return nil, fmt.Errorf("query user by provider: %w", err)
	}

	return u, nil
}

// IncrementSecretsUsed increments the secrets_used counter for the given user.
func (r *Repository) IncrementSecretsUsed(ctx context.Context, id int64) error {
	const query = `UPDATE users SET secrets_used = secrets_used + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("increment secrets used: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return nil
}

// ResetSecretsUsed resets the secrets_used counter to zero for the given user.
func (r *Repository) ResetSecretsUsed(ctx context.Context, id int64) error {
	const query = `UPDATE users SET secrets_used = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("reset secrets used: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return nil
}

// UpdateTier updates the tier for the given user.
func (r *Repository) UpdateTier(ctx context.Context, id int64, tier string) error {
	const query = `UPDATE users SET tier = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, tier, id)
	if err != nil {
		return fmt.Errorf("update tier: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return nil
}

// DeleteUser deletes a user and their subscriptions within a transaction.
// Returns model.ErrNotFound if no user exists with the given ID.
func (r *Repository) DeleteUser(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	if _, execErr := tx.ExecContext(ctx, "DELETE FROM subscriptions WHERE user_id = ?", id); execErr != nil {
		return fmt.Errorf("delete subscriptions: %w", execErr)
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("commit tx: %w", commitErr)
	}

	return nil
}

// UpsertSubscription inserts a new subscription or updates an existing one matched by stripe_subscription_id.
func (r *Repository) UpsertSubscription(ctx context.Context, sub *model.Subscription) error {
	const query = `
		INSERT INTO subscriptions
			(user_id, stripe_customer_id, stripe_subscription_id,
			 status, current_period_start, current_period_end)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(stripe_subscription_id) DO UPDATE SET
			status               = excluded.status,
			current_period_start = excluded.current_period_start,
			current_period_end   = excluded.current_period_end
	`

	_, err := r.db.ExecContext(ctx, query,
		sub.UserID,
		sub.StripeCustomerID,
		sub.StripeSubscriptionID,
		sub.Status,
		sub.CurrentPeriodStart,
		sub.CurrentPeriodEnd,
	)
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}

	return nil
}

// FindSubscriptionByUserID retrieves a subscription by user ID.
// Returns model.ErrNotFound if no subscription exists.
func (r *Repository) FindSubscriptionByUserID(ctx context.Context, userID int64) (*model.Subscription, error) {
	const query = `
		SELECT id, user_id, stripe_customer_id,
			stripe_subscription_id, status,
			current_period_start, current_period_end,
			created_at
		FROM subscriptions
		WHERE user_id = ?
	`

	s := &model.Subscription{}

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&s.ID,
		&s.UserID,
		&s.StripeCustomerID,
		&s.StripeSubscriptionID,
		&s.Status,
		&s.CurrentPeriodStart,
		&s.CurrentPeriodEnd,
		&s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}

		return nil, fmt.Errorf("query subscription by user id: %w", err)
	}

	return s, nil
}

// FindUserByStripeCustomerID retrieves a user by their Stripe customer ID
// via the subscriptions table. Returns model.ErrNotFound if no user exists.
func (r *Repository) FindUserByStripeCustomerID(ctx context.Context, customerID string) (*model.User, error) {
	const query = `
		SELECT u.id, u.provider, u.provider_id,
			u.email, u.name, u.avatar_url,
			u.tier, u.secrets_used,
			u.created_at, u.updated_at
		FROM users u
		JOIN subscriptions s ON u.id = s.user_id
		WHERE s.stripe_customer_id = ?
	`

	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, customerID).Scan(
		&u.ID,
		&u.Provider,
		&u.ProviderID,
		&u.Email,
		&u.Name,
		&u.AvatarURL,
		&u.Tier,
		&u.SecretsUsed,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}

		return nil, fmt.Errorf("query user by stripe customer id: %w", err)
	}

	return u, nil
}

// UpdateSubscriptionStatus updates the status of a subscription by its Stripe subscription ID.
// Returns model.ErrNotFound if no subscription exists with the given ID.
func (r *Repository) UpdateSubscriptionStatus(ctx context.Context, stripeSubID, status string) error {
	const query = `UPDATE subscriptions SET status = ? WHERE stripe_subscription_id = ?`

	result, err := r.db.ExecContext(ctx, query, status, stripeSubID)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return nil
}

// UpdateSubscriptionPeriod updates the current billing period for a subscription
// identified by its Stripe subscription ID.
// Returns model.ErrNotFound if no subscription exists with the given ID.
func (r *Repository) UpdateSubscriptionPeriod(ctx context.Context, stripeSubID string, start, end time.Time) error {
	const query = `UPDATE subscriptions
		SET current_period_start = ?, current_period_end = ?
		WHERE stripe_subscription_id = ?`

	result, err := r.db.ExecContext(ctx, query, start, end, stripeSubID)
	if err != nil {
		return fmt.Errorf("update subscription period: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	if err := r.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}
