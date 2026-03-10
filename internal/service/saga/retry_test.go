package saga

import (
	"errors"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

type stubOrchestrator struct {
	startCalls  int
	cancelCalls int
	refundCalls int
}

func (s *stubOrchestrator) Start(string) {
	s.startCalls++
}

func (s *stubOrchestrator) Cancel(string, string) {
	s.cancelCalls++
}

func (s *stubOrchestrator) Refund(string, int64, string) {
	s.refundCalls++
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxAttempts != 3 {
		t.Fatalf("unexpected MaxAttempts: %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay <= 0 || cfg.MaxDelay <= 0 {
		t.Fatalf("delays must be positive: %+v", cfg)
	}
	if cfg.BackoffFactor <= 1 {
		t.Fatalf("backoff factor should be > 1: %f", cfg.BackoffFactor)
	}
}

func TestRetryableOrchestratorHelpers(t *testing.T) {
	stub := &stubOrchestrator{}
	ro := NewRetryableOrchestrator(stub, RetryConfig{MaxAttempts: 1}, nil)
	if ro.logger == nil {
		t.Fatal("expected default logger")
	}

	ro.Start("o-1")
	ro.Cancel("o-1", "r")
	ro.Refund("o-1", 10, "refund")

	if stub.startCalls != 1 || stub.cancelCalls != 1 || stub.refundCalls != 1 {
		t.Fatalf("unexpected delegate calls: %+v", stub)
	}
}

func TestRetryableOrchestratorExecuteWithRetry(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, BackoffFactor: 2}
	ro := NewRetryableOrchestrator(&stubOrchestrator{}, cfg, log.New().WithField("test", "retry"))

	t.Run("retry then success", func(t *testing.T) {
		attempts := 0
		ro.executeWithRetry("op", "order-1", func() error {
			attempts++
			if attempts < 3 {
				return domain.ErrInventoryTemporary
			}
			return nil
		})
		if attempts != 3 {
			t.Fatalf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable", func(t *testing.T) {
		attempts := 0
		ro.executeWithRetry("op", "order-2", func() error {
			attempts++
			return domain.ErrOrderNotFound
		})
		if attempts != 1 {
			t.Fatalf("expected single attempt for non-retryable error, got %d", attempts)
		}
	})

	t.Run("exhausted retries", func(t *testing.T) {
		attempts := 0
		ro.executeWithRetry("op", "order-3", func() error {
			attempts++
			return errors.New("temporary")
		})
		if attempts != cfg.MaxAttempts {
			t.Fatalf("expected %d attempts, got %d", cfg.MaxAttempts, attempts)
		}
	})
}

func TestRetryableOrchestratorShouldRetry(t *testing.T) {
	ro := NewRetryableOrchestrator(&stubOrchestrator{}, RetryConfig{MaxAttempts: 1}, nil)

	if ro.shouldRetry(domain.ErrOrderNotFound) {
		t.Fatal("ErrOrderNotFound should not be retried")
	}
	if ro.shouldRetry(domain.ErrOrderVersionConflict) {
		t.Fatal("ErrOrderVersionConflict should not be retried")
	}
	if ro.shouldRetry(domain.ErrInventoryUnavailable) {
		t.Fatal("ErrInventoryUnavailable should not be retried")
	}
	if ro.shouldRetry(domain.ErrPaymentDeclined) {
		t.Fatal("ErrPaymentDeclined should not be retried")
	}
	if !ro.shouldRetry(domain.ErrInventoryTemporary) {
		t.Fatal("ErrInventoryTemporary should be retried")
	}
	if !ro.shouldRetry(domain.ErrPaymentTemporary) {
		t.Fatal("ErrPaymentTemporary should be retried")
	}
	if !ro.shouldRetry(domain.ErrPaymentIndeterminate) {
		t.Fatal("ErrPaymentIndeterminate should be retried")
	}
	if !ro.shouldRetry(errors.New("unknown")) {
		t.Fatal("unknown errors should be retried by default")
	}
}

func TestCircuitBreakerExecute(t *testing.T) {
	cb := NewCircuitBreaker(2, 20*time.Millisecond, nil)
	if cb.logger == nil {
		t.Fatal("expected default logger")
	}
	if cb.state != CircuitClosed {
		t.Fatalf("expected closed state, got %v", cb.state)
	}

	// Successful call keeps breaker closed.
	if err := cb.Execute("ok", func() error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.state != CircuitClosed || cb.failures != 0 {
		t.Fatalf("unexpected state after success: state=%v failures=%d", cb.state, cb.failures)
	}

	// Two failures open the breaker.
	if err := cb.Execute("fail-1", func() error { return errors.New("boom") }); err == nil {
		t.Fatal("expected first failure")
	}
	if cb.state != CircuitClosed {
		t.Fatalf("breaker should still be closed after first failure, got %v", cb.state)
	}
	if err := cb.Execute("fail-2", func() error { return errors.New("boom") }); err == nil {
		t.Fatal("expected second failure")
	}
	if cb.state != CircuitOpen {
		t.Fatalf("breaker should be open, got %v", cb.state)
	}

	// Open breaker rejects immediately.
	if err := cb.Execute("blocked", func() error { return nil }); err == nil || err.Error() != "circuit breaker is open" {
		t.Fatalf("expected open breaker error, got %v", err)
	}

	// After reset timeout, breaker goes half-open and closes on success.
	cb.lastFailure = time.Now().Add(-time.Second)
	if err := cb.Execute("half-open-success", func() error { return nil }); err != nil {
		t.Fatalf("unexpected error in half-open: %v", err)
	}
	if cb.state != CircuitClosed {
		t.Fatalf("expected closed state after half-open success, got %v", cb.state)
	}

	// Half-open failure re-opens.
	cb.state = CircuitOpen
	cb.lastFailure = time.Now().Add(-time.Second)
	if err := cb.Execute("half-open-fail", func() error { return errors.New("still failing") }); err == nil {
		t.Fatal("expected error in half-open failure")
	}
	if cb.state != CircuitOpen {
		t.Fatalf("expected open state after half-open failure, got %v", cb.state)
	}
}

func TestCircuitBreakerOrchestrator(t *testing.T) {
	stub := &stubOrchestrator{}
	breaker := NewCircuitBreaker(1, time.Hour, log.New().WithField("test", "breaker"))
	logger := log.New().WithField("test", "cbo")
	cbo := NewCircuitBreakerOrchestrator(stub, breaker, logger)

	// Closed breaker delegates calls.
	cbo.Start("o-1")
	cbo.Cancel("o-1", "reason")
	cbo.Refund("o-1", 10, "refund")
	if stub.startCalls != 1 || stub.cancelCalls != 1 || stub.refundCalls != 1 {
		t.Fatalf("unexpected delegate calls in closed state: %+v", stub)
	}

	// Open breaker blocks calls before fn executes.
	breaker.state = CircuitOpen
	breaker.lastFailure = time.Now()
	cbo.Start("o-2")
	cbo.Cancel("o-2", "reason")
	cbo.Refund("o-2", 10, "refund")
	if stub.startCalls != 1 || stub.cancelCalls != 1 || stub.refundCalls != 1 {
		t.Fatalf("calls should be blocked in open state: %+v", stub)
	}
}
