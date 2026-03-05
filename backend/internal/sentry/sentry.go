// Package sentry provides initialization for the Sentry error tracking SDK.
package sentry

import (
	"fmt"

	"github.com/getsentry/sentry-go"

	"github.com/bilustek/secretdrop/internal/appinfo"
)

// Init initializes the Sentry SDK with the given DSN and options.
// Returns an error if initialization fails.
func Init(dsn, environment string, tracesSampleRate float64) error {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          appinfo.Version,
		TracesSampleRate: tracesSampleRate,
		EnableTracing:    true,
		EnableLogs:       true,
	}); err != nil {
		return fmt.Errorf("sentry init: %w", err)
	}

	return nil
}
