package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	handler := NewHandler("v1.0.0")

	// Добавляем здоровую проверку
	handler.RegisterChecker("test-healthy", NewSimpleChecker("test", func() error {
		return nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response Response
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != StatusHealthy {
		t.Errorf("expected status healthy, got %s", response.Status)
	}

	if response.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %s", response.Version)
	}

	if len(response.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(response.Checks))
	}
}

func TestHealthHandler_Unhealthy(t *testing.T) {
	handler := NewHandler("v1.0.0")

	// Добавляем нездоровую проверку
	handler.RegisterChecker("test-unhealthy", NewSimpleChecker("test", func() error {
		return errors.New("service unavailable")
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response Response
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != StatusUnhealthy {
		t.Errorf("expected status unhealthy, got %s", response.Status)
	}
}

func TestLivenessHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()

	LivenessHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %s", w.Body.String())
	}
}

func TestReadinessHandler(t *testing.T) {
	handler := NewHandler("v1.0.0")

	// Добавляем здоровую проверку
	handler.RegisterChecker("test", NewSimpleChecker("test", func() error {
		return nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ReadinessHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "ready" {
		t.Errorf("expected body 'ready', got %s", w.Body.String())
	}
}

func TestReadinessHandler_NotReady(t *testing.T) {
	handler := NewHandler("v1.0.0")

	// Добавляем нездоровую проверку
	handler.RegisterChecker("test", NewSimpleChecker("test", func() error {
		return errors.New("not ready")
	}))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ReadinessHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	if w.Body.String() != "not ready" {
		t.Errorf("expected body 'not ready', got %s", w.Body.String())
	}
}

func TestSimpleChecker(t *testing.T) {
	checker := NewSimpleChecker("test", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	check := checker.Check()

	if check.Status != StatusHealthy {
		t.Errorf("expected status healthy, got %s", check.Status)
	}

	if check.Duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", check.Duration)
	}
}

func TestSimpleChecker_Error(t *testing.T) {
	checker := NewSimpleChecker("test", func() error {
		return errors.New("test error")
	})

	check := checker.Check()

	if check.Status != StatusUnhealthy {
		t.Errorf("expected status unhealthy, got %s", check.Status)
	}

	if check.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", check.Message)
	}
}
