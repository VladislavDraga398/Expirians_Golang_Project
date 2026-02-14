package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/app"
	"github.com/vladislavdragonenkov/oms/internal/version"
)

const (
	envLogLevel            = "LOG_LEVEL"
	envGRPCAddr            = "OMS_GRPC_ADDR"
	envMetricsAddr         = "OMS_METRICS_ADDR"
	envStorageDriver       = "OMS_STORAGE_DRIVER"
	envPostgresDSN         = "OMS_POSTGRES_DSN"
	envPostgresAutoMigrate = "OMS_POSTGRES_AUTO_MIGRATE"
	envOutboxPollInterval  = "OMS_OUTBOX_POLL_INTERVAL"
	envOutboxBatchSize     = "OMS_OUTBOX_BATCH_SIZE"
	envOutboxMaxAttempts   = "OMS_OUTBOX_MAX_ATTEMPTS"
	envOutboxRetryDelay    = "OMS_OUTBOX_RETRY_DELAY"
	envOutboxMaxPending    = "OMS_OUTBOX_MAX_PENDING"
)

type configWarning struct {
	env   string
	value string
	err   error
}

type envLookup func(string) (string, bool)

// setupLogger настраивает формат и уровень логирования для сервиса.
func setupLogger() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetLevel(log.InfoLevel)

	levelRaw, ok := lookupEnvTrimmed(os.LookupEnv, envLogLevel)
	if !ok {
		return
	}

	level, err := log.ParseLevel(strings.ToLower(levelRaw))
	if err != nil {
		log.WithError(err).WithField("env", envLogLevel).WithField("value", levelRaw).Warn("invalid log level, using info")
		return
	}

	log.SetLevel(level)
}

// readConfig формирует конфигурацию приложения, позволяя переопределить адреса через переменные окружения.
func readConfig() app.Config {
	cfg, warnings := readConfigFromEnv(os.LookupEnv)
	for _, warning := range warnings {
		log.WithError(warning.err).WithFields(log.Fields{
			"env":   warning.env,
			"value": warning.value,
		}).Warn("invalid configuration value, using default")
	}
	return cfg
}

func readConfigFromEnv(lookup envLookup) (app.Config, []configWarning) {
	cfg := app.DefaultConfig()

	if v, ok := lookupEnvTrimmed(lookup, envGRPCAddr); ok {
		cfg.GRPCAddr = v
	}
	if v, ok := lookupEnvTrimmed(lookup, envMetricsAddr); ok {
		cfg.MetricsAddr = v
	}
	if v, ok := lookupEnvTrimmed(lookup, envStorageDriver); ok {
		cfg.StorageDriver = strings.ToLower(v)
	}
	if v, ok := lookupEnvTrimmed(lookup, envPostgresDSN); ok {
		cfg.PostgresDSN = v
	}

	var warnings []configWarning

	if raw, ok := lookupEnvTrimmed(lookup, envPostgresAutoMigrate); ok {
		value, err := parseBool(raw)
		if err != nil {
			warnings = append(warnings, configWarning{env: envPostgresAutoMigrate, value: raw, err: err})
		} else {
			cfg.PostgresAutoMigrate = value
		}
	}

	if raw, ok := lookupEnvTrimmed(lookup, envOutboxPollInterval); ok {
		value, err := parseDuration(raw, func(d time.Duration) bool { return d > 0 }, "must be > 0")
		if err != nil {
			warnings = append(warnings, configWarning{env: envOutboxPollInterval, value: raw, err: err})
		} else {
			cfg.OutboxPollInterval = value
		}
	}

	if raw, ok := lookupEnvTrimmed(lookup, envOutboxBatchSize); ok {
		value, err := parseInt(raw, func(v int) bool { return v > 0 }, "must be > 0")
		if err != nil {
			warnings = append(warnings, configWarning{env: envOutboxBatchSize, value: raw, err: err})
		} else {
			cfg.OutboxBatchSize = value
		}
	}

	if raw, ok := lookupEnvTrimmed(lookup, envOutboxMaxAttempts); ok {
		value, err := parseInt(raw, func(v int) bool { return v > 0 }, "must be > 0")
		if err != nil {
			warnings = append(warnings, configWarning{env: envOutboxMaxAttempts, value: raw, err: err})
		} else {
			cfg.OutboxMaxAttempts = value
		}
	}

	if raw, ok := lookupEnvTrimmed(lookup, envOutboxRetryDelay); ok {
		value, err := parseDuration(raw, func(d time.Duration) bool { return d >= 0 }, "must be >= 0")
		if err != nil {
			warnings = append(warnings, configWarning{env: envOutboxRetryDelay, value: raw, err: err})
		} else {
			cfg.OutboxRetryDelay = value
		}
	}

	if raw, ok := lookupEnvTrimmed(lookup, envOutboxMaxPending); ok {
		value, err := parseInt(raw, func(v int) bool { return v >= 0 }, "must be >= 0")
		if err != nil {
			warnings = append(warnings, configWarning{env: envOutboxMaxPending, value: raw, err: err})
		} else {
			cfg.OutboxMaxPending = value
		}
	}

	return cfg, warnings
}

func lookupEnvTrimmed(lookup envLookup, key string) (string, bool) {
	value, ok := lookup(key)
	if !ok {
		return "", false
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}

	return trimmed, true
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value")
	}
}

func parseInt(value string, validate func(int) bool, constraints string) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	if !validate(number) {
		return 0, fmt.Errorf("invalid integer value: %s", constraints)
	}
	return number, nil
}

func parseDuration(value string, validate func(time.Duration) bool, constraints string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	if !validate(duration) {
		return 0, fmt.Errorf("invalid duration value: %s", constraints)
	}
	return duration, nil
}

func main() {
	setupLogger()
	cfg := readConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.WithFields(log.Fields{
		"grpc_addr":             cfg.GRPCAddr,
		"metrics_addr":          cfg.MetricsAddr,
		"storage_driver":        cfg.StorageDriver,
		"postgres_auto_migrate": cfg.PostgresAutoMigrate,
		"outbox_poll_interval":  cfg.OutboxPollInterval.String(),
		"outbox_batch_size":     cfg.OutboxBatchSize,
		"outbox_max_attempts":   cfg.OutboxMaxAttempts,
		"outbox_retry_delay":    cfg.OutboxRetryDelay.String(),
		"outbox_max_pending":    cfg.OutboxMaxPending,
		"build":                 version.String(),
	}).Info("запускаем OrderService")

	if err := app.Run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		log.WithError(err).Fatal("приложение завершилось с ошибкой")
	}
}
