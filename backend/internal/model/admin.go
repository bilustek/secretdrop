package model

// AdminUserResponse represents a user in admin list responses.
type AdminUserResponse struct {
	ID                      int64  `json:"id"`
	Email                   string `json:"email"`
	Name                    string `json:"name"`
	Provider                string `json:"provider"`
	Tier                    string `json:"tier"`
	SecretsUsed             int    `json:"secrets_used"`
	SecretsLimit            int    `json:"secrets_limit"`
	SecretsLimitOverride    *int   `json:"secrets_limit_override"`
	RecipientsLimit         int    `json:"recipients_limit"`
	RecipientsLimitOverride *int   `json:"recipients_limit_override"`
	CreatedAt               string `json:"created_at"`
}

// AdminUsersListResponse is the paginated list of users.
type AdminUsersListResponse struct {
	Users   []AdminUserResponse `json:"users"`
	Total   int64               `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"per_page"`
}

// AdminSubscriptionResponse represents a subscription in admin list responses.
type AdminSubscriptionResponse struct {
	ID                   int64  `json:"id"`
	UserID               int64  `json:"user_id"`
	UserEmail            string `json:"user_email"`
	UserName             string `json:"user_name"`
	StripeCustomerID     string `json:"stripe_customer_id"`
	StripeSubscriptionID string `json:"stripe_subscription_id"`
	Status               string `json:"status"`
	CurrentPeriodStart   string `json:"current_period_start"`
	CurrentPeriodEnd     string `json:"current_period_end"`
	CreatedAt            string `json:"created_at"`
}

// AdminSubscriptionsListResponse is the paginated list of subscriptions.
type AdminSubscriptionsListResponse struct {
	Subscriptions []AdminSubscriptionResponse `json:"subscriptions"`
	Total         int64                       `json:"total"`
	Page          int                         `json:"page"`
	PerPage       int                         `json:"per_page"`
}

// AdminUpdateUserRequest is the body for PATCH /api/v1/admin/users/{id}.
type AdminUpdateUserRequest struct {
	Tier                    *string `json:"tier,omitempty"`
	SecretsLimitOverride    *int    `json:"secrets_limit_override,omitempty"`
	ClearSecretsLimit       bool    `json:"clear_secrets_limit,omitempty"`
	RecipientsLimitOverride *int    `json:"recipients_limit_override,omitempty"`
	ClearRecipientsLimit    bool    `json:"clear_recipients_limit,omitempty"`
}

// AdminLimitsResponse represents a tier limit configuration.
type AdminLimitsResponse struct {
	Tier            string `json:"tier"`
	SecretsLimit    int    `json:"secrets_limit"`
	RecipientsLimit int    `json:"recipients_limit"`
	StripePriceID   string `json:"stripe_price_id"`
	PriceCents      int    `json:"price_cents"`
	Currency        string `json:"currency"`
}

// AdminUpsertLimitsRequest is the body for PUT /api/v1/admin/limits/{tier}.
type AdminUpsertLimitsRequest struct {
	SecretsLimit    int    `json:"secrets_limit"`
	RecipientsLimit int    `json:"recipients_limit"`
	StripePriceID   string `json:"stripe_price_id"`
	PriceCents      int    `json:"price_cents"`
	Currency        string `json:"currency"`
}
