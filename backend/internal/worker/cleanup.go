package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/devutility/webhookplatform/internal/domain"
)

// CleanupDeps are the dependencies the cleanup worker needs.
type CleanupDeps interface {
	ListExpiredEndpoints(ctx context.Context) ([]*domain.Endpoint, error)
	DeleteEndpoint(ctx context.Context, id string) error
}

// StoragePurger can delete objects from S3/MinIO.
type StoragePurger interface {
	DeleteByPrefix(ctx context.Context, prefix string) error
}

// CleanupWorker runs on a ticker and removes endpoints whose TTL has elapsed.
// It also purges every S3 object stored under the endpoint's prefix.
type CleanupWorker struct {
	repo    CleanupDeps
	storage StoragePurger
	log     *slog.Logger
}

func NewCleanupWorker(repo CleanupDeps, storage StoragePurger, log *slog.Logger) *CleanupWorker {
	return &CleanupWorker{repo: repo, storage: storage, log: log}
}

// Start runs the cleanup loop in a background goroutine. It stops when ctx is cancelled.
func (w *CleanupWorker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *CleanupWorker) loop(ctx context.Context) {
	// Run immediately at startup to catch any endpoints that expired while the server was down.
	w.run(ctx)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

func (w *CleanupWorker) run(ctx context.Context) {
	expired, err := w.repo.ListExpiredEndpoints(ctx)
	if err != nil {
		w.log.Error("cleanup: list expired endpoints", "error", err)
		return
	}
	for _, e := range expired {
		w.delete(ctx, e)
	}
}

func (w *CleanupWorker) delete(ctx context.Context, e *domain.Endpoint) {
	// Purge S3 objects first so we never orphan storage even if the DB delete fails.
	prefix := e.ID + "/"
	if err := w.storage.DeleteByPrefix(ctx, prefix); err != nil {
		w.log.Error("cleanup: purge s3 objects", "endpoint_id", e.ID, "error", err)
		// Continue and delete the DB row anyway.
	}

	if err := w.repo.DeleteEndpoint(ctx, e.ID); err != nil {
		w.log.Error("cleanup: delete endpoint", "endpoint_id", e.ID, "error", err)
		return
	}

	w.log.Info("endpoint expired and deleted", "endpoint_id", e.ID, "name", e.Name)
}
