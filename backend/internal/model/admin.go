package model

// AdminUserResponse represents a user in admin list responses.
type AdminUserResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Tier        string `json:"tier"`
	SecretsUsed int    `json:"secrets_used"`
	CreatedAt   string `json:"created_at"`
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

// AdminUpdateTierRequest is the body for PATCH /api/v1/admin/users/{id}.
type AdminUpdateTierRequest struct {
	Tier string `json:"tier"`
}
