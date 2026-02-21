package handler

import (
	"log/slog"
	"net/http"

	"github.com/getsentry/sentry-go"
)

// NewDebugSentryHandler returns a handler that sends a test event to Sentry.
// Use GET /debug/sentry to verify Sentry integration is working.
func NewDebugSentryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// 1. Direct message capture
		eventID := sentry.CaptureMessage("sentry debug: test message from secretdrop")

		// 2. Trigger via slog (tests the slog→Sentry bridge)
		slog.Error("sentry debug: test error via slog bridge", "source", "debug_endpoint")

		id := ""
		if eventID != nil {
			id = string(*eventID)
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"message":  "sentry test event sent",
			"event_id": id,
		})
	}
}
