package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/model"

	_ "modernc.org/sqlite" // SQLite driver
)

const migration = `
CREATE TABLE IF NOT EXISTS secrets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    token           TEXT     NOT NULL UNIQUE,
    encrypted_blob  BLOB     NOT NULL,
    nonce           BLOB     NOT NULL,
    recipient_hash  TEXT     NOT NULL,
    expires_at      DATETIME NOT NULL,
    viewed          BOOLEAN  NOT NULL DEFAULT FALSE,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_secrets_token_recipient
    ON secrets(token, recipient_hash);
CREATE INDEX IF NOT EXISTS idx_secrets_expires_at
    ON secrets(expires_at);
`

// compile-time interface check.
var _ Repository = (*SQLite)(nil)

// SQLite implements Repository using a SQLite database.
type SQLite struct {
	db *sql.DB
}

// NewSQLite opens a SQLite database and runs migrations.
func NewSQLite(dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(migration); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("run migration: %w", err)
	}

	return &SQLite{db: db}, nil
}

// Store inserts a new secret record.
func (s *SQLite) Store(ctx context.Context, secret *model.Secret) error {
	const query = `
		INSERT INTO secrets (token, encrypted_blob, nonce, recipient_hash, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := s.db.ExecContext(ctx, query,
		secret.Token,
		secret.EncryptedBlob,
		secret.Nonce,
		secret.RecipientHash,
		secret.ExpiresAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert secret: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	secret.ID = id

	return nil
}

// FindByTokenAndHash retrieves a secret by token and recipient hash.
func (s *SQLite) FindByTokenAndHash(ctx context.Context, token, recipientHash string) (*model.Secret, error) {
	const query = `
		SELECT id, token, encrypted_blob, nonce, recipient_hash, expires_at, viewed, created_at
		FROM secrets
		WHERE token = ? AND recipient_hash = ?
	`

	secret := &model.Secret{}

	err := s.db.QueryRowContext(ctx, query, token, recipientHash).Scan(
		&secret.ID,
		&secret.Token,
		&secret.EncryptedBlob,
		&secret.Nonce,
		&secret.RecipientHash,
		&secret.ExpiresAt,
		&secret.Viewed,
		&secret.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}

		return nil, fmt.Errorf("query secret: %w", err)
	}

	return secret, nil
}

// Delete removes a secret by ID.
func (s *SQLite) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM secrets WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}

	return nil
}

// DeleteExpired removes all secrets that have expired before the given time.
func (s *SQLite) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM secrets WHERE expires_at <= ?", now.UTC())
	if err != nil {
		return 0, fmt.Errorf("delete expired secrets: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return count, nil
}

// Close closes the database connection.
func (s *SQLite) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}
