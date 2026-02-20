package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
    UNIQUE(provider, provider_id)
);
CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider, provider_id);
`

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

// Upsert inserts a new user or updates an existing one matched by provider and provider_id.
// On conflict, only email, name, and avatar_url are updated; tier and secrets_used are preserved.
func (r *Repository) Upsert(ctx context.Context, u *model.User) (*model.User, error) {
	const query = `
		INSERT INTO users (provider, provider_id, email, name, avatar_url)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(provider, provider_id) DO UPDATE SET
			email      = excluded.email,
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
		return fmt.Errorf("rows affected: %w", err)
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
		return fmt.Errorf("rows affected: %w", err)
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
		return fmt.Errorf("rows affected: %w", err)
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
