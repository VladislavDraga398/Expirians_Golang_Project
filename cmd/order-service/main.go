package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/app"
)

// setupLogger настраивает формат и уровень логирования для сервиса.
func setupLogger() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetLevel(log.InfoLevel)
}

// readConfig формирует конфигурацию приложения, позволяя переопределить адреса через переменные окружения.
func readConfig() app.Config {
	cfg := app.DefaultConfig()
	if v := os.Getenv("OMS_GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
	}
	if v := os.Getenv("OMS_METRICS_ADDR"); v != "" {
		cfg.MetricsAddr = v
	}
	return cfg
}

func main() {
	setupLogger()
	cfg := readConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.WithFields(log.Fields{
		"grpc_addr":    cfg.GRPCAddr,
		"metrics_addr": cfg.MetricsAddr,
	}).Info("запускаем OrderService")

	if err := app.Run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		log.WithError(err).Fatal("приложение завершилось с ошибкой")
	}

	log.Info("OrderService остановлен")
}
