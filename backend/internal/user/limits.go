package user //nolint:revive // "user" is intentional; os/user is unrelated to this domain package

// TierLimits holds the limit configuration for a tier.
type TierLimits struct {
	Tier            string
	SecretsLimit    int
	RecipientsLimit int
}
