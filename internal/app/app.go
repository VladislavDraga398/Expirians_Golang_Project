package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	promgrpc "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	healthcheck "github.com/vladislavdragonenkov/oms/internal/health"
	"github.com/vladislavdragonenkov/oms/internal/messaging/kafka"
	grpcsvc "github.com/vladislavdragonenkov/oms/internal/service/grpc"
	"github.com/vladislavdragonenkov/oms/internal/service/inventory"
	outboxsvc "github.com/vladislavdragonenkov/oms/internal/service/outbox"
	"github.com/vladislavdragonenkov/oms/internal/service/payment"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
	"github.com/vladislavdragonenkov/oms/internal/storage/postgres"
	"github.com/vladislavdragonenkov/oms/internal/version"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

const (
	StorageDriverMemory   = "memory"
	StorageDriverPostgres = "postgres"

	storagePingTimeout      = 2 * time.Second
	gracefulShutdownTimeout = 5 * time.Second
)

// Config описывает минимальные настройки запуска приложения.
type Config struct {
	GRPCAddr            string
	MetricsAddr         string
	StorageDriver       string
	PostgresDSN         string
	PostgresAutoMigrate bool
	OutboxPollInterval  time.Duration
	OutboxBatchSize     int
	OutboxMaxAttempts   int
	OutboxRetryDelay    time.Duration
	OutboxMaxPending    int
}

// DefaultConfig возвращает базовые адреса для gRPC и HTTP-метрик.
func DefaultConfig() Config {
	return Config{
		GRPCAddr:            ":50051",
		MetricsAddr:         ":9090",
		StorageDriver:       StorageDriverMemory,
		PostgresAutoMigrate: true,
		OutboxPollInterval:  time.Second,
		OutboxBatchSize:     100,
		OutboxMaxAttempts:   3,
		OutboxRetryDelay:    50 * time.Millisecond,
		OutboxMaxPending:    10000,
	}
}

func Run(ctx context.Context, cfg Config) error {
	logger := log.WithField("component", "app")

	runtimeDeps, err := initRuntimeDependencies(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if runtimeDeps.closeFn == nil {
			return
		}
		if closeErr := runtimeDeps.closeFn(); closeErr != nil {
			logger.WithError(closeErr).Warn("failed to close storage")
		}
	}()

	repo := runtimeDeps.repo
	outboxRepo := runtimeDeps.outboxRepo
	timelineRepo := runtimeDeps.timelineRepo

	// Для локальной разработки используются mock-сервисы.
	inventorySvc := inventory.NewMockService()
	paymentSvc := payment.NewMockService()

	// Kafka producer опционален: если брокер недоступен, сервис продолжает работу.
	var kafkaProducer *kafka.Producer
	var outboxWorkerCancel context.CancelFunc
	var outboxWorkerDone chan struct{}
	var outboxChecker healthcheck.Checker
	var sagaOrchestrator saga.Orchestrator

	rawKafkaBrokers := os.Getenv("KAFKA_BROKERS")
	brokers := parseKafkaBrokers(rawKafkaBrokers)
	if strings.TrimSpace(rawKafkaBrokers) != "" && len(brokers) == 0 {
		logger.Warn("KAFKA_BROKERS is set but no valid broker addresses were parsed")
	}
	if len(brokers) > 0 {
		producer, err := kafka.NewProducer(brokers)
		if err != nil {
			logger.WithError(err).Warn("failed to create kafka producer, continuing without kafka")
		} else {
			kafkaProducer = producer
			logger.WithField("brokers", brokers).Info("kafka producer initialized")

			outboxWorker := outboxsvc.NewWorker(
				outboxRepo,
				kafka.NewOutboxPublisher(kafkaProducer, kafka.TopicOrderEvents),
				outboxsvc.WithDLQPublisher(kafka.NewOutboxPublisher(kafkaProducer, kafka.TopicDeadLetterQueue)),
				outboxsvc.WithLogger(logger.WithField("component", "outbox-worker")),
				outboxsvc.WithPollInterval(cfg.OutboxPollInterval),
				outboxsvc.WithBatchSize(cfg.OutboxBatchSize),
				outboxsvc.WithMaxAttempts(cfg.OutboxMaxAttempts),
				outboxsvc.WithRetryBaseDelay(cfg.OutboxRetryDelay),
			)
			workerCtx, workerCancel := context.WithCancel(ctx)
			outboxWorkerCancel = workerCancel
			outboxWorkerDone = make(chan struct{})
			go func() {
				defer close(outboxWorkerDone)
				outboxWorker.Run(workerCtx)
			}()

			outboxChecker = healthcheck.NewSimpleChecker("outbox", func() error {
				stats, err := outboxRepo.Stats()
				if err != nil {
					return err
				}
				if cfg.OutboxMaxPending > 0 && stats.PendingCount > cfg.OutboxMaxPending {
					return fmt.Errorf("outbox backlog %d exceeds threshold %d", stats.PendingCount, cfg.OutboxMaxPending)
				}
				return nil
			})

			// Создаём orchestrator с Kafka
			sagaOrchestrator = saga.NewOrchestratorWithKafka(
				repo,
				outboxRepo,
				timelineRepo,
				inventorySvc,
				paymentSvc,
				kafkaProducer,
				logger,
			)
		}
	}

	// Если Kafka не настроен, используем обычный orchestrator
	if sagaOrchestrator == nil {
		sagaOrchestrator = saga.NewOrchestrator(
			repo,
			outboxRepo,
			timelineRepo,
			inventorySvc,
			paymentSvc,
			logger,
		)
	}

	serviceLogger := logger.WithField("layer", "grpc")
	orderService := grpcsvc.NewOrderService(repo, timelineRepo, runtimeDeps.idempotencyRepo, sagaOrchestrator, serviceLogger)
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

	// Register reflection service for grpcurl and load testing tools
	reflection.Register(grpcServer)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	// HTTP Health checks
	healthHandler := healthcheck.NewHandler(version.GetVersion())
	if runtimeDeps.storageChecker != nil {
		healthHandler.RegisterChecker("storage", runtimeDeps.storageChecker)
	}
	if outboxChecker != nil {
		healthHandler.RegisterChecker("outbox", outboxChecker)
	}

	metricsSrv := startMetricsServer(ctx, cfg.MetricsAddr, logger, healthHandler)

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
		case <-time.After(gracefulShutdownTimeout):
			logger.Warn("graceful stop превысил таймаут, принудительно останавливаем")
			grpcServer.Stop()
		}
		shutdownOrderService(orderService, logger)
		shutdownHTTP(metricsSrv, logger)
		shutdownOutboxWorker(outboxWorkerCancel, outboxWorkerDone, logger)

		closeKafkaProducer(kafkaProducer, logger)

		return ctx.Err()
	case err := <-errCh:
		shutdownOrderService(orderService, logger)
		shutdownHTTP(metricsSrv, logger)
		shutdownOutboxWorker(outboxWorkerCancel, outboxWorkerDone, logger)
		closeKafkaProducer(kafkaProducer, logger)

		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
}

type runtimeDependencies struct {
	repo            domain.OrderRepository
	outboxRepo      domain.OutboxRepository
	timelineRepo    domain.TimelineRepository
	idempotencyRepo domain.IdempotencyRepository
	storageChecker  healthcheck.Checker
	closeFn         func() error
}

func initRuntimeDependencies(ctx context.Context, cfg Config, logger *log.Entry) (runtimeDependencies, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.StorageDriver))
	if driver == "" {
		driver = StorageDriverMemory
	}

	switch driver {
	case StorageDriverMemory:
		return runtimeDependencies{
			repo:            memory.NewOrderRepository(),
			outboxRepo:      memory.NewOutboxRepository(),
			timelineRepo:    memory.NewTimelineRepository(),
			idempotencyRepo: memory.NewIdempotencyRepository(),
		}, nil
	case StorageDriverPostgres:
		if strings.TrimSpace(cfg.PostgresDSN) == "" {
			return runtimeDependencies{}, fmt.Errorf("OMS_POSTGRES_DSN is required for postgres storage driver")
		}

		store, err := postgres.Open(ctx, cfg.PostgresDSN)
		if err != nil {
			return runtimeDependencies{}, fmt.Errorf("init postgres store: %w", err)
		}

		if cfg.PostgresAutoMigrate {
			if err := store.EnsureSchema(ctx); err != nil {
				_ = store.Close()
				return runtimeDependencies{}, fmt.Errorf("apply postgres schema: %w", err)
			}
		}

		checker := healthcheck.NewSimpleChecker("postgres", func() error {
			pingCtx, cancel := context.WithTimeout(context.Background(), storagePingTimeout)
			defer cancel()
			return store.Ping(pingCtx)
		})

		logger.Info("postgres storage initialized")

		return runtimeDependencies{
			repo:            postgres.NewOrderRepository(store),
			outboxRepo:      postgres.NewOutboxRepository(store),
			timelineRepo:    postgres.NewTimelineRepository(store),
			idempotencyRepo: postgres.NewIdempotencyRepository(store),
			storageChecker:  checker,
			closeFn:         store.Close,
		}, nil
	default:
		return runtimeDependencies{}, fmt.Errorf("unsupported storage driver: %s", driver)
	}
}

// startMetricsServer запускает HTTP-обработчик /metrics для Prometheus.
func startMetricsServer(ctx context.Context, addr string, logger *log.Entry, healthHandler http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/healthz", healthHandler)
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if handler, ok := healthHandler.(*healthcheck.Handler); ok {
		mux.HandleFunc("/readyz", handler.ReadinessHandler)
	}

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		logger.Infof("метрики доступны по адресу %s/metrics", addr)
		logger.Infof("health checks: %s/healthz, %s/livez, %s/readyz", addr, addr, addr)
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
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.WithError(err).Warn("metrics shutdown with error")
	}
}

func shutdownOrderService(orderService *grpcsvc.OrderService, logger *log.Entry) {
	if orderService == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	if err := orderService.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logger.WithError(err).Warn("order service shutdown with error")
	}
}

func shutdownOutboxWorker(cancel context.CancelFunc, done <-chan struct{}, logger *log.Entry) {
	if cancel == nil || done == nil {
		return
	}

	cancel()

	select {
	case <-done:
	case <-time.After(gracefulShutdownTimeout):
		logger.Warn("outbox worker shutdown timeout")
	}
}

func parseKafkaBrokers(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	chunks := strings.Split(raw, ",")
	brokers := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		broker := strings.TrimSpace(chunk)
		if broker == "" {
			continue
		}
		brokers = append(brokers, broker)
	}

	return brokers
}

func closeKafkaProducer(producer *kafka.Producer, logger *log.Entry) {
	if producer == nil {
		return
	}

	if err := producer.Close(); err != nil {
		logger.WithError(err).Warn("failed to close kafka producer")
		return
	}

	logger.Info("kafka producer closed")
}
