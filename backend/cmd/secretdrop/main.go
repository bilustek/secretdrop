package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bilusteknoloji/secretdrop/docs"
	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
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

const (
	readHeaderTimeout   = 5 * time.Second
	shutdownGracePeriod = 10 * time.Second
)

func main() {
	if err := Run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func Run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

	var sender email.Sender
	if cfg.IsDev() {
		sender = console.New()
		slog.Info("development mode: emails will be logged to console")
	} else {
		sender, err = resend.New(cfg.ResendAPIKey(), resend.WithFrom(cfg.FromEmail()))
		if err != nil {
			return fmt.Errorf("create email sender: %w", err)
		}
	}

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL(cfg.BaseURL()),
		service.WithFromEmail(cfg.FromEmail()),
		service.WithExpiry(cfg.SecretExpiry()),
	)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	h := handler.NewSecretHandler(svc)

	mux := http.NewServeMux()
	h.Register(mux)

	handler.SetOpenAPISpec(docs.OpenAPISpec)
	handler.RegisterDocs(mux)

	rl, err := middleware.NewRateLimiter()
	if err != nil {
		return fmt.Errorf("create rate limiter: %w", err)
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
