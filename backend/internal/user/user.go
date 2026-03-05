package user //nolint:revive // "user" is intentional; os/user is unrelated to this domain package

import (
	"context"
	"time"

	"github.com/bilustek/secretdrop/internal/model"
)

// Repository defines the persistence operations for users.
type Repository interface {
	Upsert(ctx context.Context, u *model.User) (*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindByProvider(ctx context.Context, provider, providerID string) (*model.User, error)
	IncrementSecretsUsed(ctx context.Context, id int64) error
	ResetSecretsUsed(ctx context.Context, id int64) error
	UpdateTier(ctx context.Context, id int64, tier string) error
	UpdateTimezone(ctx context.Context, id int64, timezone string) error
	DeleteUser(ctx context.Context, id int64) error

	UpsertSubscription(ctx context.Context, sub *model.Subscription) error
	FindSubscriptionByUserID(ctx context.Context, userID int64) (*model.Subscription, error)
	FindUserByStripeCustomerID(ctx context.Context, customerID string) (*model.User, error)
	UpdateSubscriptionStatus(ctx context.Context, stripeSubID, status string) error
	UpdateSubscriptionPeriod(ctx context.Context, stripeSubID string, start, end time.Time) error

	GetLimits(ctx context.Context, tier string) (*TierLimits, error)
}

// AdminRepository extends Repository with admin query operations.
type AdminRepository interface {
	Repository

	ListUsers(ctx context.Context, opts ...ListOption) ([]*model.User, error)
	CountUsers(ctx context.Context, opts ...ListOption) (int64, error)
	ListSubscriptions(ctx context.Context, opts ...ListOption) ([]*SubscriptionWithUser, error)
	CountSubscriptions(ctx context.Context, opts ...ListOption) (int64, error)

	ListLimits(ctx context.Context) ([]*TierLimits, error)
	UpsertLimits(ctx context.Context, tl *TierLimits) error
	DeleteLimits(ctx context.Context, tier string) error
	UpdateSecretsLimitOverride(ctx context.Context, id int64, limit *int) error
	UpdateRecipientsLimitOverride(ctx context.Context, id int64, limit *int) error
	TierExists(ctx context.Context, tier string) (bool, error)
}

// SubscriptionWithUser holds a subscription joined with its user's email and name.
type SubscriptionWithUser struct {
	model.Subscription
	UserEmail string
	UserName  string
}

// DefaultPerPage is the default number of items returned per page.
const DefaultPerPage = 20

// ListOption configures a list query.
type ListOption func(*ListQuery)

// ListQuery holds the parameters for list queries.
type ListQuery struct {
	Search   string
	Tier     string
	Provider string
	Status   string
	Sort     string
	Order    string
	Page     int
	PerPage  int
}

// DefaultListQuery returns a ListQuery with sensible defaults.
func DefaultListQuery() *ListQuery {
	return &ListQuery{
		Sort:    "created_at",
		Order:   "desc",
		Page:    1,
		PerPage: DefaultPerPage,
	}
}

// ApplyOptions applies the given options to a default ListQuery.
func ApplyOptions(opts ...ListOption) *ListQuery {
	q := DefaultListQuery()
	for _, opt := range opts {
		opt(q)
	}

	return q
}

// WithSearch filters results by email (LIKE match).
func WithSearch(search string) ListOption {
	return func(q *ListQuery) {
		q.Search = search
	}
}

// WithTier filters users by tier.
func WithTier(tier string) ListOption {
	return func(q *ListQuery) {
		q.Tier = tier
	}
}

// WithProvider filters users by authentication provider.
func WithProvider(provider string) ListOption {
	return func(q *ListQuery) {
		q.Provider = provider
	}
}

// WithStatus filters subscriptions by status.
func WithStatus(status string) ListOption {
	return func(q *ListQuery) {
		q.Status = status
	}
}

// WithSort sets the sort field and order.
func WithSort(field, order string) ListOption {
	return func(q *ListQuery) {
		q.Sort = field
		q.Order = order
	}
}

// WithPage sets the page number and page size.
func WithPage(page, perPage int) ListOption {
	return func(q *ListQuery) {
		q.Page = page
		q.PerPage = perPage
	}
}
