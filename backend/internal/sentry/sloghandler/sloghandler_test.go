package sloghandler_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/sentry/sloghandler"
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
