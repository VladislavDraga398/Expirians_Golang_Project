# 📚 Документация OMS

**Версия:** v2.0 | **Обновлено:** 2025-10-01 | **Статус:** 92% Production Ready

---

## 🚀 Быстрый старт

Новичок в проекте? Начните здесь:

1. **[Quick Start Guide](quick-start.md)** - запустите проект за 5 минут
2. **[README](../README.md)** - обзор проекта
3. **[Roadmap](roadmap.md)** - план развития

---

## 📖 Руководства (guides/)

### API и интеграция
- **[API Examples](guides/api-examples.md)** - примеры использования API (gRPC + REST)
- **[API Specification](guides/api-specification.md)** - полная спецификация gRPC API
- **[gRPC-Gateway](guides/grpc-gateway.md)** - REST API поверх gRPC

### Разработка
- **[Makefile Guide](guides/makefile.md)** - все команды для разработки
- **[CI/CD Pipeline](guides/ci-cd.md)** - GitHub Actions, автоматизация

### Инфраструктура
- **[Kafka Integration](guides/kafka.md)** - Event-Driven Architecture
- **[Kubernetes Deployment](../deploy/k8s/README.md)** - деплой в K8s
- **[Helm Chart](../deploy/helm/oms/README.md)** - Helm chart guide

---

## 🏗️ Архитектура (architecture/)

### Основы
- **[Architecture Overview](architecture/overview.md)** - общая архитектура системы
- **[Data Model](architecture/data-model.md)** - модель данных
- **[Saga Pattern](architecture/saga.md)** - оркестрация распределённых транзакций

### Паттерны
- **[Idempotency](architecture/idempotency.md)** - обеспечение идемпотентности
- **[Transactional Outbox](architecture/outbox.md)** - гарантированная доставка событий

---

## 🔧 Операции (operations/)

### Deployment
- **[Deployment Guide](operations/deployment.md)** - стратегии деплоя
- **[Observability](operations/observability.md)** - мониторинг и логирование
- **[Security](operations/security.md)** - безопасность
- **[Testing Strategy](operations/testing.md)** - стратегия тестирования

### Поддержка
- **[Runbooks](operations/runbooks.md)** - инструкции для инцидентов

---

## 📝 Решения (decisions/)

### ADR (Architecture Decision Records)
- **[ADR Index](decisions/adr/INDEX.md)** - все архитектурные решения
- [ADR-0001: gRPC Communication](decisions/adr/0001-communication-grpc.md)
- [ADR-0002: Saga Orchestration](decisions/adr/0002-consistency-saga-orchestration.md)
- [ADR-0003: Idempotency Keys](decisions/adr/0003-idempotency-key.md)
- [ADR-0004: Outbox vs CDC](decisions/adr/0004-outbox-vs-cdc.md)

### Открытые вопросы
- **[Open Questions](decisions/open-questions.md)** - нерешённые вопросы

---

## 📋 Шаблоны (Templates)

- **[ADR Template](templates/adr-template.md)** - шаблон для новых ADR
- **[Runbook Template](templates/runbook-template.md)** - шаблон runbook
- **[Incident Report](templates/incident-report-template.md)** - отчёт об инциденте
- **[RFC Template](templates/rfc-template.md)** - Request for Comments

---

## 🗺️ Roadmap

- **[Project Roadmap](roadmap.md)** - план развития проекта

---

## 🔍 Поиск по темам

### По функциональности
- **Создание заказа:** [API Examples](guides/api-examples.md#createorder), [Saga](architecture/saga.md)
- **Оплата:** [API Examples](guides/api-examples.md#payorder), [Saga](architecture/saga.md)
- **Отмена/Возврат:** [API Examples](guides/api-examples.md#cancelorder), [Saga](architecture/saga.md)
- **События:** [Kafka Integration](guides/kafka.md), [Outbox](architecture/outbox.md)

### По технологиям
- **gRPC:** [API Spec](guides/api-specification.md), [API Examples](guides/api-examples.md)
- **REST:** [gRPC-Gateway](guides/grpc-gateway.md)
- **Kafka:** [Kafka Integration](guides/kafka.md)
- **Kubernetes:** [K8s README](../deploy/k8s/README.md)
- **Helm:** [Helm README](../deploy/helm/oms/README.md)
- **CI/CD:** [CI/CD Guide](guides/ci-cd.md)

### По задачам
- **Запустить проект:** [Quick Start](quick-start.md)
- **Написать тесты:** [Testing](operations/testing.md)
- **Задеплоить:** [Deployment](operations/deployment.md), [K8s](../deploy/k8s/README.md)
- **Отладить:** [Runbooks](operations/runbooks.md), [Observability](operations/observability.md)
- **Добавить фичу:** [Architecture](architecture/overview.md), [ADR](decisions/adr/INDEX.md)

---

## 💡 Полезные ссылки

### Внешние ресурсы
- [Saga Pattern](https://microservices.io/patterns/data/saga.html)
- [gRPC Best Practices](https://grpc.io/docs/guides/performance/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)

### Инструменты
- [grpcurl](https://github.com/fullstorydev/grpcurl) - gRPC CLI
- [ghz](https://ghz.sh/) - gRPC benchmarking
- [kubectl](https://kubernetes.io/docs/reference/kubectl/) - K8s CLI
- [helm](https://helm.sh/) - Kubernetes package manager

---

## 🤝 Contributing

Хотите улучшить документацию?

1. Используйте [шаблоны](templates/) для новых документов
2. Следуйте структуре существующих документов
3. Обновляйте этот индекс при добавлении новых файлов
4. Указывайте дату обновления в документах

---

## 📞 Нужна помощь?

- **Не можете найти информацию?** Проверьте [поиск по темам](#поиск-по-темам)
- **Нашли ошибку?** Создайте issue или PR
- **Есть вопрос?** Проверьте [Open Questions](open-questions.md)

---

**Последнее обновление:** 2025-10-01  
**Поддерживается:** Vladislav Dragonenkov
