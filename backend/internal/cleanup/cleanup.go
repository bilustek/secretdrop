package cleanup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bilustek/secretdrop/internal/repository"
)

const defaultInterval = 1 * time.Minute

// Worker periodically deletes expired secrets from the database.
type Worker struct {
	repo     repository.Repository
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// Option configures a Worker value.
type Option func(*Worker) error

// WithInterval sets the cleanup interval.
func WithInterval(d time.Duration) Option {
	return func(w *Worker) error {
		if d <= 0 {
			return errors.New("interval must be positive")
		}

		w.interval = d

		return nil
	}
}

// New creates a new cleanup Worker.
func New(repo repository.Repository, opts ...Option) (*Worker, error) {
	w := &Worker{
		repo:     repo,
		interval: defaultInterval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(w); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return w, nil
}

// Start begins the periodic cleanup in a goroutine.
func (w *Worker) Start() {
	go w.run()
}

// Stop signals the worker to stop and waits for it to finish.
func (w *Worker) Stop() {
	close(w.stop)
	<-w.done
}

func (w *Worker) run() {
	defer close(w.done)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.cleanup()
		}
	}
}

func (w *Worker) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := w.repo.DeleteExpired(ctx, time.Now())
	if err != nil {
		slog.Error("cleanup expired secrets", "error", err)

		return
	}

	if count > 0 {
		slog.Info("cleaned up expired secrets", "count", count)
	}
}
