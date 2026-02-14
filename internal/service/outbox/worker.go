package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

const (
	defaultPollInterval   = 1 * time.Second
	defaultBatchSize      = 100
	defaultMaxAttempts    = 3
	defaultRetryBaseDelay = 50 * time.Millisecond
)

var (
	outboxPublishAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oms_outbox_publish_attempts_total",
		Help: "Total number of outbox publish attempts grouped by result.",
	}, []string{"result"})
	outboxPendingRecords = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oms_outbox_pending_records",
		Help: "Current number of pending records in transactional outbox.",
	})
	outboxOldestPendingAge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oms_outbox_oldest_pending_age_seconds",
		Help: "Age in seconds of the oldest pending outbox record.",
	})
)

// WorkerOptions задаёт параметры outbox worker.
type WorkerOptions struct {
	Logger         *log.Entry
	DLQPublisher   domain.OutboxPublisher
	PollInterval   time.Duration
	BatchSize      int
	MaxAttempts    int
	RetryBaseDelay time.Duration
}

// Option настраивает Worker.
type Option func(*WorkerOptions)

// WithLogger задаёт logger для воркера.
func WithLogger(logger *log.Entry) Option {
	return func(opts *WorkerOptions) {
		opts.Logger = logger
	}
}

// WithDLQPublisher задаёт publisher для отправки в DLQ после исчерпания retry.
func WithDLQPublisher(publisher domain.OutboxPublisher) Option {
	return func(opts *WorkerOptions) {
		opts.DLQPublisher = publisher
	}
}

// WithPollInterval задаёт частоту опроса outbox.
func WithPollInterval(interval time.Duration) Option {
	return func(opts *WorkerOptions) {
		opts.PollInterval = interval
	}
}

// WithBatchSize задаёт размер батча из outbox.
func WithBatchSize(batchSize int) Option {
	return func(opts *WorkerOptions) {
		opts.BatchSize = batchSize
	}
}

// WithMaxAttempts задаёт число попыток публикации перед failed/DLQ.
func WithMaxAttempts(maxAttempts int) Option {
	return func(opts *WorkerOptions) {
		opts.MaxAttempts = maxAttempts
	}
}

// WithRetryBaseDelay задаёт базовый delay для exponential backoff.
func WithRetryBaseDelay(delay time.Duration) Option {
	return func(opts *WorkerOptions) {
		opts.RetryBaseDelay = delay
	}
}

// Worker публикует pending-сообщения из outbox в брокер.
type Worker struct {
	repo           domain.OutboxRepository
	publisher      domain.OutboxPublisher
	dlqPublisher   domain.OutboxPublisher
	logger         *log.Entry
	pollInterval   time.Duration
	batchSize      int
	maxAttempts    int
	retryBaseDelay time.Duration
}

// NewWorker создаёт outbox worker.
func NewWorker(repo domain.OutboxRepository, publisher domain.OutboxPublisher, options ...Option) *Worker {
	opts := WorkerOptions{
		PollInterval:   defaultPollInterval,
		BatchSize:      defaultBatchSize,
		MaxAttempts:    defaultMaxAttempts,
		RetryBaseDelay: defaultRetryBaseDelay,
	}
	for _, option := range options {
		option(&opts)
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.WithField("component", "outbox-worker")
	}

	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultPollInterval
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultMaxAttempts
	}
	if opts.RetryBaseDelay < 0 {
		opts.RetryBaseDelay = 0
	}

	return &Worker{
		repo:           repo,
		publisher:      publisher,
		dlqPublisher:   opts.DLQPublisher,
		logger:         logger,
		pollInterval:   opts.PollInterval,
		batchSize:      opts.BatchSize,
		maxAttempts:    opts.MaxAttempts,
		retryBaseDelay: opts.RetryBaseDelay,
	}
}

// Run запускает периодический polling outbox до отмены ctx.
func (w *Worker) Run(ctx context.Context) {
	if w.repo == nil || w.publisher == nil {
		w.logger.Warn("outbox worker is disabled: repo or publisher is nil")
		return
	}

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	w.ProcessOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.ProcessOnce(ctx)
		}
	}
}

// ProcessOnce выполняет один polling-цикл.
func (w *Worker) ProcessOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	w.refreshBacklogMetrics()

	events, err := w.repo.PullPending(w.batchSize)
	if err != nil {
		w.logger.WithError(err).Warn("failed to pull pending outbox messages")
		return
	}
	if len(events) == 0 {
		return
	}

	for _, event := range events {
		if ctx.Err() != nil {
			return
		}

		if err := w.publishWithRetry(ctx, event); err != nil {
			w.logger.WithError(err).WithFields(log.Fields{
				"outbox_id":  event.ID,
				"event_type": event.EventType,
			}).Error("outbox publish failed after retries")
			outboxPublishAttempts.WithLabelValues("failed").Inc()

			if dlqErr := w.publishToDLQ(event, err); dlqErr != nil {
				w.logger.WithError(dlqErr).WithField("outbox_id", event.ID).Warn("failed to publish to DLQ")
				outboxPublishAttempts.WithLabelValues("dlq_failed").Inc()
			}
			if markErr := w.repo.MarkFailed(event.ID); markErr != nil {
				w.logger.WithError(markErr).WithField("outbox_id", event.ID).Warn("failed to mark outbox as failed")
			}
			continue
		}

		if err := w.repo.MarkSent(event.ID); err != nil {
			w.logger.WithError(err).WithField("outbox_id", event.ID).Warn("failed to mark outbox as sent")
		}
	}

	w.refreshBacklogMetrics()
}

func (w *Worker) publishWithRetry(ctx context.Context, event domain.OutboxMessage) error {
	var lastErr error

	for attempt := 1; attempt <= w.maxAttempts; attempt++ {
		err := w.publisher.Publish(event)
		if err == nil {
			outboxPublishAttempts.WithLabelValues("sent").Inc()
			return nil
		}
		lastErr = err
		outboxPublishAttempts.WithLabelValues("retry_error").Inc()

		if attempt >= w.maxAttempts {
			break
		}

		delay := w.retryBackoff(attempt)
		if delay <= 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("publish failed after %d attempts: %w", w.maxAttempts, lastErr)
}

func (w *Worker) refreshBacklogMetrics() {
	stats, err := w.repo.Stats()
	if err != nil {
		w.logger.WithError(err).Warn("failed to collect outbox backlog stats")
		return
	}

	outboxPendingRecords.Set(float64(stats.PendingCount))
	if stats.PendingCount == 0 || stats.OldestPendingAt.IsZero() {
		outboxOldestPendingAge.Set(0)
		return
	}

	age := time.Since(stats.OldestPendingAt).Seconds()
	if age < 0 {
		age = 0
	}
	outboxOldestPendingAge.Set(age)
}

func (w *Worker) retryBackoff(attempt int) time.Duration {
	if w.retryBaseDelay <= 0 {
		return 0
	}
	if attempt <= 1 {
		return w.retryBaseDelay
	}

	const maxDuration = time.Duration(1<<63 - 1)
	delay := w.retryBaseDelay
	for i := 1; i < attempt; i++ {
		if delay > maxDuration/2 {
			return maxDuration
		}
		delay *= 2
	}
	return delay
}

func (w *Worker) publishToDLQ(event domain.OutboxMessage, publishErr error) error {
	if w.dlqPublisher == nil {
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"outbox_id":        event.ID,
		"aggregate_type":   event.AggregateType,
		"aggregate_id":     event.AggregateID,
		"event_type":       event.EventType,
		"payload":          json.RawMessage(event.Payload),
		"publish_error":    publishErr.Error(),
		"dlq_published_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("marshal dlq payload: %w", err)
	}

	dlqEvent := domain.OutboxMessage{
		ID:            event.ID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		Payload:       payload,
	}
	if err := w.dlqPublisher.Publish(dlqEvent); err != nil {
		return fmt.Errorf("publish to dlq: %w", err)
	}

	return nil
}
