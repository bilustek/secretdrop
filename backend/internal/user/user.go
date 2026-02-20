package user //nolint:revive // "user" is intentional; os/user is unrelated to this domain package

import (
	"context"

	"github.com/bilusteknoloji/secretdrop/internal/model"
)

// Repository defines the persistence operations for users.
type Repository interface {
	Upsert(ctx context.Context, u *model.User) (*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindByProvider(ctx context.Context, provider, providerID string) (*model.User, error)
	IncrementSecretsUsed(ctx context.Context, id int64) error
	ResetSecretsUsed(ctx context.Context, id int64) error
	UpdateTier(ctx context.Context, id int64, tier string) error

	UpsertSubscription(ctx context.Context, sub *model.Subscription) error
	FindSubscriptionByUserID(ctx context.Context, userID int64) (*model.Subscription, error)
	FindUserByStripeCustomerID(ctx context.Context, customerID string) (*model.User, error)
	UpdateSubscriptionStatus(ctx context.Context, stripeSubID, status string) error
}
