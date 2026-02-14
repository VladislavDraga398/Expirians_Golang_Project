# OMS Technical Documentation Hub

Единая точка входа во всю техническую документацию проекта.

**Версия:** v2.2  
**Обновлено:** 2026-02-14  
**Статус:** актуально

## Start Here

1. [Quick Start](quick-start.md)  
2. [Project README](../README.md)  
3. [Roadmap](roadmap.md)

## Architecture

- [Architecture Overview](architecture/overview.md)
- [Data Model](architecture/data-model.md)
- [Saga Pattern](architecture/saga.md)
- [Idempotency](architecture/idempotency.md)
- [Transactional Outbox](architecture/outbox.md)

## API and Integration

- [API Specification](guides/api-specification.md)
- [API Examples](guides/api-examples.md)
- [gRPC Gateway](guides/grpc-gateway.md)
- [Kafka Integration](guides/kafka.md)

## Operations and Reliability

- [Deployment](operations/deployment.md)
- [Graceful Shutdown](operations/graceful-shutdown.md)
- [Observability](operations/observability.md)
- [Security](operations/security.md)
- [Testing Strategy](operations/testing.md)
- [Runbooks](operations/runbooks.md)

## Engineering Process

- [CI/CD](guides/ci-cd.md)
- [Makefile Commands](guides/makefile.md)
- [Open Questions](decisions/open-questions.md)

## Architecture Decisions

- [ADR Index](decisions/adr/INDEX.md)
- [ADR-0001: gRPC](decisions/adr/0001-communication-grpc.md)
- [ADR-0002: Saga orchestration](decisions/adr/0002-consistency-saga-orchestration.md)
- [ADR-0003: Idempotency](decisions/adr/0003-idempotency-key.md)
- [ADR-0004: Outbox vs CDC](decisions/adr/0004-outbox-vs-cdc.md)

## Infrastructure Docs

- [Kubernetes Guide](../deploy/k8s/README.md)
- [Helm Guide](../deploy/helm/oms/README.md)

## Templates

- [ADR Template](templates/adr-template.md)
- [Runbook Template](templates/runbook-template.md)
- [Incident Report Template](templates/incident-report-template.md)
- [RFC Template](templates/rfc-template.md)

## Documentation Rules

- Все новые документы добавляются в этот файл.
- Новые ADR добавляются в `docs/decisions/adr/INDEX.md` и в раздел ADR выше.
- Если ссылка устарела, правка начинается с этого файла.
