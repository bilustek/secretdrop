package model

import "time"

const (
	// TierFree is the default tier for new users.
	TierFree = "free"
	// TierPro is the paid tier with higher limits.
	TierPro = "pro"

	// FreeTierLimit is the maximum number of secrets a free user can create.
	FreeTierLimit = 5
	// ProTierLimit is the maximum number of secrets a pro user can create.
	ProTierLimit = 100
)

// User represents an authenticated user in the system.
type User struct {
	ID          int64
	Provider    string
	ProviderID  string
	Email       string
	Name        string
	AvatarURL   string
	Tier        string
	SecretsUsed int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SecretsLimit returns the maximum number of secrets the user can create
// based on their tier.
func (u *User) SecretsLimit() int {
	if u.Tier == TierPro {
		return ProTierLimit
	}

	return FreeTierLimit
}

// CanCreateSecret reports whether the user has not yet reached their
// secret creation limit.
func (u *User) CanCreateSecret() bool {
	return u.SecretsUsed < u.SecretsLimit()
}
