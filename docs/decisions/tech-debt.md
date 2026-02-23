# Tech Debt Register

> Актуальный реестр технического долга для текущей вехи BoostMarket

**Версия:** v1.0 | **Обновлено:** 2026-02-23 | **Статус:** Active

---

## TL;DR
- Критичный долг: runtime всё ещё работает на mock Inventory/Payment интеграциях.
- Существенный долг: delivery foundation (курьеры/зоны/слоты) есть в storage/domain, но ещё не выведен в публичный API.
- Контрактный долг: часть proto-полей зарезервирована, но runtime пока их не обрабатывает (`ListOrders` pagination/filter).
- Security-контур production-уровня ещё не внедрён.

## Приоритеты

| Приоритет | Долг | Влияние | Целевое закрытие |
|---|---|---|---|
| P0 | Mock-only Inventory/Payment в runtime | Нельзя считать production-ready финансовый контур | Sprint 2-3 |
| P1 | Нет публичного Courier API при наличии delivery storage/domain | Sprint 2 не закрыт end-to-end | Sprint 2 |
| P1 | Неполная реализация `ListOrders` (`page_token`, `filter_statuses`) | Ограничения API-контракта для клиентов | Sprint 3 |
| P1 | Security baseline (mTLS/JWT/RBAC/secret manager) не реализован | Повышенный риск при внешнем доступе | До production gate |
| P2 | Дублирование контура инициализации зависимостей (`Dependencies`/`runtimeDependencies`) | Сложность поддержки и риски расхождения | Backlog |

## Детализация долгов

### P0 — Mock integrations в runtime-path
- Факт: сервис использует `inventory.NewMockService()` и `payment.NewMockService()`.
- Факт: для `postgres` запуска нужен `OMS_ALLOW_MOCK_INTEGRATIONS=true`.
- Риск: поведение оплаты/резерва не отражает реальные внешние зависимости.
- Критерий закрытия:
  1. Внедрены реальные адаптеры Inventory/Payment.
  2. `OMS_ALLOW_MOCK_INTEGRATIONS` не требуется для production-профиля.
  3. Есть integration tests для happy-path и failure/retry-path.

### P1 — Delivery API не доведён до runtime
- Факт: таблицы/репозитории `couriers`, `courier_zones`, `courier_slots`, `courier_vehicle_capabilities` уже есть.
- Факт: публичный gRPC API для курьеров пока отсутствует в `proto`/runtime registration.
- Риск: нет end-to-end потока для delivery-домена.
- Критерий закрытия:
  1. Добавлены proto-контракты courier management.
  2. Добавлены gRPC handlers + валидация доменных правил.
  3. Добавлены integration/e2e тесты по зонам/слотам/capacity.

### P1 — Контрактный долг `ListOrders`
- Факт: `ListOrders` использует только `customer_id` + `page_size`.
- Факт: `page_token` и `filter_statuses` пока не применяются, `next_page_token` пуст.
- Риск: несоответствие ожиданий клиентов от proto-контракта.
- Критерий закрытия:
  1. Реализованы keyset pagination и фильтрация статусов.
  2. Контракт покрыт интеграционными тестами.

### P1 — Security baseline
- Факт: authn/authz слой не включён в runtime.
- Риск: повышенный операционный и комплаенс-риск при внешнем доступе.
- Критерий закрытия:
  1. mTLS/JWT/RBAC внедрены.
  2. Секреты в secret manager, ротация регламентирована.
  3. Security gates присутствуют в CI/CD.

### P2 — Инициализация зависимостей
- Факт: сохраняется разделение `Dependencies` и `runtimeDependencies` с частичным дублированием.
- Риск: рост сложности и ошибок при расширении bootstrapping.
- Критерий закрытия:
  1. Единый прозрачный bootstrap flow.
  2. Тесты на конфиг-профили и wiring.

## Правила ведения реестра
- Любой новый значимый долг добавляется с приоритетом, риском и DoD закрытия.
- При закрытии долга фиксируется ссылка на PR/ADR.
- Этот файл — источник истины по техдолгу для planning/retro.
