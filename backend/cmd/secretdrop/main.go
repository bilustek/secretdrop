package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"

	"github.com/bilusteknoloji/secretdrop/docs"
	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/bilusteknoloji/secretdrop/internal/billing"
	"github.com/bilusteknoloji/secretdrop/internal/cleanup"
	"github.com/bilusteknoloji/secretdrop/internal/config"
	"github.com/bilusteknoloji/secretdrop/internal/email"
	"github.com/bilusteknoloji/secretdrop/internal/email/console"
	"github.com/bilusteknoloji/secretdrop/internal/email/resend"
	"github.com/bilusteknoloji/secretdrop/internal/handler"
	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/repository/sqlite"
	sentrypkg "github.com/bilusteknoloji/secretdrop/internal/sentry"
	sentryslog "github.com/bilusteknoloji/secretdrop/internal/sentry/sloghandler"
	"github.com/bilusteknoloji/secretdrop/internal/service"
	slackpkg "github.com/bilusteknoloji/secretdrop/internal/slack"
	slackconsole "github.com/bilusteknoloji/secretdrop/internal/slack/console"
	slacknoop "github.com/bilusteknoloji/secretdrop/internal/slack/noop"
	"github.com/bilusteknoloji/secretdrop/internal/slack/sloghandler"
	slackwebhook "github.com/bilusteknoloji/secretdrop/internal/slack/webhook"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

const (
	readHeaderTimeout   = 5 * time.Second
	shutdownGracePeriod = 10 * time.Second
	dbDirPermissions    = 0o750
)

func main() {
	if err := Run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func Run() error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Slack notifiers
	var subscriptionNotifier slackpkg.Notifier
	var errorNotifier slackpkg.Notifier

	if cfg.IsDev() {
		subscriptionNotifier = slackconsole.New()
		errorNotifier = slackconsole.New()
	} else {
		subscriptionNotifier = selectNotifier(cfg.SlackWebhookSubscriptions())
		errorNotifier = selectNotifier(cfg.SlackWebhookNotifications())
	}

	// Sentry error tracking (enabled when SENTRY_DSN is set)
	if cfg.SentryDSN() != "" {
		if initErr := sentrypkg.Init(cfg.SentryDSN(), cfg.Env(), cfg.SentryTracesSampleRate()); initErr != nil {
			return fmt.Errorf("init sentry: %w", initErr)
		}

		defer sentry.Flush(2 * time.Second)

		slog.Info("sentry enabled", "environment", cfg.Env())
	}

	// Logger with Slack error handler (and optional Sentry bridge)
	var logHandler slog.Handler = slog.NewJSONHandler(os.Stdout, nil)
	logHandler = sloghandler.New(logHandler, errorNotifier)

	if cfg.SentryDSN() != "" {
		logHandler = sentryslog.New(logHandler)
	}

	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	// Ensure database directory exists.
	if dir := dbDir(cfg.DatabaseURL()); dir != "." && dir != "" {
		if mkErr := os.MkdirAll(dir, dbDirPermissions); mkErr != nil {
			return fmt.Errorf("create database directory: %w", mkErr)
		}
	}

	repo, err := sqlite.New(cfg.DatabaseURL())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if closeErr := repo.Close(); closeErr != nil {
			slog.Error("close database", "error", closeErr)
		}
	}()

	// User repository (shares the same SQLite database file)
	userRepo, err := usersqlite.New(cfg.DatabaseURL())
	if err != nil {
		return fmt.Errorf("open user database: %w", err)
	}
	defer func() {
		if closeErr := userRepo.Close(); closeErr != nil {
			slog.Error("close user database", "error", closeErr)
		}
	}()

	// Auth service (JWT)
	jwtSecret := cfg.JWTSecret()
	if cfg.IsDev() && jwtSecret == "" {
		jwtSecret = "dev-jwt-secret-do-not-use-in-production" //nolint:gosec // safe default for dev only
		slog.Warn("using default JWT secret for development mode")
	}

	authSvc, err := auth.New(jwtSecret,
		auth.WithGoogleClientID(cfg.GoogleClientID()),
		auth.WithFrontendBaseURL(cfg.FrontendBaseURL()),
	)
	if err != nil {
		return fmt.Errorf("create auth service: %w", err)
	}

	var sender email.Sender
	if cfg.IsDev() {
		sender = console.New(console.WithFrom(cfg.FromEmail()))
		slog.Info("development mode: emails will be printed to stderr")
	} else {
		sender, err = resend.New(
			cfg.ResendAPIKey(),
			resend.WithFrom(cfg.FromEmail()),
			resend.WithReplyTo(cfg.ReplyToEmail()),
		)
		if err != nil {
			return fmt.Errorf("create email sender: %w", err)
		}
	}

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL(cfg.FrontendBaseURL()),
		service.WithFromEmail(cfg.FromEmail()),
		service.WithExpiry(cfg.SecretExpiry()),
		service.WithUserRepo(userRepo),
	)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	h := handler.NewSecretHandler(svc, userRepo)

	mux := http.NewServeMux()
	h.Register(mux)

	// Contact form (public — no auth required)
	mux.HandleFunc("POST /api/v1/contact", handler.NewContactHandler(sender))

	handler.SetOpenAPISpec(docs.OpenAPISpec)

	// OAuth routes (public — no auth required)
	googleCfg := auth.GoogleConfig(
		cfg.GoogleClientID(),
		cfg.GoogleClientSecret(),
		cfg.APIBaseURL()+"/auth/google/callback",
	)
	githubCfg := auth.GithubConfig(
		cfg.GithubClientID(),
		cfg.GithubClientSecret(),
		cfg.APIBaseURL()+"/auth/github/callback",
	)

	mux.HandleFunc("GET /auth/google", authSvc.HandleGoogleLogin(googleCfg))
	mux.HandleFunc("GET /auth/google/callback", authSvc.HandleGoogleCallback(googleCfg, userRepo))
	mux.HandleFunc("GET /auth/github", authSvc.HandleGithubLogin(githubCfg))
	mux.HandleFunc("GET /auth/github/callback", authSvc.HandleGithubCallback(githubCfg, userRepo))
	mux.HandleFunc("POST /auth/token", authSvc.HandleTokenExchange(userRepo))

	// Billing routes (conditional — only in non-dev mode)
	billingSvc, billingErr := setupBilling(mux, cfg, userRepo, subscriptionNotifier)
	if billingErr != nil {
		return billingErr
	}

	// Delete account route (uses billing service for subscription cancellation)
	var canceller handler.SubscriptionCanceller
	if billingSvc != nil {
		canceller = billingSvc
	}

	mux.HandleFunc("DELETE /api/v1/me", handler.NewDeleteAccountHandler(userRepo, canceller, subscriptionNotifier))

	// Admin routes (conditional — only when ADMIN_USERNAME and ADMIN_PASSWORD are set)
	if cfg.AdminUsername() != "" && cfg.AdminPassword() != "" {
		adminAuth := middleware.BasicAuth(cfg.AdminUsername(), cfg.AdminPassword())
		adminHandler := handler.NewAdminHandler(userRepo, billingSvc)
		adminMux := http.NewServeMux()
		adminHandler.Register(adminMux)

		mux.Handle("/api/v1/admin/", adminAuth(adminMux))

		handler.RegisterDocs(mux, adminAuth)

		slog.Info("admin routes enabled")
	} else {
		handler.RegisterDocs(mux, nil)

		slog.Info("admin routes disabled (ADMIN_USERNAME or ADMIN_PASSWORD not set)")
	}

	var chain http.Handler = mux
	chain = middleware.RequireJSON(chain)
	chain = middleware.OptionalAuthenticate(authSvc)(chain)
	chain = middleware.Logging(chain)
	chain = middleware.RequestID(chain)

	chain = middleware.CORS(cfg.FrontendBaseURL())(chain)

	if cfg.SentryDSN() != "" {
		sentryHandler := sentryhttp.New(sentryhttp.Options{
			Repanic: true,
		})
		chain = sentryHandler.Handle(chain)
	}

	addr := ":" + cfg.Port()
	srv := &http.Server{
		Addr:              addr,
		Handler:           chain,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	cleanupWorker, err := cleanup.New(repo, cleanup.WithInterval(cfg.CleanupInterval()))
	if err != nil {
		return fmt.Errorf("create cleanup worker: %w", err)
	}

	cleanupWorker.Start()

	errCh := make(chan error, 1)

	go func() {
		slog.Info("server starting", "addr", addr, "version", appinfo.Version)

		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("server failed: %w", listenErr)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("shutting down server", "signal", sig.String())
	case err := <-errCh:
		return err
	}

	cleanupWorker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	slog.Info("server stopped gracefully")

	return nil
}

// setupBilling creates the billing service and registers Stripe routes.
// In development mode the routes are skipped entirely since Stripe
// credentials are not available, and nil is returned for the service.
func setupBilling(
	mux *http.ServeMux,
	cfg *config.Config,
	userRepo *usersqlite.Repository,
	notifier slackpkg.Notifier,
) (*billing.Service, error) {
	if cfg.StripeSecretKey() == "" || cfg.StripeWebhookSecret() == "" || cfg.StripePriceID() == "" {
		slog.Info("billing routes disabled (STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET, or STRIPE_PRICE_ID not set)")

		return nil, nil
	}

	billingSvc, err := billing.New(
		cfg.StripeSecretKey(),
		cfg.StripeWebhookSecret(),
		cfg.StripePriceID(),
		userRepo,
		billing.WithSuccessURL(cfg.FrontendBaseURL()+"/billing/success"),
		billing.WithCancelURL(cfg.FrontendBaseURL()+"/billing/cancel"),
		billing.WithPortalReturnURL(cfg.FrontendBaseURL()+"/dashboard"),
		billing.WithNotifier(notifier),
	)
	if err != nil {
		return nil, fmt.Errorf("create billing service: %w", err)
	}

	// Webhook is public (Stripe signature verification)
	mux.HandleFunc("POST /billing/webhook", billingSvc.HandleWebhook())

	// Checkout and portal require auth (enforced at handler level)
	mux.HandleFunc("POST /billing/checkout", billingSvc.HandleCheckout())
	mux.HandleFunc("POST /billing/portal", billingSvc.HandlePortal())

	return billingSvc, nil
}

func selectNotifier(webhookURL string) slackpkg.Notifier {
	if webhookURL == "" {
		return slacknoop.New()
	}

	n, err := slackwebhook.New(webhookURL)
	if err != nil {
		slog.Warn("invalid slack webhook URL, using noop notifier", "error", err)

		return slacknoop.New()
	}

	return n
}

// dbDir extracts the directory from a SQLite DSN like "file:db/secretdrop.db?_journal_mode=WAL".
func dbDir(dsn string) string {
	path := strings.TrimPrefix(dsn, "file:")
	if i := strings.Index(path, "?"); i != -1 {
		path = path[:i]
	}

	return filepath.Dir(path)
}
