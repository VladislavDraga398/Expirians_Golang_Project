# OMS - Order Management System

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)]()
[![Coverage Policy](https://img.shields.io/badge/coverage%20policy-80%25-brightgreen.svg)]()

**Модульный монолит OMS** с надёжным ядром заказа и roadmap расширения в delivery-продукт **BoostMarket**.

## Статус проекта

- **Версия:** v3.0
- **Статус:** Sprint 2 Active (Delivery Domain Foundation)
- **Последнее обновление:** 2026-02-23

## Ключевые возможности

- **Saga Orchestrator** - Reserve → Pay → Confirm с компенсациями
- **Event-Driven Architecture** - Apache Kafka для асинхронных событий
- **Transactional Outbox** - гарантированная доставка событий
- **Full Observability** - Prometheus метрики + Grafana дашборды
- **Graceful Shutdown** - контролируемое завершение gRPC/HTTP и фоновых saga-задач
- **Race-free код** - тесты проходят с `-race` флагом
- **Dead Letter Queue** - обработка failed Kafka messages
- **Retry логика** - exponential backoff для version conflicts
- **Timeline события** - audit trail для каждого заказа

## Архитектура

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   gRPC API  │─────│     Saga     │─────│  Inventory  │
│             │      │ Orchestrator │      │   Service   │
└─────────────┘      └──────────────┘      └─────────────┘
                            │
                            ├────────────── Payment Service
                            │
                            ├────────────── Kafka Producer
                            │
                            └────────────── Transactional Outbox
```

**Стек технологий:**
- Go 1.24+
- gRPC + Protobuf
- Apache Kafka 7.5.0
- Prometheus + Grafana
- Docker Compose

## Быстрый старт

### Предварительные требования

- Go 1.24+
- Docker & Docker Compose
- Make
- grpcurl (опционально)

### Установка

```bash
# Клонировать репозиторий
git clone https://github.com/VladislavDraga398/Expirians_Golang_Project.git
cd Expirians_Golang_Project

# Установить зависимости
make deps

# Запустить инфраструктуру (Kafka, Prometheus, Grafana)
make compose-up

# Дождаться готовности сервисов
make wait-health
```

### Запуск сервиса

```bash
# Вариант 1: Локально
make run

# Вариант 2: В Docker
make docker-build
make docker-run

# Вариант 3: Полное демо с тестовыми сценариями
make demo
```

Если используется `OMS_STORAGE_DRIVER=postgres`, сейчас требуется `OMS_ALLOW_MOCK_INTEGRATIONS=true` (реальные Inventory/Payment интеграции пока не внедрены в runtime).

## Тестирование

### Базовые команды

```bash
# Запустить все тесты
make test

# Тесты с race detector (ВАЖНО!)
make test-race

# Coverage отчёт
make cover

# Тесты по компонентам
make test-saga      # Saga orchestrator
make test-kafka     # Kafka integration
make test-grpc      # gRPC service
```

### Специальные режимы

```bash
# Быстрые тесты
make test-short

# Тесты 10 раз (проверка стабильности)
make test-count

# Остановить при первой ошибке
make test-failfast

# Бенчмарки
make bench
```

Централизованные скрипты запуска: `test/run/all.sh`, `test/run/unit.sh`, `test/run/integration.sh`, `test/run/race.sh`.

Полный список команд: `make help`

### Controlled replay из DLQ

```bash
# Dry-run (ничего не публикует, только показывает кандидатов)
make dlq-reprocess LIMIT=50

# Реальный replay последних сообщений
make dlq-reprocess LIMIT=50 EXECUTE=1 FROM_NEWEST=1
```

## API Примеры

### CreateOrder

```bash
grpcurl -plaintext -H 'idempotency-key: readme-create-001' -d '{
  "customer_id": "customer-123",
  "currency": "USD",
  "items": [{
    "sku": "SKU-001",
    "qty": 2,
    "price": {"currency": "USD", "amount_minor": 10000}
  }]
}' localhost:50051 oms.v1.OrderService/CreateOrder
```

### PayOrder

```bash
grpcurl -plaintext -H 'idempotency-key: readme-pay-001' -d '{
  "order_id": "order-123"
}' localhost:50051 oms.v1.OrderService/PayOrder
```

### GetOrder

```bash
grpcurl -plaintext -d '{
  "order_id": "order-123"
}' localhost:50051 oms.v1.OrderService/GetOrder
```

Больше примеров: [docs/guides/api-examples.md](docs/guides/api-examples.md)

## Мониторинг

После запуска `make demo` доступны:

- **Prometheus:** http://localhost:9091
- **Grafana:** http://localhost:3000 (`GRAFANA_ADMIN_USER`/`GRAFANA_ADMIN_PASSWORD`, по умолчанию `admin/admin`)
  - Dashboard: OMS → OMS Saga Overview
- **Kafka UI:** http://localhost:8080

Порты стенда по умолчанию публикуются только на `127.0.0.1` (настраивается через `HOST_BIND_ADDR`).

### Ключевые метрики

- `oms_saga_started_total` - запущенные саги
- `oms_saga_completed_total` - успешные саги
- `oms_saga_failed_total` - проваленные саги
- `oms_saga_duration_seconds` - длительность саги
- `oms_active_sagas` - активные саги

## Разработка

### Структура проекта

```
.
├── cmd/order-service/      # Entry point
├── internal/
│   ├── app/                # Application setup
│   ├── domain/             # Domain models
│   ├── messaging/kafka/    # Kafka integration
│   ├── metrics/            # Prometheus metrics
│   ├── service/            # Business logic
│   │   ├── grpc/           # gRPC handlers
│   │   ├── saga/           # Saga orchestrator
│   │   ├── inventory/      # Inventory service
│   │   └── payment/        # Payment service
│   └── storage/            # Data access
├── proto/oms/v1/           # Protobuf definitions
├── test/integration/       # Integration tests
├── deploy/                 # Docker, Grafana configs
├── docs/                   # Documentation
└── scripts/                # Demo scripts
```

### Workflow разработки

```bash
# 1. Создать ветку
git checkout -b feature/my-feature

# 2. Внести изменения
# ...

# 3. Форматирование
make fmt

# 4. Проверки перед коммитом
make test-race
make lint

# 5. Коммит (pre-commit hook запустится автоматически)
git add .
git commit -m "feat: add new feature"
```

### Веточная политика

- Основной поток: `feature/* -> dev -> main`.
- Временный тестовый стенд живет в CI (GitHub Actions), а не в отдельной git-ветке.
- Для каждого PR запускается `premerge_stand`: PR в `dev` использует быстрый dev-профиль, PR в `main`/`master` использует усиленный release-профиль.
- В `premerge_stand` load gate проверяет бизнес-поток `create-pay-cancel`, а не только `create`.
- Merge разрешен только после успешных checks (`Lint`, `Tests`, `Build`, `Pre-Merge Stand`).

### Pre-commit hook

Автоматически проверяет:
- Форматирование (gofmt)
- Статический анализ (go vet)
- Race conditions (go test -race)
- Линтинг (golangci-lint)
- TODO/FIXME комментарии
- Debug print statements

Установка:
```bash
git config core.hooksPath .githooks
```

## Документация

- **Единая точка входа:** [Technical Documentation Hub](docs/TECHDOCS.md)
- [Documentation Index](docs/INDEX.md)
- [Quick Start Guide](docs/quick-start.md)

## Roadmap

Текущий программный статус:
- Sprint 1 (`Core Hardening`) завершён: стабилизировано runtime-ядро OMS, усилены outbox/idempotency/CI.
- Sprint 2 (`Delivery Domain Foundation`) в работе: курьеры, зоны Москвы, транспорт, слоты и ограничения ёмкости.
- Дальше по плану: рейтинг курьеров, dispatch-алгоритм, динамическое ценообразование, внешние geo/weather/traffic API.

Детали: [docs/roadmap.md](docs/roadmap.md)

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

**Vladislav Dragonenkov**

- GitHub: [@vladislavdragonenkov](https://github.com/vladislavdragonenkov)

## Acknowledgments

- Saga Pattern inspiration from [Microservices Patterns](https://microservices.io/patterns/data/saga.html)
- Event-Driven Architecture best practices
- Go community for excellent tooling

---

** Если проект полезен, поставьте звезду!**
