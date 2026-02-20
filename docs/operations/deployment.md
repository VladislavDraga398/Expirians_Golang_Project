# Deployment

> Стратегии деплоя и развёртывания OMS

**Версия:** v2.3 | **Обновлено:** 2026-02-20 | **Статус:** Актуально

---

## TL;DR
- Окружения: Dev (Compose/Testcontainers), CI, Staging, Prod (K8s).
- Zero-downtime: Rolling Update как дефолт, Canary/Blue-Green для рискованных релизов.
- Миграции: backwards-compatible, запуск до/вместе с выкладкой, план отката.
- Конфиг через env/Secret Manager, rate limit + mTLS на периметре.

## Назначение
Архитектура развёртывания, стратегии без простоя, миграции, конфигурация и секреты.

## Окружения
- Dev (локально)
- CI (автотесты)
- Staging (похоже на прод)
- Prod

## Принципы
- Иммутабельные образы, декларативные манифесты.
- Конфигурация через env/secret manager, не в коде.
- Обратносovместимые миграции, релизы без простоя.

## Локально (Dev)
- Контейнеры: OrderService, реляционная БД, брокер, стек наблюдаемости (опц.), моки Inventory/Payment.
- Make-таргеты/скрипты для миграций и фикстур.
- Альтернатива: Testcontainers в тестах.

## Прод (Kubernetes)
- Deployments для приложения; предпочтительно управляемые/внешние БД и брокер.
- Пробы: `readinessProbe`, `livenessProbe`.
- Масштабирование: requests/limits, `HPA`.
- Надёжность: `PodDisruptionBudget`, `NetworkPolicy`, TLS/mTLS.

## Стратегии деплоя
- Rolling Update (по умолчанию)
- Canary для рискованных релизов
- Blue/Green для быстрого отката

## Миграции БД
- Обратносovместимые шаги: добавить (nullable/с дефолтом) → backfill → переключить код → удалить старое.
- Запускать миграции до/вместе с выкладкой; избегать блокирующих DDL.
- План отката версий схемы.

## Конфиг и секреты
- Конфиг: env/config maps; таймауты, ретраи, фичефлаги.
- Секреты: secret manager (Vault/Cloud). Регулярная ротация; ограничение доступа.

### Минимальные env для storage-драйвера
- `OMS_STORAGE_DRIVER=memory|postgres`
- `OMS_POSTGRES_DSN=postgres://...`
- `OMS_POSTGRES_AUTO_MIGRATE=true|false`
- `OMS_IDEMPOTENCY_CLEANUP_INTERVAL=10m` (0 — отключить cleanup)
- `OMS_IDEMPOTENCY_CLEANUP_BATCH_SIZE=500`

### Миграции
- Локально/CI миграции запускаются через `cmd/migrate` (`up`, `down`, `status`).
- В CI отдельный обязательный gate проверяет цикл `up -> down -> up`.

## Управление трафиком
- Ingress/Gateway с TLS; опционально service mesh.
- Rate limiting и WAF на периметре.

## Производительность и ёмкость
- Приложение: размеры по профилю; тюнинг пула БД; масштабирование воркеров outbox.
- Брокер: партиции/subjects, retention, размеры батчей producer/consumer.

## BCP/DR
- Регулярные бэкапы БД + тренировки восстановления.
- Multi-AZ; DR-план (тёплый/холодный) по необходимости.
- Реплей событий через брокер или репроцессинг outbox.

## Жизненный цикл
- Грациозное завершение: перестать принимать, завершить текущие, «пролить» outbox.
- Readiness зависит от критичных зависимостей и порогов бэклога.

### Текущая реализация graceful shutdown
- gRPC: `GracefulStop` + fallback на hard stop по таймауту.
- Фоновые saga-задачи: drain перед завершением процесса.
- HTTP: корректный shutdown endpoints `/metrics`, `/healthz`, `/livez`, `/readyz`.
- Kafka producer: закрывается при остановке.

Подробности: `operations/graceful-shutdown.md`.

## Хуки CI/CD
- CI: lint, unit, integration (контейнеры), contract, security scan, build.
- CD: авто-деплой на stage; prod — с ручным апрувом или канареечными гейтами.
