package model

import "time"

const (
	// TierFree is the default tier for new users.
	TierFree = "free"
	// TierPro is the paid tier with higher limits.
	TierPro = "pro"
	// TierTeam is the premium tier with highest limits.
	TierTeam = "team"

	// FreeTierLimit is the hardcoded fallback for free tier secrets limit.
	FreeTierLimit = 5
	// ProTierLimit is the hardcoded fallback for pro tier secrets limit.
	ProTierLimit = 100
	// TeamTierLimit is the hardcoded fallback for team tier secrets limit.
	TeamTierLimit = 1000
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
	Timezone    string

	// SecretsLimitOverride is an optional per-user override for secrets limit.
	// When nil, the tier default from the limits table is used.
	SecretsLimitOverride *int

	// RecipientsLimitOverride is an optional per-user override for recipients limit.
	// When nil, the tier default from the limits table is used.
	RecipientsLimitOverride *int

	// TierSecretsLimit is the secrets limit from the limits table for this user's tier.
	// Set at query time; zero means not loaded.
	TierSecretsLimit int

	// TierRecipientsLimit is the recipients limit from the limits table for this user's tier.
	// Set at query time; zero means not loaded.
	TierRecipientsLimit int
}

// SecretsLimit returns the effective secrets limit for this user.
// Priority: per-user override > limits table > hardcoded fallback.
func (u *User) SecretsLimit() int {
	if u.SecretsLimitOverride != nil {
		return *u.SecretsLimitOverride
	}

	if u.TierSecretsLimit > 0 {
		return u.TierSecretsLimit
	}

	switch u.Tier {
	case TierTeam:
		return TeamTierLimit
	case TierPro:
		return ProTierLimit
	default:
		return FreeTierLimit
	}
}

// CanCreateSecret reports whether the user has not yet reached their
// secret creation limit.
func (u *User) CanCreateSecret() bool {
	return u.SecretsUsed < u.SecretsLimit()
}

// MaxTextLength returns the maximum secret text length for this user's tier.
func (u *User) MaxTextLength() int {
	switch u.Tier {
	case TierTeam:
		return TeamMaxTextLength
	case TierPro:
		return ProMaxTextLength
	default:
		return FreeMaxTextLength
	}
}

// RecipientsLimit returns the maximum number of recipients per secret.
// Priority: per-user override > limits table > hardcoded fallback.
func (u *User) RecipientsLimit() int {
	if u.RecipientsLimitOverride != nil {
		return *u.RecipientsLimitOverride
	}

	if u.TierRecipientsLimit > 0 {
		return u.TierRecipientsLimit
	}

	switch u.Tier {
	case TierTeam:
		return TeamMaxRecipients
	case TierPro:
		return ProMaxRecipients
	default:
		return FreeMaxRecipients
	}
}
