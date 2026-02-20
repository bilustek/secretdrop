package main

import (
	"context"
	_ "embed"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/cleanup"
	"github.com/bilusteknoloji/secretdrop/internal/config"
	"github.com/bilusteknoloji/secretdrop/internal/email"
	"github.com/bilusteknoloji/secretdrop/internal/email/console"
	"github.com/bilusteknoloji/secretdrop/internal/email/resend"
	"github.com/bilusteknoloji/secretdrop/internal/handler"
	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/repository/sqlite"
	"github.com/bilusteknoloji/secretdrop/internal/service"
)

//go:embed docs/openapi.yaml
var openAPISpec []byte

const (
	readHeaderTimeout   = 5 * time.Second
	shutdownGracePeriod = 10 * time.Second
	logKeyError         = "error"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", logKeyError, err)
		os.Exit(1)
	}

	repo, err := sqlite.New(cfg.DatabaseURL())
	if err != nil {
		slog.Error("open database", logKeyError, err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := repo.Close(); closeErr != nil {
			slog.Error("close database", logKeyError, closeErr)
		}
	}()

	var sender email.Sender
	if cfg.IsDev() {
		sender = console.New()
		slog.Info("development mode: emails will be logged to console")
	} else {
		sender, err = resend.New(cfg.ResendAPIKey(), resend.WithFrom(cfg.FromEmail()))
		if err != nil {
			slog.Error("create email sender", logKeyError, err)
			os.Exit(1)
		}
	}

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL(cfg.BaseURL()),
		service.WithFromEmail(cfg.FromEmail()),
		service.WithExpiry(cfg.SecretExpiry()),
	)
	if err != nil {
		slog.Error("create service", logKeyError, err)
		os.Exit(1)
	}

	h := handler.NewSecretHandler(svc)

	mux := http.NewServeMux()
	h.Register(mux)

	handler.SetOpenAPISpec(openAPISpec)
	handler.RegisterDocs(mux)

	rl, err := middleware.NewRateLimiter()
	if err != nil {
		slog.Error("create rate limiter", logKeyError, err)
		os.Exit(1)
	}

	var chain http.Handler = mux
	chain = middleware.RequireJSON(chain)
	chain = rl.Limit(chain)
	chain = middleware.Logging(chain)
	chain = middleware.RequestID(chain)

	addr := ":" + cfg.Port()
	srv := &http.Server{
		Addr:              addr,
		Handler:           chain,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	cleanupWorker, err := cleanup.New(repo, cleanup.WithInterval(cfg.CleanupInterval()))
	if err != nil {
		slog.Error("create cleanup worker", logKeyError, err)
		os.Exit(1)
	}

	cleanupWorker.Start()

	go func() {
		slog.Info("server starting", "addr", addr)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", logKeyError, err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutting down server", "signal", sig.String())

	cleanupWorker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", logKeyError, err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
