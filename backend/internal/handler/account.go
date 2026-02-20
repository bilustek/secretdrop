package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

// SubscriptionCanceller cancels a Stripe subscription.
type SubscriptionCanceller interface {
	CancelSubscription(ctx context.Context, stripeSubID string) error
}

// NewDeleteAccountHandler returns a handler for DELETE /api/v1/me.
func NewDeleteAccountHandler(userRepo user.Repository, canceller SubscriptionCanceller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)

			return
		}

		// Cancel Stripe subscription if exists.
		if canceller != nil {
			sub, err := userRepo.FindSubscriptionByUserID(r.Context(), claims.UserID)
			if err == nil && sub.Status == model.SubscriptionActive {
				if cancelErr := canceller.CancelSubscription(r.Context(), sub.StripeSubscriptionID); cancelErr != nil {
					slog.Error("cancel stripe subscription during account deletion", "error", cancelErr)
				}
			} else if err != nil && !errors.Is(err, model.ErrNotFound) {
				slog.Error("find subscription for deletion", "error", err)
			}
		}

		// Delete user and subscription records from DB.
		if err := userRepo.DeleteUser(r.Context(), claims.UserID); err != nil {
			if errors.Is(err, model.ErrNotFound) {
				writeError(w, "not_found", "User not found", http.StatusNotFound)
			} else {
				slog.Error("delete user account", "error", err)
				writeError(w, "internal_error", "Failed to delete account", http.StatusInternalServerError)
			}

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
