# 🛒 OMS - Order Management System

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)]()
[![Coverage](https://img.shields.io/badge/coverage-44%25-yellow.svg)]()

**Production-ready микросервис** для управления заказами с реализацией **Saga Pattern** и **Event-Driven Architecture** через Apache Kafka.

## 📊 Статус проекта

- **Версия:** v2.0
- **Статус:** Phase 4 Complete (85% Production Ready)
- **Последнее обновление:** 2025-10-01

## ✨ Ключевые возможности

- ✅ **Saga Orchestrator** - Reserve → Pay → Confirm с компенсациями
- ✅ **Event-Driven Architecture** - Apache Kafka для асинхронных событий
- ✅ **Transactional Outbox** - гарантированная доставка событий
- ✅ **Full Observability** - Prometheus метрики + Grafana дашборды
- ✅ **Race-free код** - все тесты проходят с `-race` флагом
- ✅ **Dead Letter Queue** - обработка failed Kafka messages
- ✅ **Retry логика** - exponential backoff для version conflicts
- ✅ **Timeline события** - audit trail для каждого заказа

## 🏗️ Архитектура

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   gRPC API  │─────▶│     Saga     │─────▶│  Inventory  │
│             │      │ Orchestrator │      │   Service   │
└─────────────┘      └──────────────┘      └─────────────┘
                            │
                            ├──────────────▶ Payment Service
                            │
                            ├──────────────▶ Kafka Producer
                            │
                            └──────────────▶ Transactional Outbox
```

**Стек технологий:**
- Go 1.21+
- gRPC + Protobuf
- Apache Kafka 7.5.0
- Prometheus + Grafana
- Docker Compose

## 🚀 Быстрый старт

### Предварительные требования

- Go 1.21+
- Docker & Docker Compose
- Make
- grpcurl (опционально)

### Установка

```bash
# Клонировать репозиторий
git clone https://github.com/vladislavdragonenkov/oms.git
cd oms

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

## 🧪 Тестирование

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

Полный список команд: `make help`

## 📖 API Примеры

### CreateOrder

```bash
grpcurl -plaintext -d '{
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
grpcurl -plaintext -d '{
  "order_id": "order-123"
}' localhost:50051 oms.v1.OrderService/PayOrder
```

### GetOrder

```bash
grpcurl -plaintext -d '{
  "order_id": "order-123"
}' localhost:50051 oms.v1.OrderService/GetOrder
```

Больше примеров: [docs/API_EXAMPLES.md](docs/API_EXAMPLES.md)

## 📊 Мониторинг

После запуска `make demo` доступны:

- **Prometheus:** http://localhost:9091
- **Grafana:** http://localhost:3000 (admin/admin)
  - Dashboard: OMS → OMS Saga Overview
- **Kafka UI:** http://localhost:8080

### Ключевые метрики

- `oms_saga_started_total` - запущенные саги
- `oms_saga_completed_total` - успешные саги
- `oms_saga_failed_total` - проваленные саги
- `oms_saga_duration_seconds` - длительность саги
- `oms_active_sagas` - активные саги

## 🔧 Разработка

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

### Pre-commit hook

Автоматически проверяет:
- ✅ Форматирование (gofmt)
- ✅ Статический анализ (go vet)
- ✅ Race conditions (go test -race)
- ✅ Линтинг (golangci-lint)
- ✅ TODO/FIXME комментарии
- ✅ Debug print statements

Установка:
```bash
git config core.hooksPath .githooks
```

## 📚 Документация

### 🚀 Начало работы
- **[Quick Start Guide](docs/quick-start.md)** - запустите за 5 минут ⚡
- **[Documentation Index](docs/INDEX.md)** - полный индекс документации 📖

### 📖 Популярные руководства
- [API Examples](docs/guides/api-examples.md) - примеры использования API
- [Makefile Guide](docs/guides/makefile.md) - все команды для разработки
- [CI/CD Pipeline](docs/guides/ci-cd.md) - автоматизация и деплой

### 🏗️ Архитектура
- [Architecture Overview](docs/architecture/overview.md) - общая архитектура
- [Saga Pattern](docs/architecture/saga.md) - распределённые транзакции
- [Kafka Integration](docs/guides/kafka.md) - Event-Driven Architecture

### 🔧 Deployment
- [Kubernetes Guide](deploy/k8s/README.md) - деплой в K8s
- [Helm Chart](deploy/helm/oms/README.md) - Helm guide

### 📝 Дополнительно
- [Roadmap](docs/roadmap.md) - план развития
- [ADR Index](docs/decisions/adr/INDEX.md) - архитектурные решения

## 🎯 Roadmap

- ✅ **Phase 1:** Domain & API v1 (100%)
- ✅ **Phase 2:** Sagas & Outbox (100%)
- ✅ **Phase 3:** Compensations & Refunds (100%)
- ✅ **Phase 4:** Event-Driven Architecture & Resilience (100%)
- ✅ **Phase 5:** Productionization - CI/CD, K8s, Helm (95%)
- 🔄 **Phase 6:** Enhancements - Tracing, PostgreSQL (15%)

**Текущий статус:** 92% Production Ready 🚀

Детали: [docs/roadmap.md](docs/roadmap.md)

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 👤 Author

**Vladislav Dragonenkov**

- GitHub: [@vladislavdragonenkov](https://github.com/vladislavdragonenkov)

## 🙏 Acknowledgments

- Saga Pattern inspiration from [Microservices Patterns](https://microservices.io/patterns/data/saga.html)
- Event-Driven Architecture best practices
- Go community for excellent tooling

---

**⭐ Если проект полезен, поставьте звезду!**
