package repository

import (
	"context"
	"time"

	"github.com/bilustek/secretdrop/internal/model"
)

// Repository defines the persistence operations for secrets.
type Repository interface {
	Store(ctx context.Context, secret *model.Secret) error
	FindByTokenAndHash(ctx context.Context, token, recipientHash string) (*model.Secret, error)
	Delete(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
	Close() error
}
