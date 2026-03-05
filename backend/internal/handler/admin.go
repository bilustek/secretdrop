package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/user"
)

const (
	defaultPerPage = 20
	maxPerPage     = 100
	timeFormat     = time.RFC3339

	errKeyError        = "error"
	errTypeInternal    = "internal_error"
	errTypeValidaton   = "validation_error"
	errTypeNotFound    = "not_found"
	errMsgUserNotFound = "User not found"
)

// AdminHandler handles admin API requests.
type AdminHandler struct {
	repo      user.AdminRepository
	canceller SubscriptionCanceller
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(repo user.AdminRepository, canceller SubscriptionCanceller) *AdminHandler {
	return &AdminHandler{repo: repo, canceller: canceller}
}

// Register registers admin routes on the given mux.
// The caller is responsible for wrapping with BasicAuth middleware.
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/users", h.ListUsers)
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateUser)
	mux.HandleFunc("GET /api/v1/admin/subscriptions", h.ListSubscriptions)
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)
	mux.HandleFunc("GET /api/v1/admin/limits", h.ListLimits)
	mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)
	mux.HandleFunc("DELETE /api/v1/admin/limits/{tier}", h.DeleteLimits)
}

// ListUsers handles GET /api/v1/admin/users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	opts := parseUserListOptions(r)

	users, err := h.repo.ListUsers(r.Context(), opts...)
	if err != nil {
		slog.Error("admin list users", errKeyError, err)
		writeError(w, errTypeInternal, "Failed to list users", http.StatusInternalServerError)

		return
	}

	count, countErr := h.repo.CountUsers(r.Context(), opts...)
	if countErr != nil {
		slog.Error("admin count users", errKeyError, countErr)
		writeError(w, errTypeInternal, "Failed to count users", http.StatusInternalServerError)

		return
	}

	limitsMap := h.buildLimitsMap(r)

	q := user.ApplyOptions(opts...)
	resp := model.AdminUsersListResponse{
		Users:   make([]model.AdminUserResponse, 0, len(users)),
		Total:   count,
		Page:    q.Page,
		PerPage: q.PerPage,
	}

	for _, u := range users {
		effectiveSecretsLimit := computeEffectiveLimit(u, limitsMap)
		effectiveRecipientsLimit := computeEffectiveRecipientsLimit(u, limitsMap)

		resp.Users = append(resp.Users, model.AdminUserResponse{
			ID:                      u.ID,
			Email:                   u.Email,
			Name:                    u.Name,
			Provider:                u.Provider,
			Tier:                    u.Tier,
			SecretsUsed:             u.SecretsUsed,
			SecretsLimit:            effectiveSecretsLimit,
			SecretsLimitOverride:    u.SecretsLimitOverride,
			RecipientsLimit:         effectiveRecipientsLimit,
			RecipientsLimitOverride: u.RecipientsLimitOverride,
			CreatedAt:               u.CreatedAt.Format(timeFormat),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateUser handles PATCH /api/v1/admin/users/{id}.
// Supports updating tier, setting a per-user secrets limit override,
// or clearing the override.
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, parseErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if parseErr != nil {
		writeError(w, errTypeValidaton, "Invalid user ID", http.StatusBadRequest)

		return
	}

	var req model.AdminUpdateUserRequest
	if jsonErr := readJSON(r, &req); jsonErr != nil {
		writeError(w, errTypeValidaton, "Invalid JSON body", http.StatusBadRequest)

		return
	}

	hasUpdate := req.Tier != nil || req.SecretsLimitOverride != nil || req.ClearSecretsLimit ||
		req.RecipientsLimitOverride != nil || req.ClearRecipientsLimit
	if !hasUpdate {
		writeError(w, errTypeValidaton, "No update fields provided", http.StatusBadRequest)

		return
	}

	if req.Tier != nil {
		exists, tierErr := h.repo.TierExists(r.Context(), *req.Tier)
		if tierErr != nil {
			slog.Error("admin check tier exists", errKeyError, tierErr)
			writeError(w, errTypeInternal, "Failed to validate tier", http.StatusInternalServerError)

			return
		}

		if !exists {
			writeError(w, errTypeValidaton, "Unknown tier", http.StatusBadRequest)

			return
		}

		if updateErr := h.repo.UpdateTier(r.Context(), id, *req.Tier); updateErr != nil {
			if errors.Is(updateErr, model.ErrNotFound) {
				writeError(w, errTypeNotFound, errMsgUserNotFound, http.StatusNotFound)
			} else {
				slog.Error("admin update tier", errKeyError, updateErr)
				writeError(w, errTypeInternal, "Failed to update tier", http.StatusInternalServerError)
			}

			return
		}
	}

	if req.ClearSecretsLimit {
		if clearErr := h.repo.UpdateSecretsLimitOverride(r.Context(), id, nil); clearErr != nil {
			if errors.Is(clearErr, model.ErrNotFound) {
				writeError(w, errTypeNotFound, errMsgUserNotFound, http.StatusNotFound)
			} else {
				slog.Error("admin clear secrets limit", errKeyError, clearErr)
				writeError(w, errTypeInternal, "Failed to clear secrets limit", http.StatusInternalServerError)
			}

			return
		}
	} else if req.SecretsLimitOverride != nil {
		if *req.SecretsLimitOverride <= 0 {
			writeError(w, errTypeValidaton, "secrets_limit_override must be positive", http.StatusBadRequest)

			return
		}

		if overrideErr := h.repo.UpdateSecretsLimitOverride(
			r.Context(), id, req.SecretsLimitOverride,
		); overrideErr != nil {
			if errors.Is(overrideErr, model.ErrNotFound) {
				writeError(w, errTypeNotFound, errMsgUserNotFound, http.StatusNotFound)
			} else {
				slog.Error("admin update secrets limit override", errKeyError, overrideErr)
				writeError(
					w, errTypeInternal, "Failed to update secrets limit",
					http.StatusInternalServerError,
				)
			}

			return
		}
	}

	if req.ClearRecipientsLimit {
		if clearErr := h.repo.UpdateRecipientsLimitOverride(r.Context(), id, nil); clearErr != nil {
			if errors.Is(clearErr, model.ErrNotFound) {
				writeError(w, errTypeNotFound, errMsgUserNotFound, http.StatusNotFound)
			} else {
				slog.Error("admin clear recipients limit", errKeyError, clearErr)
				writeError(w, errTypeInternal, "Failed to clear recipients limit", http.StatusInternalServerError)
			}

			return
		}
	} else if req.RecipientsLimitOverride != nil {
		if *req.RecipientsLimitOverride <= 0 {
			writeError(w, errTypeValidaton, "recipients_limit_override must be positive", http.StatusBadRequest)

			return
		}

		if overrideErr := h.repo.UpdateRecipientsLimitOverride(
			r.Context(), id, req.RecipientsLimitOverride,
		); overrideErr != nil {
			if errors.Is(overrideErr, model.ErrNotFound) {
				writeError(w, errTypeNotFound, errMsgUserNotFound, http.StatusNotFound)
			} else {
				slog.Error("admin update recipients limit override", errKeyError, overrideErr)
				writeError(
					w, errTypeInternal, "Failed to update recipients limit",
					http.StatusInternalServerError,
				)
			}

			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateTier is kept as an alias for backward compatibility in tests.
// Deprecated: use UpdateUser instead.
func (h *AdminHandler) UpdateTier(w http.ResponseWriter, r *http.Request) {
	h.UpdateUser(w, r)
}

// ListSubscriptions handles GET /api/v1/admin/subscriptions.
func (h *AdminHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	opts := parseSubscriptionListOptions(r)

	subs, err := h.repo.ListSubscriptions(r.Context(), opts...)
	if err != nil {
		slog.Error("admin list subscriptions", errKeyError, err)
		writeError(w, errTypeInternal, "Failed to list subscriptions", http.StatusInternalServerError)

		return
	}

	count, err := h.repo.CountSubscriptions(r.Context(), opts...)
	if err != nil {
		slog.Error("admin count subscriptions", errKeyError, err)
		writeError(w, errTypeInternal, "Failed to count subscriptions", http.StatusInternalServerError)

		return
	}

	q := user.ApplyOptions(opts...)
	resp := model.AdminSubscriptionsListResponse{
		Subscriptions: make([]model.AdminSubscriptionResponse, 0, len(subs)),
		Total:         count,
		Page:          q.Page,
		PerPage:       q.PerPage,
	}

	for _, s := range subs {
		resp.Subscriptions = append(resp.Subscriptions, model.AdminSubscriptionResponse{
			ID:                   s.ID,
			UserID:               s.UserID,
			UserEmail:            s.UserEmail,
			UserName:             s.UserName,
			StripeCustomerID:     s.StripeCustomerID,
			StripeSubscriptionID: s.StripeSubscriptionID,
			Status:               s.Status,
			CurrentPeriodStart:   s.CurrentPeriodStart.Format(timeFormat),
			CurrentPeriodEnd:     s.CurrentPeriodEnd.Format(timeFormat),
			CreatedAt:            s.CreatedAt.Format(timeFormat),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// CancelSubscription handles DELETE /api/v1/admin/subscriptions/{id}.
// The {id} path parameter is the subscription's database ID (not Stripe ID).
// It looks up the subscription by user_id to find the Stripe subscription ID.
func (h *AdminHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, errTypeValidaton, "Invalid subscription ID", http.StatusBadRequest)

		return
	}

	sub, err := h.repo.FindSubscriptionByUserID(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, errTypeNotFound, "Subscription not found", http.StatusNotFound)
		} else {
			slog.Error("admin find subscription", errKeyError, err)
			writeError(w, errTypeInternal, "Failed to find subscription", http.StatusInternalServerError)
		}

		return
	}

	if h.canceller != nil {
		if cancelErr := h.canceller.CancelSubscription(r.Context(), sub.StripeSubscriptionID); cancelErr != nil {
			slog.Error("admin cancel stripe subscription", errKeyError, cancelErr)
			writeError(w, errTypeInternal, "Failed to cancel subscription", http.StatusInternalServerError)

			return
		}
	}

	err = h.repo.UpdateSubscriptionStatus(r.Context(), sub.StripeSubscriptionID, model.SubscriptionCanceled)
	if err != nil {
		slog.Error("admin update subscription status", errKeyError, err)
		writeError(w, errTypeInternal, "Failed to update subscription status", http.StatusInternalServerError)

		return
	}

	if err := h.repo.UpdateTier(r.Context(), sub.UserID, model.TierFree); err != nil {
		slog.Error("admin downgrade tier", errKeyError, err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListLimits handles GET /api/v1/admin/limits.
func (h *AdminHandler) ListLimits(w http.ResponseWriter, r *http.Request) {
	limits, err := h.repo.ListLimits(r.Context())
	if err != nil {
		slog.Error("admin list limits", errKeyError, err)
		writeError(w, errTypeInternal, "Failed to list limits", http.StatusInternalServerError)

		return
	}

	resp := make([]model.AdminLimitsResponse, 0, len(limits))

	for _, tl := range limits {
		resp = append(resp, model.AdminLimitsResponse{
			Tier:            tl.Tier,
			SecretsLimit:    tl.SecretsLimit,
			RecipientsLimit: tl.RecipientsLimit,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpsertLimits handles PUT /api/v1/admin/limits/{tier}.
func (h *AdminHandler) UpsertLimits(w http.ResponseWriter, r *http.Request) {
	tier := r.PathValue("tier")

	var req model.AdminUpsertLimitsRequest
	if jsonErr := readJSON(r, &req); jsonErr != nil {
		writeError(w, errTypeValidaton, "Invalid JSON body", http.StatusBadRequest)

		return
	}

	if req.SecretsLimit <= 0 {
		writeError(w, errTypeValidaton, "secrets_limit must be positive", http.StatusBadRequest)

		return
	}

	if req.RecipientsLimit <= 0 {
		writeError(w, errTypeValidaton, "recipients_limit must be positive", http.StatusBadRequest)

		return
	}

	tl := &user.TierLimits{
		Tier:            tier,
		SecretsLimit:    req.SecretsLimit,
		RecipientsLimit: req.RecipientsLimit,
	}

	if upsertErr := h.repo.UpsertLimits(r.Context(), tl); upsertErr != nil {
		slog.Error("admin upsert limits", errKeyError, upsertErr)
		writeError(w, errTypeInternal, "Failed to upsert limits", http.StatusInternalServerError)

		return
	}

	writeJSON(w, http.StatusOK, model.AdminLimitsResponse{
		Tier:            tl.Tier,
		SecretsLimit:    tl.SecretsLimit,
		RecipientsLimit: tl.RecipientsLimit,
	})
}

// DeleteLimits handles DELETE /api/v1/admin/limits/{tier}.
func (h *AdminHandler) DeleteLimits(w http.ResponseWriter, r *http.Request) {
	tier := r.PathValue("tier")

	if tier == model.TierFree {
		writeError(
			w, errTypeValidaton,
			"Cannot delete the free tier; it is the default",
			http.StatusBadRequest,
		)

		return
	}

	if deleteErr := h.repo.DeleteLimits(r.Context(), tier); deleteErr != nil {
		if errors.Is(deleteErr, model.ErrNotFound) {
			writeError(w, errTypeNotFound, "Tier not found", http.StatusNotFound)
		} else {
			slog.Error("admin delete limits", errKeyError, deleteErr)
			writeError(w, errTypeInternal, "Failed to delete limits", http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// buildLimitsMap loads all tier limits and returns a map keyed by tier name.
func (h *AdminHandler) buildLimitsMap(r *http.Request) map[string]*user.TierLimits {
	limits, err := h.repo.ListLimits(r.Context())
	if err != nil {
		slog.Error("admin load limits for user list", errKeyError, err)

		return nil
	}

	m := make(map[string]*user.TierLimits, len(limits))
	for _, tl := range limits {
		m[tl.Tier] = tl
	}

	return m
}

// computeEffectiveLimit returns the effective secrets limit for a user.
// Priority: per-user override > tier limit from limits table > hardcoded fallback.
func computeEffectiveLimit(u *model.User, limitsMap map[string]*user.TierLimits) int {
	if u.SecretsLimitOverride != nil {
		return *u.SecretsLimitOverride
	}

	if tl, ok := limitsMap[u.Tier]; ok {
		return tl.SecretsLimit
	}

	if u.Tier == model.TierPro {
		return model.ProTierLimit
	}

	return model.FreeTierLimit
}

// computeEffectiveRecipientsLimit returns the effective recipients limit for a user.
// Priority: per-user override > tier limit from limits table > hardcoded fallback.
func computeEffectiveRecipientsLimit(u *model.User, limitsMap map[string]*user.TierLimits) int {
	if u.RecipientsLimitOverride != nil {
		return *u.RecipientsLimitOverride
	}

	if tl, ok := limitsMap[u.Tier]; ok {
		return tl.RecipientsLimit
	}

	if u.Tier == model.TierPro {
		return model.ProMaxRecipients
	}

	return model.FreeMaxRecipients
}

func parseUserListOptions(r *http.Request) []user.ListOption {
	var opts []user.ListOption

	if q := r.URL.Query().Get("q"); q != "" {
		opts = append(opts, user.WithSearch(q))
	}

	if tier := r.URL.Query().Get("tier"); tier != "" {
		opts = append(opts, user.WithTier(tier))
	}

	if sort := r.URL.Query().Get("sort"); sort != "" {
		order := r.URL.Query().Get("order")
		opts = append(opts, user.WithSort(sort, order))
	}

	page, perPage := parsePagination(r)
	opts = append(opts, user.WithPage(page, perPage))

	return opts
}

func parseSubscriptionListOptions(r *http.Request) []user.ListOption {
	var opts []user.ListOption

	if q := r.URL.Query().Get("q"); q != "" {
		opts = append(opts, user.WithSearch(q))
	}

	if status := r.URL.Query().Get("status"); status != "" {
		opts = append(opts, user.WithStatus(status))
	}

	if sort := r.URL.Query().Get("sort"); sort != "" {
		order := r.URL.Query().Get("order")
		opts = append(opts, user.WithSort(sort, order))
	}

	page, perPage := parsePagination(r)
	opts = append(opts, user.WithPage(page, perPage))

	return opts
}

func parsePagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = defaultPerPage

	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}

	if v, err := strconv.Atoi(r.URL.Query().Get("per_page")); err == nil && v > 0 {
		perPage = v
		if perPage > maxPerPage {
			perPage = maxPerPage
		}
	}

	return page, perPage
}
