package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

const (
	defaultCleanupInterval  = 10 * time.Minute
	defaultCleanupBatchSize = 500
)

var (
	idempotencyCleanupRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oms_idempotency_cleanup_runs_total",
		Help: "Total number of idempotency cleanup runs grouped by result.",
	}, []string{"result"})
	idempotencyCleanupDeletedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "oms_idempotency_cleanup_deleted_total",
		Help: "Total number of deleted expired idempotency records.",
	})
	idempotencyCleanupLastDeleted = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oms_idempotency_cleanup_last_deleted",
		Help: "Number of deleted records during the last cleanup run.",
	})
)

// CleanupOptions задает параметры воркера очистки idempotency ключей.
type CleanupOptions struct {
	Logger    *log.Entry
	Interval  time.Duration
	BatchSize int
}

// CleanupOption настраивает CleanupWorker.
type CleanupOption func(*CleanupOptions)

// WithLogger задает logger для воркера.
func WithLogger(logger *log.Entry) CleanupOption {
	return func(opts *CleanupOptions) {
		opts.Logger = logger
	}
}

// WithInterval задает интервал между cleanup-циклами.
func WithInterval(interval time.Duration) CleanupOption {
	return func(opts *CleanupOptions) {
		opts.Interval = interval
	}
}

// WithBatchSize задает размер batch для одного удаления.
func WithBatchSize(batchSize int) CleanupOption {
	return func(opts *CleanupOptions) {
		opts.BatchSize = batchSize
	}
}

// CleanupWorker периодически удаляет просроченные idempotency записи.
type CleanupWorker struct {
	repo      domain.IdempotencyRepository
	logger    *log.Entry
	interval  time.Duration
	batchSize int
}

// NewCleanupWorker создает воркер очистки idempotency ключей.
func NewCleanupWorker(repo domain.IdempotencyRepository, options ...CleanupOption) *CleanupWorker {
	opts := CleanupOptions{
		Interval:  defaultCleanupInterval,
		BatchSize: defaultCleanupBatchSize,
	}
	for _, option := range options {
		option(&opts)
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.WithField("component", "idempotency-cleanup-worker")
	}

	if opts.Interval <= 0 {
		opts.Interval = defaultCleanupInterval
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultCleanupBatchSize
	}

	return &CleanupWorker{
		repo:      repo,
		logger:    logger,
		interval:  opts.Interval,
		batchSize: opts.BatchSize,
	}
}

// Run запускает периодическую очистку до отмены ctx.
func (w *CleanupWorker) Run(ctx context.Context) {
	if w.repo == nil {
		w.logger.Warn("idempotency cleanup worker is disabled: repo is nil")
		return
	}

	w.cleanup(ctx, time.Now().UTC())

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanup(ctx, time.Now().UTC())
		}
	}
}

func (w *CleanupWorker) cleanup(ctx context.Context, before time.Time) {
	deleted, err := w.DeleteExpired(ctx, before)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		idempotencyCleanupRunsTotal.WithLabelValues("error").Inc()
		w.logger.WithError(err).Warn("idempotency cleanup run failed")
		return
	}

	idempotencyCleanupRunsTotal.WithLabelValues("ok").Inc()
	idempotencyCleanupLastDeleted.Set(float64(deleted))
	if deleted > 0 {
		w.logger.WithField("deleted", deleted).Info("idempotency cleanup completed")
	}
}

// DeleteExpired удаляет все записи с ttl <= before порциями batchSize.
func (w *CleanupWorker) DeleteExpired(ctx context.Context, before time.Time) (int, error) {
	if before.IsZero() {
		before = time.Now().UTC()
	}

	totalDeleted := 0
	for {
		if err := ctx.Err(); err != nil {
			return totalDeleted, err
		}

		deleted, err := w.repo.DeleteExpired(before, w.batchSize)
		if err != nil {
			return totalDeleted, err
		}

		totalDeleted += deleted
		if deleted > 0 {
			idempotencyCleanupDeletedTotal.Add(float64(deleted))
		}

		if deleted < w.batchSize {
			break
		}
	}

	return totalDeleted, nil
}
