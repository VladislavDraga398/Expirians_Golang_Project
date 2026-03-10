# Security

> Текущий security baseline и целевая модель для OMS/BoostMarket

**Версия:** v2.1 | **Обновлено:** 2026-02-23 | **Статус:** Частично реализовано

---

## TL;DR
- Runtime сейчас не содержит встроенного authn/authz слоя в gRPC API.
- Интеграции Inventory/Payment пока mock-only в runtime-path.
- Для PostgreSQL режима требуется явный флаг `OMS_ALLOW_MOCK_INTEGRATIONS=true`.
- Полноценный security-контур (mTLS/JWT/RBAC/secret manager) остаётся обязательной задачей перед production.

## Текущий runtime-статус

### Что уже есть
- Базовая валидация входных данных на уровне gRPC handlers.
- Идемпотентность mutating RPC через `idempotency-key`.
- Health/readiness/liveness endpoints для эксплуатационного контроля.

### Что ещё не реализовано в runtime
- mTLS между сервисами.
- JWT/OIDC или API gateway auth для внешнего контура.
- RBAC на уровне RPC методов.
- Централизованный secret manager как обязательный runtime dependency.
- Маскирование PII и формальная policy-аудит трасс/логов.

## Критичный operational guardrail
- В `postgres` режиме запуск разрешён только с `OMS_ALLOW_MOCK_INTEGRATIONS=true`, так как реальные Inventory/Payment адаптеры пока не внедрены.
- Это означает: текущая сборка не должна считаться production-ready для финансового контура.

## Целевая модель (до production)
- Аутентификация:
  - east-west: mTLS
  - north-south: JWT/OIDC (или gateway-managed auth)
- Авторизация:
  - RBAC по RPC операциям (`RefundOrder` и админ-операции — повышенные роли)
- Секреты:
  - хранение и ротация через secret manager
- Защита от злоупотреблений:
  - rate limiting, payload limits, abuse-control для idempotency ключей
- Аудит:
  - обязательная маскировка PII
  - структурированный security-аудит событий

## Минимальный security DoD для production
1. Включён authn/authz контур (mTLS + JWT/OIDC/RBAC).
2. Убрана зависимость от mock integrations в runtime-path.
3. Секреты управляются вне репозитория, с ротацией.
4. Добавлены security checks в CI/CD (в т.ч. policy gates).
5. Подготовлен runbook на incident response по auth/secret compromise.

## Связанные документы
- `docs/operations/deployment.md`
- `docs/operations/runbooks.md`
- `docs/decisions/tech-debt.md`
