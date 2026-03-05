// Package sloghandler provides a custom slog.Handler that sends Slack
// notifications for error-level log records while delegating all calls
// to an inner handler.
package sloghandler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bilustek/secretdrop/internal/slack"
)

const defaultDedup = time.Minute

// compile-time interface check.
var _ slog.Handler = (*Handler)(nil)

// shared holds mutable state that must be shared across WithAttrs/WithGroup clones.
type shared struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// Handler is a slog.Handler that delegates to an inner handler and sends
// Slack notifications for error-level (and above) log records.
type Handler struct {
	inner    slog.Handler
	notifier slack.Notifier
	dedup    time.Duration
	shared   *shared
}

// Option configures a Handler value.
type Option func(*Handler)

// WithDedup sets the deduplication window. Log messages with the same text
// within this window are sent to Slack only once.
func WithDedup(d time.Duration) Option {
	return func(h *Handler) {
		h.dedup = d
	}
}

// New creates a new slog handler that wraps inner and sends Slack
// notifications via notifier for error-level log records.
func New(inner slog.Handler, notifier slack.Notifier, opts ...Option) *Handler {
	h := &Handler{
		inner:    inner,
		notifier: notifier,
		dedup:    defaultDedup,
		shared: &shared{
			seen: make(map[string]time.Time),
		},
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Enabled reports whether the inner handler handles records at the given level.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle delegates the record to the inner handler. For records at
// slog.LevelError or above, it also fires a non-blocking Slack notification
// subject to deduplication.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	err := h.inner.Handle(ctx, record)

	if record.Level >= slog.LevelError {
		msg := record.Message

		if h.shouldNotify(msg) {
			event := buildEvent(record)

			go func() {
				_ = h.notifier.Notify(ctx, event)
			}()
		}
	}

	if err != nil {
		return fmt.Errorf("inner handler: %w", err)
	}

	return nil
}

// WithAttrs returns a new Handler whose inner handler has the given attrs,
// sharing the same notifier, dedup window, and dedup state.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		inner:    h.inner.WithAttrs(attrs),
		notifier: h.notifier,
		dedup:    h.dedup,
		shared:   h.shared,
	}
}

// WithGroup returns a new Handler whose inner handler uses the given group,
// sharing the same notifier, dedup window, and dedup state.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		inner:    h.inner.WithGroup(name),
		notifier: h.notifier,
		dedup:    h.dedup,
		shared:   h.shared,
	}
}

// shouldNotify checks deduplication and returns true if the message should
// be sent to Slack. It also lazily cleans up expired entries.
func (h *Handler) shouldNotify(msg string) bool {
	now := time.Now()

	h.shared.mu.Lock()
	defer h.shared.mu.Unlock()

	// Lazy cleanup of expired entries.
	for k, t := range h.shared.seen {
		if now.Sub(t) >= h.dedup {
			delete(h.shared.seen, k)
		}
	}

	if lastSeen, ok := h.shared.seen[msg]; ok {
		if now.Sub(lastSeen) < h.dedup {
			return false
		}
	}

	h.shared.seen[msg] = now

	return true
}

// buildEvent converts a slog.Record into a slack.Event suitable for
// error notifications.
func buildEvent(record slog.Record) slack.Event {
	fields := make(map[string]string)

	record.Attrs(func(attr slog.Attr) bool {
		fields[attr.Key] = attr.Value.String()

		return true
	})

	return slack.Event{
		Type:    slack.EventError,
		Message: record.Message,
		Fields:  fields,
	}
}
