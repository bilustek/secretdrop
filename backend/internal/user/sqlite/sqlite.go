package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/user"

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
CREATE TABLE IF NOT EXISTS limits (
    tier             TEXT PRIMARY KEY,
    secrets_limit    INTEGER NOT NULL DEFAULT 5,
    recipients_limit INTEGER NOT NULL DEFAULT 1,
    stripe_price_id  TEXT NOT NULL DEFAULT '',
    price_cents      INTEGER NOT NULL DEFAULT 0,
    currency         TEXT NOT NULL DEFAULT 'usd'
);
INSERT OR IGNORE INTO limits (tier, secrets_limit, recipients_limit) VALUES ('free', 5, 1);
INSERT OR IGNORE INTO limits (tier, secrets_limit, recipients_limit) VALUES ('pro', 100, 5);
`

const (
	errFmtRowsAffected = "rows affected: %w"
	sqlWhere           = " WHERE "
	sqlAnd             = " AND "
)

// compile-time interface checks.
var (
	_ user.Repository      = (*Repository)(nil)
	_ user.AdminRepository = (*Repository)(nil)
)

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

	// Add columns if they don't exist (SQLite has no ADD COLUMN IF NOT EXISTS).
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN secrets_limit INTEGER")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC'")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN recipients_limit INTEGER")
	_, _ = db.Exec("ALTER TABLE limits ADD COLUMN stripe_price_id TEXT NOT NULL DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE limits ADD COLUMN price_cents INTEGER NOT NULL DEFAULT 0")
	_, _ = db.Exec("ALTER TABLE limits ADD COLUMN currency TEXT NOT NULL DEFAULT 'usd'")
	_, _ = db.Exec(`INSERT OR IGNORE INTO limits
		(tier, secrets_limit, recipients_limit, stripe_price_id, price_cents, currency)
		VALUES ('team', 1000, 15, '', 2999, 'usd')`)
	_, _ = db.Exec("UPDATE limits SET price_cents = 299, currency = 'usd' WHERE tier = 'pro' AND price_cents = 0")

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
			provider    = excluded.provider,
			provider_id = excluded.provider_id,
			name        = CASE WHEN excluded.name = '' THEN users.name ELSE excluded.name END,
			avatar_url  = CASE WHEN excluded.avatar_url = '' THEN users.avatar_url ELSE excluded.avatar_url END,
			updated_at  = CURRENT_TIMESTAMP
		RETURNING id, provider, provider_id, email, name, avatar_url,
			tier, secrets_used, created_at, updated_at, secrets_limit, timezone, recipients_limit
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
		&result.SecretsLimitOverride,
		&result.Timezone,
		&result.RecipientsLimitOverride,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	return result, nil
}

// FindByID retrieves a user by ID. Returns model.ErrNotFound if no user exists.
func (r *Repository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, provider, provider_id, email, name, avatar_url,
			tier, secrets_used, created_at, updated_at, secrets_limit, timezone, recipients_limit
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
		&u.SecretsLimitOverride,
		&u.Timezone,
		&u.RecipientsLimitOverride,
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
		SELECT id, provider, provider_id, email, name, avatar_url,
			tier, secrets_used, created_at, updated_at, secrets_limit, timezone, recipients_limit
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
		&u.SecretsLimitOverride,
		&u.Timezone,
		&u.RecipientsLimitOverride,
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

// UpdateTimezone updates the timezone for the given user.
func (r *Repository) UpdateTimezone(ctx context.Context, id int64, timezone string) error {
	const query = `UPDATE users SET timezone = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, timezone, id)
	if err != nil {
		return fmt.Errorf("update timezone: %w", err)
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
			u.created_at, u.updated_at, u.secrets_limit, u.timezone, u.recipients_limit
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
		&u.SecretsLimitOverride,
		&u.Timezone,
		&u.RecipientsLimitOverride,
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

// FindTierByPriceID looks up the tier name associated with a Stripe price ID.
// Returns model.ErrNotFound if no tier has the given price ID.
func (r *Repository) FindTierByPriceID(ctx context.Context, priceID string) (string, error) {
	var tier string

	err := r.db.QueryRowContext(ctx,
		"SELECT tier FROM limits WHERE stripe_price_id = ?", priceID,
	).Scan(&tier)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", model.ErrNotFound
		}

		return "", fmt.Errorf("find tier by price ID: %w", err)
	}

	return tier, nil
}

// Allowed sort columns for users (whitelist to prevent SQL injection).
var userSortColumns = map[string]string{
	"created_at":   "created_at",
	"email":        "email",
	"name":         "name",
	"tier":         "tier",
	"secrets_used": "secrets_used",
}

// Allowed sort columns for subscriptions.
var subscriptionSortColumns = map[string]string{
	"created_at": "s.created_at",
	"status":     "s.status",
}

// ListUsers returns a paginated list of users with optional search, filter, and sort.
func (r *Repository) ListUsers(ctx context.Context, opts ...user.ListOption) ([]*model.User, error) {
	q := user.ApplyOptions(opts...)

	query := "SELECT id, provider, provider_id, email, name, avatar_url," +
		" tier, secrets_used, created_at, updated_at, secrets_limit, timezone, recipients_limit FROM users"
	var args []any
	var clauses []string

	if q.Search != "" {
		clauses = append(clauses, "(email LIKE ? OR name LIKE ?)")
		args = append(args, "%"+q.Search+"%", "%"+q.Search+"%")
	}

	if q.Tier != "" {
		clauses = append(clauses, "tier = ?")
		args = append(args, q.Tier)
	}

	if q.Provider != "" {
		clauses = append(clauses, "provider = ?")
		args = append(args, q.Provider)
	}

	if len(clauses) > 0 {
		query += sqlWhere + strings.Join(clauses, sqlAnd)
	}

	col := userSortColumns[q.Sort]
	if col == "" {
		col = "created_at"
	}

	order := "DESC"
	if strings.EqualFold(q.Order, "asc") {
		order = "ASC"
	}

	query += " ORDER BY " + col + " " + order

	if q.PerPage > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, q.PerPage, (q.Page-1)*q.PerPage)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var users []*model.User

	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(
			&u.ID, &u.Provider, &u.ProviderID, &u.Email,
			&u.Name, &u.AvatarURL, &u.Tier, &u.SecretsUsed,
			&u.CreatedAt, &u.UpdatedAt, &u.SecretsLimitOverride,
			&u.Timezone, &u.RecipientsLimitOverride,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

// CountUsers returns the total count of users matching the given options.
func (r *Repository) CountUsers(ctx context.Context, opts ...user.ListOption) (int64, error) {
	q := user.ApplyOptions(opts...)

	query := "SELECT COUNT(*) FROM users"
	var args []any
	var clauses []string

	if q.Search != "" {
		clauses = append(clauses, "(email LIKE ? OR name LIKE ?)")
		args = append(args, "%"+q.Search+"%", "%"+q.Search+"%")
	}

	if q.Tier != "" {
		clauses = append(clauses, "tier = ?")
		args = append(args, q.Tier)
	}

	if q.Provider != "" {
		clauses = append(clauses, "provider = ?")
		args = append(args, q.Provider)
	}

	if len(clauses) > 0 {
		query += sqlWhere + strings.Join(clauses, sqlAnd)
	}

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}

	return count, nil
}

// ListSubscriptions returns a paginated list of subscriptions with user info.
func (r *Repository) ListSubscriptions(
	ctx context.Context,
	opts ...user.ListOption,
) ([]*user.SubscriptionWithUser, error) {
	q := user.ApplyOptions(opts...)

	query := `SELECT s.id, s.user_id, s.stripe_customer_id,
		s.stripe_subscription_id, s.status,
		s.current_period_start, s.current_period_end,
		s.created_at, u.email, u.name
		FROM subscriptions s
		JOIN users u ON s.user_id = u.id`
	var args []any
	var clauses []string

	if q.Status != "" {
		clauses = append(clauses, "s.status = ?")
		args = append(args, q.Status)
	}

	if q.Search != "" {
		clauses = append(clauses, "(u.email LIKE ? OR u.name LIKE ?)")
		args = append(args, "%"+q.Search+"%", "%"+q.Search+"%")
	}

	if len(clauses) > 0 {
		query += sqlWhere + strings.Join(clauses, sqlAnd)
	}

	col := subscriptionSortColumns[q.Sort]
	if col == "" {
		col = "s.created_at"
	}

	order := "DESC"
	if strings.EqualFold(q.Order, "asc") {
		order = "ASC"
	}

	query += " ORDER BY " + col + " " + order

	if q.PerPage > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, q.PerPage, (q.Page-1)*q.PerPage)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var subs []*user.SubscriptionWithUser

	for rows.Next() {
		s := &user.SubscriptionWithUser{}
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.StripeCustomerID,
			&s.StripeSubscriptionID, &s.Status,
			&s.CurrentPeriodStart, &s.CurrentPeriodEnd,
			&s.CreatedAt, &s.UserEmail, &s.UserName,
		); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}

		subs = append(subs, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}

	return subs, nil
}

// CountSubscriptions returns the total count of subscriptions matching the given options.
func (r *Repository) CountSubscriptions(ctx context.Context, opts ...user.ListOption) (int64, error) {
	q := user.ApplyOptions(opts...)

	query := `SELECT COUNT(*) FROM subscriptions s JOIN users u ON s.user_id = u.id`
	var args []any
	var clauses []string

	if q.Status != "" {
		clauses = append(clauses, "s.status = ?")
		args = append(args, q.Status)
	}

	if q.Search != "" {
		clauses = append(clauses, "(u.email LIKE ? OR u.name LIKE ?)")
		args = append(args, "%"+q.Search+"%", "%"+q.Search+"%")
	}

	if len(clauses) > 0 {
		query += sqlWhere + strings.Join(clauses, sqlAnd)
	}

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count subscriptions: %w", err)
	}

	return count, nil
}

// GetLimits returns the tier limits for the given tier.
// Returns model.ErrNotFound if the tier does not exist.
func (r *Repository) GetLimits(ctx context.Context, tier string) (*user.TierLimits, error) {
	const query = `SELECT tier, secrets_limit, recipients_limit,
		stripe_price_id, price_cents, currency FROM limits WHERE tier = ?`

	tl := &user.TierLimits{}

	err := r.db.QueryRowContext(ctx, query, tier).Scan(
		&tl.Tier, &tl.SecretsLimit, &tl.RecipientsLimit,
		&tl.StripePriceID, &tl.PriceCents, &tl.Currency,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}

		return nil, fmt.Errorf("get limits: %w", err)
	}

	return tl, nil
}

// ListLimits returns all tier limit configurations ordered by tier name.
func (r *Repository) ListLimits(ctx context.Context) ([]*user.TierLimits, error) {
	const query = `SELECT tier, secrets_limit, recipients_limit,
		stripe_price_id, price_cents, currency FROM limits ORDER BY tier`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list limits: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var limits []*user.TierLimits

	for rows.Next() {
		tl := &user.TierLimits{}
		if err := rows.Scan(
			&tl.Tier, &tl.SecretsLimit, &tl.RecipientsLimit,
			&tl.StripePriceID, &tl.PriceCents, &tl.Currency,
		); err != nil {
			return nil, fmt.Errorf("scan limits: %w", err)
		}

		limits = append(limits, tl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate limits: %w", err)
	}

	return limits, nil
}

// UpsertLimits creates or updates the limits for a tier.
func (r *Repository) UpsertLimits(ctx context.Context, tl *user.TierLimits) error {
	const query = `
		INSERT INTO limits (tier, secrets_limit, recipients_limit, stripe_price_id, price_cents, currency)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tier) DO UPDATE SET
			secrets_limit    = excluded.secrets_limit,
			recipients_limit = excluded.recipients_limit,
			stripe_price_id  = excluded.stripe_price_id,
			price_cents      = excluded.price_cents,
			currency         = excluded.currency
	`

	if _, err := r.db.ExecContext(ctx, query,
		tl.Tier, tl.SecretsLimit, tl.RecipientsLimit,
		tl.StripePriceID, tl.PriceCents, tl.Currency,
	); err != nil {
		return fmt.Errorf("upsert limits: %w", err)
	}

	return nil
}

// DeleteLimits deletes the limits for a tier.
// Returns model.ErrNotFound if the tier does not exist.
func (r *Repository) DeleteLimits(ctx context.Context, tier string) error {
	const query = `DELETE FROM limits WHERE tier = ?`

	result, err := r.db.ExecContext(ctx, query, tier)
	if err != nil {
		return fmt.Errorf("delete limits: %w", err)
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

// UpdateSecretsLimitOverride sets or clears the per-user secrets limit override.
// Pass nil to clear the override.
func (r *Repository) UpdateSecretsLimitOverride(ctx context.Context, id int64, limit *int) error {
	const query = `UPDATE users SET secrets_limit = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, limit, id)
	if err != nil {
		return fmt.Errorf("update secrets limit override: %w", err)
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

// UpdateRecipientsLimitOverride sets or clears the per-user recipients limit override.
// Pass nil to clear the override.
func (r *Repository) UpdateRecipientsLimitOverride(ctx context.Context, id int64, limit *int) error {
	const query = `UPDATE users SET recipients_limit = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, limit, id)
	if err != nil {
		return fmt.Errorf("update recipients limit override: %w", err)
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

// TierExists checks if a tier exists in the limits table.
func (r *Repository) TierExists(ctx context.Context, tier string) (bool, error) {
	var count int

	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM limits WHERE tier = ?", tier).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("tier exists: %w", err)
	}

	return count > 0, nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	if err := r.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}
