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
	"github.com/bilusteknoloji/secretdrop/internal/handler"
	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/repository"
	"github.com/bilusteknoloji/secretdrop/internal/service"
)

//go:embed docs/openapi.yaml
var openAPISpec []byte

const (
	readHeaderTimeout   = 5 * time.Second
	shutdownGracePeriod = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	repo, err := repository.NewSQLite(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	sender := email.NewResendSender(cfg.ResendAPIKey, cfg.FromEmail)
	svc := service.NewSecretService(repo, sender, cfg.BaseURL, cfg.FromEmail, cfg.SecretExpiry)
	h := handler.NewSecretHandler(svc)

	mux := http.NewServeMux()
	h.Register(mux)

	handler.SetOpenAPISpec(openAPISpec)
	handler.RegisterDocs(mux)

	rl := middleware.NewRateLimiter()

	var chain http.Handler = mux
	chain = middleware.RequireJSON(chain)
	chain = rl.Limit(chain)
	chain = middleware.Logging(chain)
	chain = middleware.RequestID(chain)

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           chain,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	cleanupWorker := cleanup.NewWorker(repo, cfg.CleanupInterval)
	cleanupWorker.Start()

	go func() {
		slog.Info("server starting", "addr", addr)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
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
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
