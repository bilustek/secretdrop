package sloghandler_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/bilustek/secretdrop/internal/sentry/sloghandler"
)

// captureHandler records the slog.Records it handled.
type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)

	return nil
}
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

// errorHandler always returns an error from Handle.
type errorHandler struct{}

func (h *errorHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *errorHandler) Handle(_ context.Context, _ slog.Record) error {
	return errors.New("inner error")
}
func (h *errorHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *errorHandler) WithGroup(_ string) slog.Handler      { return h }

func TestErrorLevelDelegatesToInner(t *testing.T) {
	t.Parallel()

	inner := &captureHandler{}
	handler := sloghandler.New(inner)

	logger := slog.New(handler)
	logger.Error("test error", "key", "value")

	if len(inner.records) != 1 {
		t.Fatalf("inner handler records = %d; want 1", len(inner.records))
	}

	if inner.records[0].Message != "test error" {
		t.Errorf("message = %q; want %q", inner.records[0].Message, "test error")
	}
}

func TestInfoLevelDelegatesToInner(t *testing.T) {
	t.Parallel()

	inner := &captureHandler{}
	handler := sloghandler.New(inner)

	logger := slog.New(handler)
	logger.Info("test info")

	if len(inner.records) != 1 {
		t.Fatalf("inner handler records = %d; want 1", len(inner.records))
	}
}

func TestWithAttrsReturnsNewHandler(t *testing.T) {
	t.Parallel()

	inner := &captureHandler{}
	handler := sloghandler.New(inner)

	newHandler := handler.WithAttrs([]slog.Attr{slog.String("foo", "bar")})
	if newHandler == nil {
		t.Fatal("WithAttrs returned nil")
	}
}

func TestWithGroupReturnsNewHandler(t *testing.T) {
	t.Parallel()

	inner := &captureHandler{}
	handler := sloghandler.New(inner)

	newHandler := handler.WithGroup("test")
	if newHandler == nil {
		t.Fatal("WithGroup returned nil")
	}
}

func TestErrorLevelWithSentryHub(t *testing.T) {
	t.Parallel()

	inner := &captureHandler{}
	handler := sloghandler.New(inner)

	hub := sentry.NewHub(nil, sentry.NewScope())
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	record := slog.NewRecord(time.Now(), slog.LevelError, "hub error", 0)
	_ = handler.Handle(ctx, record)

	if len(inner.records) != 1 {
		t.Fatalf("inner handler records = %d; want 1", len(inner.records))
	}
}

func TestInnerHandlerErrorIsPropagated(t *testing.T) {
	t.Parallel()

	inner := &errorHandler{}
	handler := sloghandler.New(inner)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	err := handler.Handle(context.Background(), record)

	if err == nil {
		t.Fatal("expected error from inner handler, got nil")
	}
}

func TestErrorWithInnerHandlerErrorIsPropagated(t *testing.T) {
	t.Parallel()

	inner := &errorHandler{}
	handler := sloghandler.New(inner)

	record := slog.NewRecord(time.Now(), slog.LevelError, "error msg", 0)
	err := handler.Handle(context.Background(), record)

	if err == nil {
		t.Fatal("expected error from inner handler, got nil")
	}
}
