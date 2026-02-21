# Sentry Integration Design

## Overview

Integrate Sentry error tracking and performance monitoring into the SecretDrop
Go backend. Captures unhandled panics, structured error logs, and request
tracing with minimal changes to existing architecture.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | Error capture + request tracing | Full observability |
| User context | User ID only | Privacy-friendly |
| slog bridge | Custom slog handler | Matches existing Slack sloghandler pattern |
| Package structure | Dedicated `internal/sentry/` | Interface segregation pattern |
| Activation | DSN presence | DSN set = active, empty = no-op |

## Architecture

### Package Structure

```
internal/sentry/
├── sentry.go              # Init() — wraps sentry.Init with app defaults
└── sloghandler/
    └── handler.go          # slog.Handler that forwards errors to Sentry
```

### Config Additions

Two new environment variables in `internal/config/`:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `SENTRY_DSN` | string | `""` | Sentry DSN. Empty = Sentry disabled |
| `SENTRY_TRACES_SAMPLE_RATE` | float64 | `0.1` | Fraction of requests traced (0.0-1.0) |

Exposed via `cfg.SentryDSN()` and `cfg.SentryTracesSampleRate()` getters.
Functional options pattern with `WithSentryDSN()` and
`WithSentryTracesSampleRate()`.

### Init Function

`internal/sentry/sentry.go` provides `Init()`:

```go
func Init(dsn, environment string, tracesSampleRate float64) error {
    return sentry.Init(sentry.ClientOptions{
        Dsn:              dsn,
        Environment:      environment,
        TracesSampleRate: tracesSampleRate,
        EnableTracing:    true,
        EnableLogs:       true,
    })
}
```

Called in `main.go` only when `cfg.SentryDSN() != ""`.

### slog Handler

`internal/sentry/sloghandler/handler.go` implements `slog.Handler`:

- Wraps an inner handler (chain pattern, same as Slack sloghandler)
- On `slog.LevelError` and above: calls `sentry.CaptureMessage()`
- Passes all records to inner handler unchanged
- Extracts structured attributes as Sentry tags/context

### Middleware Chain

`sentryhttp.New()` wraps the entire handler chain as the outermost middleware:

```
sentryhttp → RequestID → Logging → OptionalAuthenticate → RequireJSON → mux
```

This ensures:
- Panic recovery covers all handlers
- Request spans capture full request lifecycle
- Request context flows through the entire chain

### User Context in Auth Middleware

After successful JWT validation in `OptionalAuthenticate`, add user ID to
Sentry scope:

```go
if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
    hub.Scope().SetUser(sentry.User{ID: claims.UserID})
}
```

Only user ID is sent. No email or PII.

### Graceful Shutdown

Add `sentry.Flush(2 * time.Second)` to the shutdown sequence in `main.go`,
before server shutdown, to ensure buffered events are transmitted.

## Environment Variables Summary

Add to CLAUDE.md table:

| Variable | Required | Default |
|----------|----------|---------|
| `SENTRY_DSN` | No | `""` |
| `SENTRY_TRACES_SAMPLE_RATE` | No | `0.1` |

## Testing Strategy

1. Initial setup: set `SENTRY_DSN` in development to verify integration works
2. After verification: remove DSN from development, keep only in production
3. Unit tests: verify slog handler forwards error-level records
4. Integration: no Sentry dependency in tests (DSN empty = no-op)
