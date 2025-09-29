package saga

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

// RetryConfig конфигурация для retry логики.
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig возвращает конфигурацию по умолчанию.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// RetryableOrchestrator оборачивает обычный оркестратор retry логикой.
type RetryableOrchestrator struct {
	orchestrator Orchestrator
	config       RetryConfig
	logger       *log.Entry
}

// NewRetryableOrchestrator создаёт новый оркестратор с retry логикой.
func NewRetryableOrchestrator(orchestrator Orchestrator, config RetryConfig, logger *log.Entry) *RetryableOrchestrator {
	if logger == nil {
		logger = log.New().WithField("component", "retryable-orchestrator")
	}
	
	return &RetryableOrchestrator{
		orchestrator: orchestrator,
		config:       config,
		logger:       logger,
	}
}

// Start запускает обработку заказа с retry логикой.
func (ro *RetryableOrchestrator) Start(orderID string) {
	ro.executeWithRetry("Start", orderID, func() error {
		ro.orchestrator.Start(orderID)
		return nil // Start не возвращает ошибку, но мы можем добавить проверки
	})
}

// Cancel отменяет заказ с retry логикой.
func (ro *RetryableOrchestrator) Cancel(orderID, reason string) {
	ro.executeWithRetry("Cancel", orderID, func() error {
		ro.orchestrator.Cancel(orderID, reason)
		return nil
	})
}

// Refund возвращает средства с retry логикой.
func (ro *RetryableOrchestrator) Refund(orderID string, amountMinor int64, reason string) {
	ro.executeWithRetry("Refund", orderID, func() error {
		ro.orchestrator.Refund(orderID, amountMinor, reason)
		return nil
	})
}

func (ro *RetryableOrchestrator) executeWithRetry(operation, orderID string, fn func() error) {
	var lastErr error
	delay := ro.config.InitialDelay
	
	for attempt := 1; attempt <= ro.config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			if attempt > 1 {
				ro.logger.WithFields(log.Fields{
					"operation": operation,
					"order_id":  orderID,
					"attempt":   attempt,
				}).Info("Operation succeeded after retry")
			}
			return
		}
		
		lastErr = err
		
		// Проверяем, стоит ли повторять попытку
		if !ro.shouldRetry(err) {
			ro.logger.WithFields(log.Fields{
				"operation": operation,
				"order_id":  orderID,
				"error":     err,
			}).Warn("Operation failed with non-retryable error")
			return
		}
		
		if attempt < ro.config.MaxAttempts {
			ro.logger.WithFields(log.Fields{
				"operation": operation,
				"order_id":  orderID,
				"attempt":   attempt,
				"delay":     delay,
				"error":     err,
			}).Warn("Operation failed, retrying")
			
			time.Sleep(delay)
			
			// Экспоненциальная задержка с ограничением
			delay = time.Duration(float64(delay) * ro.config.BackoffFactor)
			if delay > ro.config.MaxDelay {
				delay = ro.config.MaxDelay
			}
		}
	}
	
	ro.logger.WithFields(log.Fields{
		"operation":    operation,
		"order_id":     orderID,
		"max_attempts": ro.config.MaxAttempts,
		"error":        lastErr,
	}).Error("Operation failed after all retry attempts")
}

// shouldRetry определяет, стоит ли повторять операцию при данной ошибке.
func (ro *RetryableOrchestrator) shouldRetry(err error) bool {
	// Не повторяем при бизнес-логических ошибках
	if errors.Is(err, domain.ErrOrderNotFound) ||
		errors.Is(err, domain.ErrOrderVersionConflict) {
		return false
	}
	
	// Повторяем при временных ошибках сети, базы данных и т.д.
	if errors.Is(err, domain.ErrInventoryUnavailable) ||
		errors.Is(err, domain.ErrPaymentDeclined) {
		return true
	}
	
	// По умолчанию повторяем неизвестные ошибки
	return true
}

// CircuitBreaker простая реализация circuit breaker паттерна.
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	
	failures    int
	lastFailure time.Time
	state       CircuitState
	logger      *log.Entry
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker создаёт новый circuit breaker.
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration, logger *log.Entry) *CircuitBreaker {
	if logger == nil {
		logger = log.New().WithField("component", "circuit-breaker")
	}
	
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        CircuitClosed,
		logger:       logger,
	}
}

// Execute выполняет операцию через circuit breaker.
func (cb *CircuitBreaker) Execute(operation string, fn func() error) error {
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.logger.WithField("operation", operation).Info("Circuit breaker half-open")
		} else {
			return errors.New("circuit breaker is open")
		}
	}
	
	err := fn()
	
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		
		if cb.state == CircuitHalfOpen || cb.failures >= cb.maxFailures {
			cb.state = CircuitOpen
			cb.logger.WithFields(log.Fields{
				"operation": operation,
				"failures":  cb.failures,
			}).Warn("Circuit breaker opened")
		}
		
		return err
	}
	
	// Успешное выполнение - сбрасываем счётчик
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		cb.logger.WithField("operation", operation).Info("Circuit breaker closed")
	}
	cb.failures = 0
	
	return nil
}

// CircuitBreakerOrchestrator оркестратор с circuit breaker защитой.
type CircuitBreakerOrchestrator struct {
	orchestrator Orchestrator
	breaker      *CircuitBreaker
	logger       *log.Entry
}

// NewCircuitBreakerOrchestrator создаёт оркестратор с circuit breaker.
func NewCircuitBreakerOrchestrator(orchestrator Orchestrator, breaker *CircuitBreaker, logger *log.Entry) *CircuitBreakerOrchestrator {
	return &CircuitBreakerOrchestrator{
		orchestrator: orchestrator,
		breaker:      breaker,
		logger:       logger,
	}
}

// Start запускает обработку через circuit breaker.
func (cbo *CircuitBreakerOrchestrator) Start(orderID string) {
	err := cbo.breaker.Execute("Start", func() error {
		cbo.orchestrator.Start(orderID)
		return nil
	})
	
	if err != nil {
		cbo.logger.WithFields(log.Fields{
			"order_id": orderID,
			"error":    err,
		}).Error("Start operation blocked by circuit breaker")
	}
}

// Cancel отменяет заказ через circuit breaker.
func (cbo *CircuitBreakerOrchestrator) Cancel(orderID, reason string) {
	err := cbo.breaker.Execute("Cancel", func() error {
		cbo.orchestrator.Cancel(orderID, reason)
		return nil
	})
	
	if err != nil {
		cbo.logger.WithFields(log.Fields{
			"order_id": orderID,
			"reason":   reason,
			"error":    err,
		}).Error("Cancel operation blocked by circuit breaker")
	}
}

// Refund возвращает средства через circuit breaker.
func (cbo *CircuitBreakerOrchestrator) Refund(orderID string, amountMinor int64, reason string) {
	err := cbo.breaker.Execute("Refund", func() error {
		cbo.orchestrator.Refund(orderID, amountMinor, reason)
		return nil
	})
	
	if err != nil {
		cbo.logger.WithFields(log.Fields{
			"order_id":     orderID,
			"amount_minor": amountMinor,
			"reason":       reason,
			"error":        err,
		}).Error("Refund operation blocked by circuit breaker")
	}
}
