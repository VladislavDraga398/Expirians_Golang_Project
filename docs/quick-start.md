# ⚡ Quick Start Guide

Запустите OMS за **5 минут**!

---

## 📋 Предварительные требования

Убедитесь, что установлено:

```bash
# Проверить версии
go version        # Go 1.21+
docker --version  # Docker 20.10+
make --version    # GNU Make
```

Если чего-то нет:
- **Go:** https://go.dev/dl/
- **Docker:** https://docs.docker.com/get-docker/
- **Make:** `brew install make` (macOS)

---

## 🚀 Шаг 1: Клонирование и зависимости (1 мин)

```bash
# Клонировать репозиторий
git clone https://github.com/vladislavdragonenkov/oms.git
cd oms

# Установить зависимости
make deps
```

---

## 🐳 Шаг 2: Запуск инфраструктуры (2 мин)

```bash
# Запустить Kafka, Prometheus, Grafana
make compose-up

# Дождаться готовности (займёт ~30 сек)
make wait-health
```

**Что запустилось:**
- ✅ Kafka (localhost:9092)
- ✅ Zookeeper (localhost:2181)
- ✅ Prometheus (http://localhost:9091)
- ✅ Grafana (http://localhost:3000)

---

## 🏃 Шаг 3: Запуск сервиса (1 мин)

### Вариант A: Локально (для разработки)

```bash
# Собрать и запустить
make build
make run
```

### Вариант B: Docker (ближе к production)

```bash
# Собрать Docker image и запустить
make docker-build
make docker-run
```

**Сервис запущен на:**
- 🔌 gRPC: `localhost:50051`
- 📊 Metrics: `http://localhost:9090/metrics`
- 🏥 Health: `http://localhost:9090/healthz`

---

## ✅ Шаг 4: Проверка (1 мин)

### Проверить health

```bash
curl http://localhost:9090/healthz
```

Ожидаемый ответ:
```json
{
  "status": "healthy",
  "timestamp": "2025-10-01T...",
  "version": "dev",
  "uptime_seconds": 5
}
```

### Создать заказ

```bash
# Установить grpcurl (если нет)
make ensure-grpcurl

# Создать заказ
grpcurl -plaintext -d '{
  "customer_id": "customer-123",
  "currency": "USD",
  "items": [{
    "sku": "SKU-001",
    "qty": 2,
    "price": {
      "currency": "USD",
      "amount_minor": 10000
    }
  }]
}' localhost:50051 oms.v1.OrderService/CreateOrder
```

Ожидаемый ответ:
```json
{
  "order_id": "01HQZX...",
  "status": "ORDER_STATUS_PENDING",
  "created_at": "2025-10-01T..."
}
```

---

## 🎬 Бонус: Запустить демо (опционально)

```bash
# Полный демо-сценарий
make demo

# Откроется Grafana с метриками
open http://localhost:3000
# Логин: admin / Пароль: admin
# Dashboard: OMS → OMS Saga Overview
```

---

## 🧪 Запустить тесты

```bash
# Все тесты
make test

# С race detector (важно!)
make test-race

# Coverage отчёт
make cover
```

---

## 🛑 Остановка

```bash
# Остановить сервис
Ctrl+C

# Остановить инфраструктуру
make compose-down

# Полная очистка
make clean-all
```

---

## 📚 Что дальше?

### Для разработчиков
1. **[API Examples](API_EXAMPLES.md)** - все операции с примерами
2. **[Makefile Guide](MAKEFILE_GUIDE.md)** - все доступные команды
3. **[Architecture](architecture.md)** - как устроена система

### Для DevOps
1. **[Kubernetes Deployment](../deploy/k8s/README.md)** - деплой в K8s
2. **[Helm Chart](../deploy/helm/oms/README.md)** - Helm guide
3. **[CI/CD](CI_CD.md)** - автоматизация

### Для изучения
1. **[Saga Pattern](saga.md)** - распределённые транзакции
2. **[Kafka Integration](KAFKA_INTEGRATION.md)** - Event-Driven Architecture
3. **[ADR Index](adr/INDEX.md)** - архитектурные решения

---

## ❓ Troubleshooting

### Порты заняты

```bash
# Проверить, что занимает порт
lsof -i :50051
lsof -i :9090

# Убить процесс
kill -9 <PID>
```

### Docker проблемы

```bash
# Перезапустить Docker
# macOS: Docker Desktop → Restart

# Очистить всё
docker system prune -a
```

### Kafka не запускается

```bash
# Проверить логи
docker compose logs kafka

# Перезапустить
make compose-down
make compose-up
```

### Тесты не проходят

```bash
# Проверить зависимости
make deps

# Проверить форматирование
make fmt

# Запустить с verbose
make test-v
```

---

## 🎯 Полезные команды

```bash
# Показать все команды
make help

# Проверить статус
make k8s-status          # Kubernetes
make helm-status         # Helm

# Логи
make k8s-logs            # K8s logs
docker compose logs -f   # Docker logs

# Тестирование
make test-race           # Race detector
make test-saga           # Saga тесты
make bench               # Бенчмарки
```

---

## 📞 Нужна помощь?

- **Документация:** [INDEX.md](INDEX.md)
- **API примеры:** [API_EXAMPLES.md](API_EXAMPLES.md)
- **Issues:** https://github.com/vladislavdragonenkov/oms/issues

---

**🎉 Готово! Проект запущен!**

**Время:** ~5 минут  
**Следующий шаг:** [API Examples](API_EXAMPLES.md)
