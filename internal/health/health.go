package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status представляет статус компонента
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check представляет проверку здоровья компонента
type Check struct {
	Name       string `json:"name"`
	Status     Status `json:"status"`
	Message    string `json:"message,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// Response представляет ответ health check
type Response struct {
	Status        Status           `json:"status"`
	Timestamp     time.Time        `json:"timestamp"`
	Checks        map[string]Check `json:"checks,omitempty"`
	Version       string           `json:"version,omitempty"`
	UptimeSeconds int64            `json:"uptime_seconds"`
}

// Checker интерфейс для проверки здоровья компонента
type Checker interface {
	Check() Check
}

// Handler обрабатывает health check запросы
type Handler struct {
	mu        sync.RWMutex
	checkers  map[string]Checker
	version   string
	startTime time.Time
}

// NewHandler создаёт новый health handler
func NewHandler(version string) *Handler {
	return &Handler{
		checkers:  make(map[string]Checker),
		version:   version,
		startTime: time.Now(),
	}
}

// RegisterChecker регистрирует проверку компонента
func (h *Handler) RegisterChecker(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// ServeHTTP обрабатывает HTTP запрос
func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	checkers := make(map[string]Checker, len(h.checkers))
	for k, v := range h.checkers {
		checkers[k] = v
	}
	h.mu.RUnlock()

	// Выполняем все проверки
	checks := make(map[string]Check)
	overallStatus := StatusHealthy

	for name, checker := range checkers {
		check := checker.Check()
		checks[name] = check

		// Определяем общий статус
		if check.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if check.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	// Формируем ответ
	response := Response{
		Status:        overallStatus,
		Timestamp:     time.Now(),
		Checks:        checks,
		Version:       h.version,
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
	}

	// Устанавливаем HTTP статус
	statusCode := http.StatusOK
	if overallStatus == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// LivenessHandler простой liveness probe (всегда возвращает 200)
func LivenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ReadinessHandler проверяет готовность к обработке запросов
func (h *Handler) ReadinessHandler(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	checkers := make(map[string]Checker, len(h.checkers))
	for k, v := range h.checkers {
		checkers[k] = v
	}
	h.mu.RUnlock()

	// Проверяем критичные компоненты
	for _, checker := range checkers {
		check := checker.Check()
		if check.Status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// SimpleChecker простая проверка с функцией
type SimpleChecker struct {
	name    string
	checkFn func() error
}

// NewSimpleChecker создаёт простую проверку
func NewSimpleChecker(name string, checkFn func() error) *SimpleChecker {
	return &SimpleChecker{
		name:    name,
		checkFn: checkFn,
	}
}

// Check выполняет проверку
func (c *SimpleChecker) Check() Check {
	start := time.Now()
	err := c.checkFn()
	duration := time.Since(start)

	if err != nil {
		return Check{
			Name:       c.name,
			Status:     StatusUnhealthy,
			Message:    err.Error(),
			DurationMs: duration.Milliseconds(),
		}
	}

	return Check{
		Name:       c.name,
		Status:     StatusHealthy,
		DurationMs: duration.Milliseconds(),
	}
}
