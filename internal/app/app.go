package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	promgrpc "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	grpcsvc "github.com/vladislavdragonenkov/oms/internal/service/grpc"
	"github.com/vladislavdragonenkov/oms/internal/service/inventory"
	"github.com/vladislavdragonenkov/oms/internal/service/payment"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

// Config описывает минимальные настройки запуска приложения.
type Config struct {
	GRPCAddr    string
	MetricsAddr string
}

// DefaultConfig возвращает базовые адреса для gRPC и HTTP-метрик.
func DefaultConfig() Config {
	return Config{
		GRPCAddr:    ":50051",
		MetricsAddr: ":9090",
	}
}

func Run(ctx context.Context, cfg Config) error {
	logger := log.WithField("component", "app")
	repo := memory.NewOrderRepository()
	outboxRepo := memory.NewOutboxRepository()
	timelineRepo := memory.NewTimelineRepository()
	inventorySvc := inventory.NewMockService()
	paymentSvc := payment.NewMockService()
	// Allow forcing failures for testing metrics via env flags
	failReserve := os.Getenv("OMS_FAIL_RESERVE")
	failPay := os.Getenv("OMS_FAIL_PAY")
	logger.Errorf("DEBUG: Failure flags: OMS_FAIL_RESERVE=%s, OMS_FAIL_PAY=%s", failReserve, failPay)
	
	if failReserve == "true" {
		inventorySvc.ReserveErr = errors.New("forced reserve fail")
		logger.Error("TESTING: Reserve failures enabled")
	}
	if failPay == "true" {
		paymentSvc.PayErr = errors.New("forced pay fail")
		logger.Error("TESTING: Payment failures enabled")
	}
	sagaOrchestrator := saga.NewOrchestrator(
		repo,
		outboxRepo,
		timelineRepo,
		inventorySvc,
		paymentSvc,
		logger,
	)

	serviceLogger := logger.WithField("layer", "grpc")
	orderService := grpcsvc.NewOrderService(repo, timelineRepo, sagaOrchestrator, serviceLogger)
	grpcMetrics := promgrpc.NewServerMetrics()
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcMetrics.UnaryServerInterceptor()))
	if err := prometheus.Register(grpcMetrics); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok2 := are.ExistingCollector.(*promgrpc.ServerMetrics); ok2 {
				grpcMetrics = existing
			}
		} else {
			logger.WithError(err).Warn("failed to register grpc metrics")
		}
	}

omsv1.RegisterOrderServiceServer(grpcServer, orderService)
	grpcMetrics.InitializeMetrics(grpcServer)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	metricsSrv := startMetricsServer(ctx, cfg.MetricsAddr, logger)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Infof("gRPC сервер слушает %s", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		logger.Info("получен сигнал остановки, останавливаем gRPC сервер")
		stoppedCh := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
			close(stoppedCh)
		}()
		select {
		case <-stoppedCh:
		case <-time.After(5 * time.Second):
			logger.Warn("graceful stop превысил таймаут, принудительно останавливаем")
			grpcServer.Stop()
		}
		shutdownHTTP(metricsSrv, logger)
		return ctx.Err()
	case err := <-errCh:
		shutdownHTTP(metricsSrv, logger)
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
}

// startMetricsServer запускает HTTP-обработчик /metrics для Prometheus.
func startMetricsServer(ctx context.Context, addr string, logger *log.Entry) *http.Server {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		logger.Infof("метрики доступны по адресу %s/metrics", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WithError(err).Warn("metrics server failed")
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownHTTP(srv, logger)
	}()

	return srv
}

// shutdownHTTP аккуратно останавливает HTTP-сервер.
func shutdownHTTP(srv *http.Server, logger *log.Entry) {
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.WithError(err).Warn("metrics shutdown with error")
	}
}
