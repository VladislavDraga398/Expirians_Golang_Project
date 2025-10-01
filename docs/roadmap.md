# 🗺️ Roadmap

> План развития проекта OMS

**Версия:** v2.0 | **Обновлено:** 2025-10-01 | **Статус:** Актуально

---

## 📊 Текущий статус: **Phase 5 Complete** (92% Production Ready)

## 🎯 TL;DR
- ✅ **Phase 1:** Домен + базовый API, идемпотентность, базовая observability
- ✅ **Phase 2:** Саги и Outbox, интеграции Inventory/Payment, E2E-тесты
- ✅ **Phase 3:** Компенсации/Refund, метрики саг, Grafana дашборды
- ✅ **Phase 4:** Kafka Event-Driven Architecture, retry логика, DLQ
- ✅ **Phase 5:** CI/CD, Kubernetes, Helm, Health Checks, gRPC-Gateway
- 🔄 **Phase 6:** Distributed Tracing, PostgreSQL, Schema Registry (In Progress)

## ✅ Фаза 1 — Domain & API v1 (COMPLETE)
- ✅ In-memory storage для быстрого прототипирования
- ✅ gRPC `OrderService`: CreateOrder, GetOrder, PayOrder, CancelOrder, RefundOrder
- ✅ Идемпотентность операций
- ✅ Prometheus метрики для gRPC
- ✅ Structured logging (logrus)
- ✅ Unit-тесты домена (54.5% coverage)
- ✅ Integration тесты

## ✅ Фаза 2 — Sagas & Outbox (COMPLETE)
- ✅ Saga Orchestrator: Reserve → Pay → Confirm
- ✅ Mock Inventory/Payment services
- ✅ Transactional Outbox pattern
- ✅ Timeline события для audit trail
- ✅ E2E-тесты для success/failure сценариев
- ✅ Компенсационные транзакции

## ✅ Фаза 3 — Compensations & Refunds (COMPLETE)
- ✅ Cancel/Refund flows с компенсациями
- ✅ Saga метрики (started/completed/failed/canceled/refunded)
- ✅ Grafana дашборды с визуализацией
- ✅ Demo скрипты для тестирования
- ✅ Load testing с ghz (100 RPS)

## ✅ Фаза 4 — Event-Driven Architecture & Resilience (COMPLETE)
- ✅ Apache Kafka integration (producer/consumer)
- ✅ Event-driven saga с публикацией событий
- ✅ Retry логика с exponential backoff для version conflicts
- ✅ Dead Letter Queue для failed Kafka messages
- ✅ Race condition fixes (все тесты проходят с -race)
- ✅ Production safety (удалены debug флаги)
- ✅ Makefile с 15+ тестовыми командами
- ⏳ Circuit breaker (опционально)
- ⏳ Rate limiting (опционально)

## ✅ Фаза 5 — Productionization (COMPLETE - 95%)
- ✅ Kubernetes manifests (Deployment, Service, ConfigMap, RBAC)
- ✅ Helm chart для параметризации
- ✅ Health probes (liveness, readiness, startup)
- ✅ HPA (Horizontal Pod Autoscaler)
- ✅ PodDisruptionBudget
- ✅ NetworkPolicy
- ✅ CI/CD pipeline (GitHub Actions)
  - ✅ Lint + format check
  - ✅ Unit tests с race detector
  - ✅ Integration tests
  - ✅ Security scan (Gosec)
  - ✅ Docker build & push
  - ✅ Coverage отчёт (Codecov)
- ✅ Health endpoint с детальными checks
- ✅ gRPC-Gateway (REST API поверх gRPC)
- ✅ Документация полностью реорганизована
- ✅ Makefile с 60+ командами

## 🔄 Фаза 6 — Enhancements (IN PROGRESS - 15%)
- ✅ gRPC-Gateway для REST API (proto аннотации готовы)
- 📋 Distributed tracing (Jaeger/Tempo)
- 📋 PostgreSQL вместо in-memory storage
- 📋 Schema Registry для Kafka events (Confluent/Apicurio)
- 📋 Circuit breaker для external services
- 📋 Rate limiting
- 📋 Расширенная отчётность/аналитика
---

## 🎯 Приоритеты на ближайшее время

### ✅ Завершено
1. ✅ Code review fixes (P0 блокеры)
2. ✅ Makefile реорганизация (60+ команд)
3. ✅ Pre-commit hook с race detector
4. ✅ README полностью обновлён
5. ✅ CI/CD Pipeline (GitHub Actions)
6. ✅ Kubernetes + Helm
7. ✅ Health checks (детальные)
8. ✅ gRPC-Gateway (proto готов)
9. ✅ Документация реорганизована

### 🔄 В работе (Phase 6)
10. 📋 Distributed tracing (Jaeger/Tempo)
11. 📋 PostgreSQL migration
12. 📋 Circuit breaker
13. 📋 Rate limiting

### 📋 Запланировано
14. 📋 Schema Registry
15. 📋 Canary deployments
16. 📋 Advanced monitoring

---

## 📈 Метрики прогресса

| Фаза | Статус | Прогресс | Дата завершения |
|------|--------|----------|-----------------|
| Phase 1 | ✅ Complete | 100% | 2025-09-20 |
| Phase 2 | ✅ Complete | 100% | 2025-09-25 |
| Phase 3 | ✅ Complete | 100% | 2025-09-27 |
| Phase 4 | ✅ Complete | 100% | 2025-10-01 |
| Phase 5 | ✅ Complete | 95% | 2025-10-01 |
| Phase 6 | 🔄 In Progress | 15% | TBD |

**Общий прогресс проекта:** 92% Production Ready 🚀
