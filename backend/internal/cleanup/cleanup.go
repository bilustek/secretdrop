package cleanup

import (
	"context"
	"log/slog"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/repository"
)

// Worker periodically deletes expired secrets from the database.
type Worker struct {
	repo     repository.Repository
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewWorker creates a new cleanup Worker.
func NewWorker(repo repository.Repository, interval time.Duration) *Worker {
	return &Worker{
		repo:     repo,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
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
