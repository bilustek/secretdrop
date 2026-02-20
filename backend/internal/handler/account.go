package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/slack"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

// SubscriptionCanceller cancels a Stripe subscription.
type SubscriptionCanceller interface {
	CancelSubscription(ctx context.Context, stripeSubID string) error
}

// NewDeleteAccountHandler returns a handler for DELETE /api/v1/me.
func NewDeleteAccountHandler(
	userRepo user.Repository,
	canceller SubscriptionCanceller,
	notifier slack.Notifier,
) http.HandlerFunc {
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

		if notifier != nil {
			go func() {
				ev := slack.Event{
					Type:    slack.EventUserDeleted,
					Message: "User account deleted",
					Fields: map[string]string{
						"user_id": strconv.FormatInt(claims.UserID, 10),
						"email":   claims.Email,
					},
				}
				if err := notifier.Notify(context.Background(), ev); err != nil {
					slog.Warn("send slack notification", "error", err)
				}
			}()
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
