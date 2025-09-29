package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SagaMetrics содержит метрики для saga операций.
type SagaMetrics struct {
	// Счётчики операций
	sagaStarted   prometheus.Counter
	sagaCanceled  prometheus.Counter
	sagaRefunded  prometheus.Counter
	sagaCompleted prometheus.Counter
	sagaFailed    prometheus.Counter

	// Гистограммы времени выполнения
	sagaDuration prometheus.Histogram
	stepDuration *prometheus.HistogramVec

	// Счётчики событий timeline
	timelineEvents prometheus.Counter
	outboxEvents   prometheus.Counter

	// Gauge для активных саг
	activeSagas prometheus.Gauge
}

// NewSagaMetrics создаёт новый экземпляр метрик saga.
func NewSagaMetrics() *SagaMetrics {
	return &SagaMetrics{
		sagaStarted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "oms_saga_started_total",
			Help: "Total number of saga operations started",
		}),
		sagaCanceled: promauto.NewCounter(prometheus.CounterOpts{
			Name: "oms_saga_canceled_total",
			Help: "Total number of saga operations canceled",
		}),
		sagaRefunded: promauto.NewCounter(prometheus.CounterOpts{
			Name: "oms_saga_refunded_total",
			Help: "Total number of saga operations refunded",
		}),
		sagaCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "oms_saga_completed_total",
			Help: "Total number of saga operations completed successfully",
		}),
		sagaFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "oms_saga_failed_total",
			Help: "Total number of saga operations failed",
		}),
		sagaDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "oms_saga_duration_seconds",
			Help:    "Duration of saga operations in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		stepDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "oms_saga_step_duration_seconds",
			Help:    "Duration of individual saga steps in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		}, []string{"step"}),
		timelineEvents: promauto.NewCounter(prometheus.CounterOpts{
			Name: "oms_timeline_events_total",
			Help: "Total number of timeline events recorded",
		}),
		outboxEvents: promauto.NewCounter(prometheus.CounterOpts{
			Name: "oms_outbox_events_total",
			Help: "Total number of outbox events published",
		}),
		activeSagas: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "oms_active_sagas",
			Help: "Number of currently active saga operations",
		}),
	}
}

// RecordSagaStarted увеличивает счётчик запущенных саг.
func (m *SagaMetrics) RecordSagaStarted() {
	m.sagaStarted.Inc()
	m.activeSagas.Inc()
}

// RecordSagaCanceled увеличивает счётчик отменённых саг.
func (m *SagaMetrics) RecordSagaCanceled() {
	m.sagaCanceled.Inc()
	m.activeSagas.Dec()
}

// RecordSagaRefunded увеличивает счётчик возвращённых саг.
func (m *SagaMetrics) RecordSagaRefunded() {
	m.sagaRefunded.Inc()
	m.activeSagas.Dec()
}

// RecordSagaCompleted увеличивает счётчик завершённых саг.
func (m *SagaMetrics) RecordSagaCompleted() {
	m.sagaCompleted.Inc()
	m.activeSagas.Dec()
}

// RecordSagaFailed увеличивает счётчик неудачных саг.
func (m *SagaMetrics) RecordSagaFailed() {
	m.sagaFailed.Inc()
	m.activeSagas.Dec()
}

// RecordSagaDuration записывает время выполнения саги.
func (m *SagaMetrics) RecordSagaDuration(duration time.Duration) {
	m.sagaDuration.Observe(duration.Seconds())
}

// RecordStepDuration записывает время выполнения шага саги.
func (m *SagaMetrics) RecordStepDuration(step string, duration time.Duration) {
	m.stepDuration.WithLabelValues(step).Observe(duration.Seconds())
}

// RecordTimelineEvent увеличивает счётчик событий timeline.
func (m *SagaMetrics) RecordTimelineEvent() {
	m.timelineEvents.Inc()
}

// RecordOutboxEvent увеличивает счётчик событий outbox.
func (m *SagaMetrics) RecordOutboxEvent() {
	m.outboxEvents.Inc()
}
