package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	return newSagaMetricsWithRegisterer(prometheus.DefaultRegisterer)
}

func newSagaMetricsWithRegisterer(registerer prometheus.Registerer) *SagaMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	return &SagaMetrics{
		sagaStarted: registerCounter(registerer, prometheus.CounterOpts{
			Name: "oms_saga_started_total",
			Help: "Total number of saga operations started",
		}),
		sagaCanceled: registerCounter(registerer, prometheus.CounterOpts{
			Name: "oms_saga_canceled_total",
			Help: "Total number of saga operations canceled",
		}),
		sagaRefunded: registerCounter(registerer, prometheus.CounterOpts{
			Name: "oms_saga_refunded_total",
			Help: "Total number of saga operations refunded",
		}),
		sagaCompleted: registerCounter(registerer, prometheus.CounterOpts{
			Name: "oms_saga_completed_total",
			Help: "Total number of saga operations completed successfully",
		}),
		sagaFailed: registerCounter(registerer, prometheus.CounterOpts{
			Name: "oms_saga_failed_total",
			Help: "Total number of saga operations failed",
		}),
		sagaDuration: registerHistogram(registerer, prometheus.HistogramOpts{
			Name:    "oms_saga_duration_seconds",
			Help:    "Duration of saga operations in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		stepDuration: registerHistogramVec(registerer, prometheus.HistogramOpts{
			Name:    "oms_saga_step_duration_seconds",
			Help:    "Duration of individual saga steps in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		}, []string{"step"}),
		timelineEvents: registerCounter(registerer, prometheus.CounterOpts{
			Name: "oms_timeline_events_total",
			Help: "Total number of timeline events recorded",
		}),
		outboxEvents: registerCounter(registerer, prometheus.CounterOpts{
			Name: "oms_outbox_events_total",
			Help: "Total number of outbox events published",
		}),
		activeSagas: registerGauge(registerer, prometheus.GaugeOpts{
			Name: "oms_active_sagas",
			Help: "Number of currently active saga operations",
		}),
	}
}

func registerCounter(registerer prometheus.Registerer, opts prometheus.CounterOpts) prometheus.Counter {
	collector := prometheus.NewCounter(opts)
	if err := registerer.Register(collector); err != nil {
		if alreadyRegistered, ok := err.(prometheus.AlreadyRegisteredError); ok {
			existing, ok := alreadyRegistered.ExistingCollector.(prometheus.Counter)
			if !ok {
				panic(fmt.Sprintf("collector %q already registered with unexpected type", opts.Name))
			}
			return existing
		}
		panic(fmt.Sprintf("register counter %q: %v", opts.Name, err))
	}
	return collector
}

func registerGauge(registerer prometheus.Registerer, opts prometheus.GaugeOpts) prometheus.Gauge {
	collector := prometheus.NewGauge(opts)
	if err := registerer.Register(collector); err != nil {
		if alreadyRegistered, ok := err.(prometheus.AlreadyRegisteredError); ok {
			existing, ok := alreadyRegistered.ExistingCollector.(prometheus.Gauge)
			if !ok {
				panic(fmt.Sprintf("collector %q already registered with unexpected type", opts.Name))
			}
			return existing
		}
		panic(fmt.Sprintf("register gauge %q: %v", opts.Name, err))
	}
	return collector
}

func registerHistogram(registerer prometheus.Registerer, opts prometheus.HistogramOpts) prometheus.Histogram {
	collector := prometheus.NewHistogram(opts)
	if err := registerer.Register(collector); err != nil {
		if alreadyRegistered, ok := err.(prometheus.AlreadyRegisteredError); ok {
			existing, ok := alreadyRegistered.ExistingCollector.(prometheus.Histogram)
			if !ok {
				panic(fmt.Sprintf("collector %q already registered with unexpected type", opts.Name))
			}
			return existing
		}
		panic(fmt.Sprintf("register histogram %q: %v", opts.Name, err))
	}
	return collector
}

func registerHistogramVec(registerer prometheus.Registerer, opts prometheus.HistogramOpts, labels []string) *prometheus.HistogramVec {
	collector := prometheus.NewHistogramVec(opts, labels)
	if err := registerer.Register(collector); err != nil {
		if alreadyRegistered, ok := err.(prometheus.AlreadyRegisteredError); ok {
			existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.HistogramVec)
			if !ok {
				panic(fmt.Sprintf("collector %q already registered with unexpected type", opts.Name))
			}
			return existing
		}
		panic(fmt.Sprintf("register histogram vec %q: %v", opts.Name, err))
	}
	return collector
}

// RecordSagaStarted увеличивает счётчик запущенных саг.
func (m *SagaMetrics) RecordSagaStarted() {
	m.sagaStarted.Inc()
	m.RecordSagaInFlightStarted()
}

// RecordSagaCanceled увеличивает счётчик отменённых саг.
func (m *SagaMetrics) RecordSagaCanceled() {
	m.sagaCanceled.Inc()
}

// RecordSagaRefunded увеличивает счётчик возвращённых саг.
func (m *SagaMetrics) RecordSagaRefunded() {
	m.sagaRefunded.Inc()
}

// RecordSagaCompleted увеличивает счётчик завершённых саг.
func (m *SagaMetrics) RecordSagaCompleted() {
	m.sagaCompleted.Inc()
}

// RecordSagaFailed увеличивает счётчик неудачных саг.
func (m *SagaMetrics) RecordSagaFailed() {
	m.sagaFailed.Inc()
}

// RecordSagaInFlightStarted увеличивает количество активных саг.
func (m *SagaMetrics) RecordSagaInFlightStarted() {
	m.activeSagas.Inc()
}

// RecordSagaInFlightFinished уменьшает количество активных саг.
func (m *SagaMetrics) RecordSagaInFlightFinished() {
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
