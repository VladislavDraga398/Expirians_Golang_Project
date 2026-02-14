package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	healthcheck "github.com/vladislavdragonenkov/oms/internal/health"
	"github.com/vladislavdragonenkov/oms/internal/version"
)

func TestStartMetricsServer_Endpoints(t *testing.T) {
	logger := log.WithField("test", "http")

	// Используем свободный порт
	port := findFreePort(t)
	addr := fmt.Sprintf(":%d", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthHandler := healthcheck.NewHandler(version.GetVersion())
	srv := startMetricsServer(ctx, addr, logger, healthHandler)

	// Даём время на запуск
	time.Sleep(100 * time.Millisecond)

	// Проверяем /metrics
	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", port)
	resp, err := http.Get(metricsURL)
	if err != nil {
		t.Fatalf("failed to get /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /metrics, got %d", resp.StatusCode)
	}

	// Проверяем что это Prometheus метрики
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if len(bodyStr) == 0 {
		t.Error("/metrics should return non-empty response")
	}

	// Проверяем /healthz
	healthURL := fmt.Sprintf("http://localhost:%d/healthz", port)
	resp2, err := http.Get(healthURL)
	if err != nil {
		t.Fatalf("failed to get /healthz: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /healthz, got %d", resp2.StatusCode)
	}

	// Проверяем /livez
	livezURL := fmt.Sprintf("http://localhost:%d/livez", port)
	resp3, err := http.Get(livezURL)
	if err != nil {
		t.Fatalf("failed to get /livez: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /livez, got %d", resp3.StatusCode)
	}

	body3, _ := io.ReadAll(resp3.Body)
	if string(body3) != "ok" {
		t.Errorf("expected 'ok' from /livez, got '%s'", string(body3))
	}

	// Проверяем /readyz
	readyzURL := fmt.Sprintf("http://localhost:%d/readyz", port)
	resp4, err := http.Get(readyzURL)
	if err != nil {
		t.Fatalf("failed to get /readyz: %v", err)
	}
	defer resp4.Body.Close()

	if resp4.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /readyz, got %d", resp4.StatusCode)
	}

	// Cleanup
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Verify server is not nil
	if srv == nil {
		t.Error("startMetricsServer should not return nil")
	}
}

func TestStartMetricsServer_Shutdown(t *testing.T) {
	logger := log.WithField("test", "http-shutdown")

	port := findFreePort(t)
	addr := fmt.Sprintf(":%d", port)

	ctx, cancel := context.WithCancel(context.Background())

	healthHandler := healthcheck.NewHandler(version.GetVersion())
	srv := startMetricsServer(ctx, addr, logger, healthHandler)

	// Даём время на запуск
	time.Sleep(100 * time.Millisecond)

	// Проверяем что сервер работает
	url := fmt.Sprintf("http://localhost:%d/livez", port)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("server should be running: %v", err)
	}
	resp.Body.Close()

	// Отменяем контекст
	cancel()

	// Даём время на shutdown
	time.Sleep(200 * time.Millisecond)

	// Проверяем что сервер остановился
	_, err = http.Get(url)
	if err == nil {
		t.Error("server should be stopped after context cancellation")
	}

	if srv == nil {
		t.Error("startMetricsServer should not return nil")
	}
}

func TestShutdownHTTP_NilServer(_ *testing.T) {
	logger := log.WithField("test", "http-nil")

	// Не должно паниковать
	shutdownHTTP(nil, logger)
}

func TestShutdownHTTP_WithServer(t *testing.T) {
	logger := log.WithField("test", "http-shutdown-func")

	port := findFreePort(t)
	addr := fmt.Sprintf(":%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
	})

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		_ = srv.ListenAndServe()
	}()

	// Даём время на запуск
	time.Sleep(100 * time.Millisecond)

	// Проверяем что работает
	url := fmt.Sprintf("http://localhost:%d/test", port)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("server should be running: %v", err)
	}
	resp.Body.Close()

	// Останавливаем
	shutdownHTTP(srv, logger)

	// Проверяем что остановился
	time.Sleep(100 * time.Millisecond)
	_, err = http.Get(url)
	if err == nil {
		t.Error("server should be stopped after shutdownHTTP")
	}
}

func TestStartMetricsServer_InvalidAddr(t *testing.T) {
	logger := log.WithField("test", "http-invalid")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Используем занятый порт
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf(":%d", port)

	healthHandler := healthcheck.NewHandler(version.GetVersion())

	// Сервер всё равно создаётся, но не может стартовать
	srv := startMetricsServer(ctx, addr, logger, healthHandler)

	if srv == nil {
		t.Error("startMetricsServer should not return nil even with invalid addr")
	}

	listener.Close()
}

func TestStartMetricsServer_MultipleEndpoints(t *testing.T) {
	logger := log.WithField("test", "http-multiple")

	port := findFreePort(t)
	addr := fmt.Sprintf(":%d", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthHandler := healthcheck.NewHandler(version.GetVersion())
	srv := startMetricsServer(ctx, addr, logger, healthHandler)

	time.Sleep(100 * time.Millisecond)

	// Проверяем все endpoints
	endpoints := []string{
		fmt.Sprintf("http://localhost:%d/metrics", port),
		fmt.Sprintf("http://localhost:%d/healthz", port),
		fmt.Sprintf("http://localhost:%d/livez", port),
		fmt.Sprintf("http://localhost:%d/readyz", port),
	}

	for _, url := range endpoints {
		resp, err := http.Get(url)
		if err != nil {
			t.Errorf("failed to get %s: %v", url, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s returned status %d, expected 200", url, resp.StatusCode)
		}
	}

	if srv == nil {
		t.Error("server should not be nil")
	}
}

// findFreePort находит свободный порт для тестов
func findFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}
