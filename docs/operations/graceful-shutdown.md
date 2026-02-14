# Graceful Shutdown

> Штатное и безопасное завершение OMS без потери in-flight операций

**Версия:** v2.1 | **Обновлено:** 2026-02-12 | **Статус:** Актуально

---

## Цель

Обеспечить предсказуемую остановку процесса:
- перестать принимать новые RPC;
- завершить текущие RPC;
- дождаться фоновых saga-задач;
- корректно остановить HTTP/metrics;
- закрыть Kafka producer.

---

## Текущая реализация

Процесс остановки инициируется `SIGINT/SIGTERM` в `cmd/order-service/main.go`.

В `internal/app/app.go` реализована последовательность:
1. `grpcServer.GracefulStop()` с fallback на `grpcServer.Stop()` после таймаута 5 секунд.
2. `orderService.Shutdown(ctx)` — ожидание завершения фоновых saga-задач.
3. `shutdownHTTP(metricsSrv)` — корректная остановка HTTP сервера (`/metrics`, `/healthz`, `/livez`, `/readyz`).
4. `kafkaProducer.Close()` — закрытие producer.

В `internal/service/grpc/order_service.go`:
- фоновые saga-dispatch (`PayOrder/CancelOrder/RefundOrder`) учитываются через `WaitGroup`;
- во время shutdown новые saga-dispatch блокируются;
- `Shutdown(ctx)` ожидает завершения уже запущенных задач.

---

## Поведение health endpoints при остановке

- `/livez` — процесс жив.
- `/readyz` — готовность к обработке новых запросов.
- `/healthz` — агрегированный health-статус.

Для orchestrator-level drain важно:
- сначала остановить приём новых RPC;
- затем дождаться окончания in-flight работ;
- после этого завершать процесс.

---

## Рекомендованные параметры

- `terminationGracePeriodSeconds`: **30s** (уже задано в k8s/helm).
- `grpc graceful timeout`: **5s** (с fallback на hard stop).
- `order-service async drain timeout`: **5s**.

Если нагрузка увеличится:
- увеличить timeout drain;
- сократить время отдельных saga-шагов;
- добавить watchdog и outbox backlog контроль.

---

## Проверка вручную

1. Запустить сервис (`make run` или `make demo`).
2. Создать нагрузку (`make load` или `scripts/saga_load.sh`).
3. Отправить сигнал остановки (`Ctrl+C` или `docker stop oms`).
4. Проверить логи:
   - нет резкого обрыва активных RPC;
   - есть последовательная остановка gRPC -> saga drain -> HTTP -> Kafka close.

---

## Failure modes и реакции

- Если `graceful stop` gRPC зависает >5s: выполняется `grpcServer.Stop()`.
- Если фоновые saga не завершились в timeout: `Shutdown(ctx)` вернёт timeout-ошибку.
- Если Kafka close завершился ошибкой: ошибка логируется, завершение процесса продолжается.

---

## Что улучшать далее

- Перевести критичные saga-операции на контекст-aware API внешних адаптеров.
- Добавить метрики drain/shutdown:
  - `oms_shutdown_duration_seconds`
  - `oms_shutdown_inflight_sagas`
- Добавить e2e тест на shutdown под нагрузкой.
