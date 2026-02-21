package sentry_test

import (
	"testing"

	sentrypkg "github.com/bilusteknoloji/secretdrop/internal/sentry"
)

func TestInitWithInvalidDSN(t *testing.T) {
	t.Parallel()

	err := sentrypkg.Init("https://invalid@sentry.io/123", "test", 1.0)
	if err != nil {
		t.Fatalf("Init with valid-format DSN should not fail, got: %v", err)
	}
}

func TestInitWithEmptyDSN(t *testing.T) {
	t.Parallel()

	err := sentrypkg.Init("", "test", 1.0)
	if err != nil {
		t.Fatalf("Init with empty DSN should not fail (no-op), got: %v", err)
	}
}
