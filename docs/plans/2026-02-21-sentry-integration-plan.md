# Sentry Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Integrate Sentry error tracking and performance monitoring into the SecretDrop Go backend.

**Architecture:** DSN-driven activation — if `SENTRY_DSN` is set, Sentry captures errors and traces; if empty, everything is no-op. A dedicated `internal/sentry/` package wraps initialization, and a `sloghandler` sub-package bridges slog errors to Sentry, following the existing Slack sloghandler pattern. `sentryhttp` middleware wraps the entire handler chain.

**Tech Stack:** `github.com/getsentry/sentry-go`, `github.com/getsentry/sentry-go/http`

---

### Task 1: Add sentry-go dependency

**Files:**
- Modify: `backend/go.mod`

**Step 1: Install sentry-go packages**

Run:
```bash
cd backend && go get github.com/getsentry/sentry-go github.com/getsentry/sentry-go/http
```

**Step 2: Tidy modules**

Run:
```bash
cd backend && go mod tidy
```

**Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore: add sentry-go dependency"
```

---

### Task 2: Add Sentry config fields

**Files:**
- Modify: `backend/internal/config/config.go`

**Step 1: Add struct fields and defaults**

Add two fields to the `Config` struct after `adminPassword`:

```go
sentryDSN             string
sentryTracesSampleRate float64
```

Add a default constant:

```go
defaultSentryTracesSampleRate = 0.1
```

**Step 2: Add functional options**

```go
// WithSentryDSN sets the Sentry DSN.
func WithSentryDSN(dsn string) Option {
	return func(c *Config) error {
		c.sentryDSN = dsn

		return nil
	}
}

// WithSentryTracesSampleRate sets the Sentry traces sample rate.
func WithSentryTracesSampleRate(rate float64) Option {
	return func(c *Config) error {
		if rate < 0 || rate > 1 {
			return errors.New("sentry traces sample rate must be between 0 and 1")
		}

		c.sentryTracesSampleRate = rate

		return nil
	}
}
```

**Step 3: Add getter methods**

```go
// SentryDSN returns the Sentry DSN. Empty means Sentry is disabled.
func (c *Config) SentryDSN() string { return c.sentryDSN }

// SentryTracesSampleRate returns the Sentry traces sample rate.
func (c *Config) SentryTracesSampleRate() float64 { return c.sentryTracesSampleRate }
```

**Step 4: Load from environment in Load()**

In the `Load()` function, after the `adminPassword` line, add:

```go
c.sentryDSN = os.Getenv("SENTRY_DSN")
c.sentryTracesSampleRate = defaultSentryTracesSampleRate

if v := os.Getenv("SENTRY_TRACES_SAMPLE_RATE"); v != "" {
	rate, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, fmt.Errorf("parse SENTRY_TRACES_SAMPLE_RATE: %w", err)
	}

	c.sentryTracesSampleRate = rate
}
```

Note: add `"strconv"` to imports.

Also set the default in the struct initializer:

```go
sentryTracesSampleRate: defaultSentryTracesSampleRate,
```

**Step 5: Build to verify**

Run:
```bash
cd backend && go build ./...
```
Expected: success

**Step 6: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat: add Sentry config fields (DSN, traces sample rate)"
```

---

### Task 3: Create `internal/sentry/` init package

**Files:**
- Create: `backend/internal/sentry/sentry.go`

**Step 1: Write the init wrapper**

```go
// Package sentry provides initialization for the Sentry error tracking SDK.
package sentry

import (
	"fmt"

	"github.com/getsentry/sentry-go"

	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
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
```

**Step 2: Build to verify**

Run:
```bash
cd backend && go build ./...
```
Expected: success

**Step 3: Commit**

```bash
git add backend/internal/sentry/sentry.go
git commit -m "feat: add sentry init package"
```

---

### Task 4: Create Sentry slog handler

**Files:**
- Create: `backend/internal/sentry/sloghandler/sloghandler.go`
- Create: `backend/internal/sentry/sloghandler/sloghandler_test.go`

**Step 1: Write the failing test**

```go
package sloghandler_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/sentry/sloghandler"
)

// captureHandler records the last slog.Record it handled.
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
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd backend && go test ./internal/sentry/sloghandler/... -v
```
Expected: compilation error (package doesn't exist yet)

**Step 3: Write the slog handler**

```go
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
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				record.Attrs(func(attr slog.Attr) bool {
					scope.SetExtra(attr.Key, attr.Value.String())

					return true
				})

				hub.CaptureMessage(record.Message)
			})
		} else {
			sentry.WithScope(func(scope *sentry.Scope) {
				record.Attrs(func(attr slog.Attr) bool {
					scope.SetExtra(attr.Key, attr.Value.String())

					return true
				})

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
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd backend && go test ./internal/sentry/sloghandler/... -v
```
Expected: PASS

**Step 5: Lint**

Run:
```bash
cd backend && golangci-lint run ./internal/sentry/...
```
Expected: no issues

**Step 6: Commit**

```bash
git add backend/internal/sentry/sloghandler/
git commit -m "feat: add Sentry slog handler for error forwarding"
```

---

### Task 5: Integrate Sentry into main.go

**Files:**
- Modify: `backend/cmd/secretdrop/main.go`

**Step 1: Add imports**

Add to imports:

```go
"github.com/getsentry/sentry-go"
sentryhttp "github.com/getsentry/sentry-go/http"

sentrypkg "github.com/bilusteknoloji/secretdrop/internal/sentry"
sentryslog "github.com/bilusteknoloji/secretdrop/internal/sentry/sloghandler"
```

**Step 2: Add Sentry init after config load**

After `cfg, err := config.Load()` and the Slack notifier setup, before the logger setup, add:

```go
// Sentry error tracking (enabled when SENTRY_DSN is set)
if cfg.SentryDSN() != "" {
	if err := sentrypkg.Init(cfg.SentryDSN(), cfg.Env(), cfg.SentryTracesSampleRate()); err != nil {
		return fmt.Errorf("init sentry: %w", err)
	}

	defer sentry.Flush(2 * time.Second)

	slog.Info("sentry enabled", "environment", cfg.Env())
}
```

**Step 3: Add Sentry slog handler to logger chain**

Modify the logger setup to include the Sentry slog handler. Change from:

```go
baseHandler := slog.NewJSONHandler(os.Stdout, nil)
logger := slog.New(sloghandler.New(baseHandler, errorNotifier))
```

To:

```go
var logHandler slog.Handler = slog.NewJSONHandler(os.Stdout, nil)
logHandler = sloghandler.New(logHandler, errorNotifier)
if cfg.SentryDSN() != "" {
	logHandler = sentryslog.New(logHandler)
}
logger := slog.New(logHandler)
```

**Step 4: Add sentryhttp middleware as outermost wrapper**

After the middleware chain, wrap with sentryhttp. Change from:

```go
var chain http.Handler = mux
chain = middleware.RequireJSON(chain)
chain = middleware.OptionalAuthenticate(authSvc)(chain)
chain = middleware.Logging(chain)
chain = middleware.RequestID(chain)
```

To:

```go
var chain http.Handler = mux
chain = middleware.RequireJSON(chain)
chain = middleware.OptionalAuthenticate(authSvc)(chain)
chain = middleware.Logging(chain)
chain = middleware.RequestID(chain)
if cfg.SentryDSN() != "" {
	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic: true,
	})
	chain = sentryHandler.Handle(chain)
}
```

**Step 5: Add user context to auth middleware**

Modify `backend/internal/middleware/auth.go` — in `OptionalAuthenticate`, after setting user context, add Sentry user scope:

```go
import "github.com/getsentry/sentry-go"
```

After `ctx := context.WithValue(r.Context(), userContextKey, claims)` in `OptionalAuthenticate`, add:

```go
if hub := sentry.GetHubFromContext(ctx); hub != nil {
	hub.Scope().SetUser(sentry.User{ID: fmt.Sprintf("%d", claims.UserID)})
}
```

Do the same in `Authenticate` after setting user context.

Note: add `"fmt"` is already imported, and add `"github.com/getsentry/sentry-go"` to imports.

**Step 6: Build to verify**

Run:
```bash
cd backend && go build ./...
```
Expected: success

**Step 7: Lint**

Run:
```bash
cd backend && golangci-lint run ./...
```
Expected: no issues

**Step 8: Run all tests**

Run:
```bash
cd backend && go test -race ./...
```
Expected: all pass

**Step 9: Commit**

```bash
git add backend/cmd/secretdrop/main.go backend/internal/middleware/auth.go
git commit -m "feat: integrate Sentry into server startup and middleware chain"
```

---

### Task 6: Update documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/openapi.yaml` (if env vars are documented there)

**Step 1: Add env vars to CLAUDE.md table**

Add to the Environment Variables table:

```
| `SENTRY_DSN` | No | — |
| `SENTRY_TRACES_SAMPLE_RATE` | No | `0.1` |
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add Sentry environment variables to CLAUDE.md"
```

---

### Task 7: Verify end-to-end integration

**Step 1: Run with SENTRY_DSN to verify Sentry receives events**

Run:
```bash
cd backend && GOLANG_ENV=development SENTRY_DSN="<your-dsn>" go run ./cmd/secretdrop/
```

Verify in Sentry dashboard that:
- A test connection is established
- The application starts without errors
- Log "sentry enabled" appears in stdout

**Step 2: Run without SENTRY_DSN to verify no-op behavior**

Run:
```bash
cd backend && GOLANG_ENV=development go run ./cmd/secretdrop/
```

Verify:
- No "sentry enabled" log message
- Application starts normally
- No Sentry-related errors

**Step 3: Remove SENTRY_DSN from local env**

Since DSN is unreachable from Turkey, remove it from local development environment. It will only be set in production deployment.
