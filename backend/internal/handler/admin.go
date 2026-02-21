package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

const (
	defaultPerPage = 20
	maxPerPage     = 100
	timeFormat     = time.RFC3339

	errKeyError      = "error"
	errTypeInternal  = "internal_error"
	errTypeValidaton = "validation_error"
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
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)
	mux.HandleFunc("GET /api/v1/admin/subscriptions", h.ListSubscriptions)
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)
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

	count, err := h.repo.CountUsers(r.Context(), opts...)
	if err != nil {
		slog.Error("admin count users", errKeyError, err)
		writeError(w, errTypeInternal, "Failed to count users", http.StatusInternalServerError)

		return
	}

	q := user.ApplyOptions(opts...)
	resp := model.AdminUsersListResponse{
		Users:   make([]model.AdminUserResponse, 0, len(users)),
		Total:   count,
		Page:    q.Page,
		PerPage: q.PerPage,
	}

	for _, u := range users {
		resp.Users = append(resp.Users, model.AdminUserResponse{
			ID:          u.ID,
			Email:       u.Email,
			Name:        u.Name,
			Provider:    u.Provider,
			Tier:        u.Tier,
			SecretsUsed: u.SecretsUsed,
			CreatedAt:   u.CreatedAt.Format(timeFormat),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateTier handles PATCH /api/v1/admin/users/{id}.
func (h *AdminHandler) UpdateTier(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, errTypeValidaton, "Invalid user ID", http.StatusBadRequest)

		return
	}

	var req model.AdminUpdateUserRequest
	if jsonErr := readJSON(r, &req); jsonErr != nil {
		writeError(w, errTypeValidaton, "Invalid JSON body", http.StatusBadRequest)

		return
	}

	if req.Tier == nil {
		writeError(w, errTypeValidaton, "Tier is required", http.StatusBadRequest)

		return
	}

	exists, err := h.repo.TierExists(r.Context(), *req.Tier)
	if err != nil {
		slog.Error("admin check tier exists", errKeyError, err)
		writeError(w, errTypeInternal, "Failed to validate tier", http.StatusInternalServerError)

		return
	}

	if !exists {
		writeError(w, errTypeValidaton, "Unknown tier", http.StatusBadRequest)

		return
	}

	if err := h.repo.UpdateTier(r.Context(), id, *req.Tier); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, "not_found", "User not found", http.StatusNotFound)
		} else {
			slog.Error("admin update tier", errKeyError, err)
			writeError(w, errTypeInternal, "Failed to update tier", http.StatusInternalServerError)
		}

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
			writeError(w, "not_found", "Subscription not found", http.StatusNotFound)
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
