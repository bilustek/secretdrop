// Package sloghandler provides a slog.Handler that forwards error-level
// log records to Sentry while delegating all records to an inner handler.
package sloghandler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/getsentry/sentry-go"
)

// compile-time interface check.
var _ slog.Handler = (*Handler)(nil)

// Handler is a slog.Handler that delegates to an inner handler and sends
// error-level (and above) log records to Sentry via CaptureMessage.
type Handler struct {
	inner slog.Handler
}

// New creates a new slog handler that wraps inner and forwards error-level
// records to Sentry.
func New(inner slog.Handler) *Handler {
	return &Handler{inner: inner}
}

// Enabled reports whether the inner handler handles records at the given level.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle delegates the record to the inner handler. For records at
// slog.LevelError or above, it also captures the message in Sentry
// with structured attributes as extra context.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	err := h.inner.Handle(ctx, record)

	if record.Level >= slog.LevelError {
		attrs := make(sentry.Context)
		record.Attrs(func(attr slog.Attr) bool {
			attrs[attr.Key] = attr.Value.String()

			return true
		})

		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.SetContext("slog", attrs)
				hub.CaptureMessage(record.Message)
			})
		} else {
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetContext("slog", attrs)
				sentry.CaptureMessage(record.Message)
			})
		}
	}

	if err != nil {
		return fmt.Errorf("inner handler: %w", err)
	}

	return nil
}

// WithAttrs returns a new Handler whose inner handler has the given attrs.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup returns a new Handler whose inner handler uses the given group.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{inner: h.inner.WithGroup(name)}
}
