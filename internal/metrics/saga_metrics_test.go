package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewSagaMetrics(t *testing.T) {
	metrics := NewSagaMetrics()

	if metrics == nil {
		t.Fatal("NewSagaMetrics should not return nil")
	}

	if metrics.sagaStarted == nil {
		t.Error("sagaStarted counter should not be nil")
	}

	if metrics.sagaCanceled == nil {
		t.Error("sagaCanceled counter should not be nil")
	}

	if metrics.sagaRefunded == nil {
		t.Error("sagaRefunded counter should not be nil")
	}

	if metrics.sagaCompleted == nil {
		t.Error("sagaCompleted counter should not be nil")
	}

	if metrics.sagaFailed == nil {
		t.Error("sagaFailed counter should not be nil")
	}

	if metrics.sagaDuration == nil {
		t.Error("sagaDuration histogram should not be nil")
	}

	if metrics.stepDuration == nil {
		t.Error("stepDuration histogram vec should not be nil")
	}

	if metrics.timelineEvents == nil {
		t.Error("timelineEvents counter should not be nil")
	}

	if metrics.outboxEvents == nil {
		t.Error("outboxEvents counter should not be nil")
	}

	if metrics.activeSagas == nil {
		t.Error("activeSagas gauge should not be nil")
	}
}

func TestRecordSagaStarted(t *testing.T) {
	// Create isolated metrics with a custom registry
	reg := prometheus.NewRegistry()

	sagaStarted := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_saga_started_total",
		Help: "Test counter",
	})
	activeSagas := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_active_sagas",
		Help: "Test gauge",
	})

	reg.MustRegister(sagaStarted, activeSagas)

	metrics := &SagaMetrics{
		sagaStarted: sagaStarted,
		activeSagas: activeSagas,
	}

	// Record saga started
	metrics.RecordSagaStarted()

	// Check counter-value
	metric := &dto.Metric{}
	if err := sagaStarted.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Counter.GetValue() != 1.0 {
		t.Errorf("expected counter value 1.0, got %f", metric.Counter.GetValue())
	}

	// Check active sagas increased
	gaugeMetric := &dto.Metric{}
	if err := activeSagas.Write(gaugeMetric); err != nil {
		t.Fatalf("failed to write gauge: %v", err)
	}

	if gaugeMetric.Gauge.GetValue() != 1.0 {
		t.Errorf("expected active sagas 1.0, got %f", gaugeMetric.Gauge.GetValue())
	}
}

func TestRecordSagaCanceled(t *testing.T) {
	reg := prometheus.NewRegistry()

	sagaCanceled := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_saga_canceled_total",
		Help: "Test counter",
	})
	activeSagas := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_active_sagas_cancel",
		Help: "Test gauge",
	})

	reg.MustRegister(sagaCanceled, activeSagas)

	metrics := &SagaMetrics{
		sagaCanceled: sagaCanceled,
		activeSagas:  activeSagas,
	}

	// Set initial active sagas
	activeSagas.Set(5)

	// Record saga canceled
	metrics.RecordSagaCanceled()

	// Check counter
	metric := &dto.Metric{}
	if err := sagaCanceled.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Counter.GetValue() != 1.0 {
		t.Errorf("expected counter value 1.0, got %f", metric.Counter.GetValue())
	}

	// Check active sagas unchanged (decrement happens on RecordSagaInFlightFinished)
	gaugeMetric := &dto.Metric{}
	if err := activeSagas.Write(gaugeMetric); err != nil {
		t.Fatalf("failed to write gauge: %v", err)
	}

	if gaugeMetric.Gauge.GetValue() != 5.0 {
		t.Errorf("expected active sagas 5.0, got %f", gaugeMetric.Gauge.GetValue())
	}
}

func TestRecordSagaRefunded(t *testing.T) {
	reg := prometheus.NewRegistry()

	sagaRefunded := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_saga_refunded_total",
		Help: "Test counter",
	})
	activeSagas := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_active_sagas_refund",
		Help: "Test gauge",
	})

	reg.MustRegister(sagaRefunded, activeSagas)

	metrics := &SagaMetrics{
		sagaRefunded: sagaRefunded,
		activeSagas:  activeSagas,
	}

	activeSagas.Set(3)
	metrics.RecordSagaRefunded()

	metric := &dto.Metric{}
	if err := sagaRefunded.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Counter.GetValue() != 1.0 {
		t.Errorf("expected counter value 1.0, got %f", metric.Counter.GetValue())
	}

	gaugeMetric := &dto.Metric{}
	if err := activeSagas.Write(gaugeMetric); err != nil {
		t.Fatalf("failed to write gauge: %v", err)
	}

	if gaugeMetric.Gauge.GetValue() != 3.0 {
		t.Errorf("expected active sagas 3.0, got %f", gaugeMetric.Gauge.GetValue())
	}
}

func TestRecordSagaCompleted(t *testing.T) {
	reg := prometheus.NewRegistry()

	sagaCompleted := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_saga_completed_total",
		Help: "Test counter",
	})
	activeSagas := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_active_sagas_complete",
		Help: "Test gauge",
	})

	reg.MustRegister(sagaCompleted, activeSagas)

	metrics := &SagaMetrics{
		sagaCompleted: sagaCompleted,
		activeSagas:   activeSagas,
	}

	activeSagas.Set(10)
	metrics.RecordSagaCompleted()

	metric := &dto.Metric{}
	if err := sagaCompleted.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Counter.GetValue() != 1.0 {
		t.Errorf("expected counter value 1.0, got %f", metric.Counter.GetValue())
	}

	gaugeMetric := &dto.Metric{}
	if err := activeSagas.Write(gaugeMetric); err != nil {
		t.Fatalf("failed to write gauge: %v", err)
	}

	if gaugeMetric.Gauge.GetValue() != 10.0 {
		t.Errorf("expected active sagas 10.0, got %f", gaugeMetric.Gauge.GetValue())
	}
}

func TestRecordSagaFailed(t *testing.T) {
	reg := prometheus.NewRegistry()

	sagaFailed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_saga_failed_total",
		Help: "Test counter",
	})
	activeSagas := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_active_sagas_fail",
		Help: "Test gauge",
	})

	reg.MustRegister(sagaFailed, activeSagas)

	metrics := &SagaMetrics{
		sagaFailed:  sagaFailed,
		activeSagas: activeSagas,
	}

	activeSagas.Set(7)
	metrics.RecordSagaFailed()

	metric := &dto.Metric{}
	if err := sagaFailed.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Counter.GetValue() != 1.0 {
		t.Errorf("expected counter value 1.0, got %f", metric.Counter.GetValue())
	}

	gaugeMetric := &dto.Metric{}
	if err := activeSagas.Write(gaugeMetric); err != nil {
		t.Fatalf("failed to write gauge: %v", err)
	}

	if gaugeMetric.Gauge.GetValue() != 7.0 {
		t.Errorf("expected active sagas 7.0, got %f", gaugeMetric.Gauge.GetValue())
	}
}

func TestRecordSagaDuration(t *testing.T) {
	reg := prometheus.NewRegistry()

	sagaDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "test_saga_duration_seconds",
		Help:    "Test histogram",
		Buckets: prometheus.DefBuckets,
	})

	reg.MustRegister(sagaDuration)

	metrics := &SagaMetrics{
		sagaDuration: sagaDuration,
	}

	// Record some durations
	metrics.RecordSagaDuration(100 * time.Millisecond)
	metrics.RecordSagaDuration(500 * time.Millisecond)
	metrics.RecordSagaDuration(1 * time.Second)

	metric := &dto.Metric{}
	if err := sagaDuration.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Histogram.GetSampleCount() != 3 {
		t.Errorf("expected 3 samples, got %d", metric.Histogram.GetSampleCount())
	}

	// Check sum is approximately correct (0.1 + 0.5 + 1.0 = 1.6)
	sum := metric.Histogram.GetSampleSum()
	if sum < 1.5 || sum > 1.7 {
		t.Errorf("expected sum around 1.6, got %f", sum)
	}
}

func TestRecordStepDuration(t *testing.T) {
	reg := prometheus.NewRegistry()

	stepDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "test_saga_step_duration_seconds",
		Help:    "Test histogram vec",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	}, []string{"step"})

	reg.MustRegister(stepDuration)

	metrics := &SagaMetrics{
		stepDuration: stepDuration,
	}

	// Record durations for different steps
	metrics.RecordStepDuration("reserve", 50*time.Millisecond)
	metrics.RecordStepDuration("pay", 100*time.Millisecond)
	metrics.RecordStepDuration("confirm", 25*time.Millisecond)

	// Check reserve step
	reserveMetric := &dto.Metric{}
	observer := stepDuration.WithLabelValues("reserve")
	if err := observer.(prometheus.Histogram).Write(reserveMetric); err != nil {
		t.Fatalf("failed to write reserve metric: %v", err)
	}

	if reserveMetric.Histogram.GetSampleCount() != 1 {
		t.Errorf("expected 1 sample for reserve, got %d", reserveMetric.Histogram.GetSampleCount())
	}
}

func TestRecordTimelineEvent(t *testing.T) {
	reg := prometheus.NewRegistry()

	timelineEvents := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_timeline_events_total",
		Help: "Test counter",
	})

	reg.MustRegister(timelineEvents)

	metrics := &SagaMetrics{
		timelineEvents: timelineEvents,
	}

	// Record multiple events
	metrics.RecordTimelineEvent()
	metrics.RecordTimelineEvent()
	metrics.RecordTimelineEvent()

	metric := &dto.Metric{}
	if err := timelineEvents.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Counter.GetValue() != 3.0 {
		t.Errorf("expected counter value 3.0, got %f", metric.Counter.GetValue())
	}
}

func TestRecordOutboxEvent(t *testing.T) {
	reg := prometheus.NewRegistry()

	outboxEvents := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_outbox_events_total",
		Help: "Test counter",
	})

	reg.MustRegister(outboxEvents)

	metrics := &SagaMetrics{
		outboxEvents: outboxEvents,
	}

	// Record multiple events
	metrics.RecordOutboxEvent()
	metrics.RecordOutboxEvent()

	metric := &dto.Metric{}
	if err := outboxEvents.Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	if metric.Counter.GetValue() != 2.0 {
		t.Errorf("expected counter value 2.0, got %f", metric.Counter.GetValue())
	}
}

func TestSagaLifecycle(t *testing.T) {
	reg := prometheus.NewRegistry()

	activeSagas := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_saga_lifecycle_active",
		Help: "Test gauge",
	})
	sagaStarted := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_saga_lifecycle_started",
		Help: "Test counter",
	})
	sagaCompleted := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_saga_lifecycle_completed",
		Help: "Test counter",
	})

	reg.MustRegister(activeSagas, sagaStarted, sagaCompleted)

	metrics := &SagaMetrics{
		activeSagas:   activeSagas,
		sagaStarted:   sagaStarted,
		sagaCompleted: sagaCompleted,
	}

	// Simulate saga lifecycle
	metrics.RecordSagaStarted() // active: 1
	metrics.RecordSagaStarted() // active: 2
	metrics.RecordSagaStarted() // active: 3

	metrics.RecordSagaCompleted()
	metrics.RecordSagaInFlightFinished() // active: 2
	metrics.RecordSagaCompleted()
	metrics.RecordSagaInFlightFinished() // active: 1

	// Check active sagas
	gaugeMetric := &dto.Metric{}
	if err := activeSagas.Write(gaugeMetric); err != nil {
		t.Fatalf("failed to write gauge: %v", err)
	}

	if gaugeMetric.Gauge.GetValue() != 1.0 {
		t.Errorf("expected 1 active saga, got %f", gaugeMetric.Gauge.GetValue())
	}

	// Check started count
	startedMetric := &dto.Metric{}
	if err := sagaStarted.Write(startedMetric); err != nil {
		t.Fatalf("failed to write started metric: %v", err)
	}

	if startedMetric.Counter.GetValue() != 3.0 {
		t.Errorf("expected 3 started sagas, got %f", startedMetric.Counter.GetValue())
	}

	// Check completed count
	completedMetric := &dto.Metric{}
	if err := sagaCompleted.Write(completedMetric); err != nil {
		t.Fatalf("failed to write completed metric: %v", err)
	}

	if completedMetric.Counter.GetValue() != 2.0 {
		t.Errorf("expected 2 completed sagas, got %f", completedMetric.Counter.GetValue())
	}
}

func TestRecordSagaInFlightFinished(t *testing.T) {
	reg := prometheus.NewRegistry()

	activeSagas := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_active_sagas_inflight",
		Help: "Test gauge",
	})

	reg.MustRegister(activeSagas)

	metrics := &SagaMetrics{
		activeSagas: activeSagas,
	}

	metrics.RecordSagaInFlightStarted()
	metrics.RecordSagaInFlightStarted()
	metrics.RecordSagaInFlightFinished()

	gaugeMetric := &dto.Metric{}
	if err := activeSagas.Write(gaugeMetric); err != nil {
		t.Fatalf("failed to write gauge: %v", err)
	}

	if gaugeMetric.Gauge.GetValue() != 1.0 {
		t.Errorf("expected 1.0 active saga, got %f", gaugeMetric.Gauge.GetValue())
	}
}
