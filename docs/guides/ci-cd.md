# 🚀 CI/CD Pipeline

Автоматизированный pipeline для проверки, тестирования и сборки проекта OMS.

## 📊 Обзор

Pipeline запускается автоматически при:
- Push в ветки `main`, `master`, `dev`
- Создании Pull Request

### Структура Pipeline

```
┌─────────────┐
│    Lint     │ ──┐
└─────────────┘   │
                  ├──▶ ┌─────────────┐
┌─────────────┐   │    │    Build    │
│    Test     │ ──┤    └─────────────┘
└─────────────┘   │
                  │    ┌─────────────┐
┌─────────────┐   ├──▶ │   Docker    │
│  Security   │ ──┘    └─────────────┘
└─────────────┘        
                       ┌─────────────┐
                       │   Summary   │
                       └─────────────┘
```

---

## 🔧 Jobs

### 1. Lint & Format

**Цель:** Проверка качества кода

**Проверки:**
- ✅ `gofmt` - форматирование кода
- ✅ `go vet` - статический анализ
- ✅ `golangci-lint` - комплексный линтинг

**Локальный запуск:**
```bash
make fmt
make vet
make lint
```

---

### 2. Tests

**Цель:** Проверка функциональности

**Проверки:**
- ✅ Unit тесты с race detector
- ✅ Integration тесты
- ✅ Coverage отчёт

**Команды:**
```bash
go test ./... -race -count=1 -v
go test ./... -coverprofile=coverage.out
```

**Coverage загружается в Codecov** (опционально)

**Локальный запуск:**
```bash
make test-race
make cover
```

---

### 3. Build

**Цель:** Сборка бинарного файла

**Действия:**
- Компиляция с ldflags (version, commit, date)
- Загрузка артефакта (retention: 7 дней)

**Команда:**
```bash
go build -ldflags "$LDFLAGS" -o bin/order-service ./cmd/order-service
```

**Локальный запуск:**
```bash
make build
```

---

### 4. Docker

**Цель:** Сборка Docker образа

**Действия:**
- Build Docker image с BuildKit cache
- Опционально: Push в Docker Hub

**Теги:**
- `order-service:${{ github.sha }}`
- `order-service:latest`

**Локальный запуск:**
```bash
make docker-build
```

**Push в registry (раскомментировать в ci.yml):**
```yaml
- name: Login to Docker Hub
  uses: docker/login-action@v3
  with:
    username: ${{ secrets.DOCKERHUB_USERNAME }}
    password: ${{ secrets.DOCKERHUB_TOKEN }}
```

---

### 5. Security Scan

**Цель:** Проверка безопасности

**Инструмент:** Gosec Security Scanner

**Проверки:**
- SQL injection
- Hardcoded credentials
- Weak crypto
- Path traversal
- И другие уязвимости

**Результат:** SARIF файл загружается в GitHub Security

---

### 6. Summary

**Цель:** Итоговый отчёт

**Формат:**
```
## 📊 CI/CD Pipeline Results

| Job | Status |
|-----|--------|
| Lint | ✅ Passed |
| Tests | ✅ Passed |
| Build | ✅ Passed |
| Docker | ✅ Passed |
| Security | ✅ Passed |

✅ **All checks passed!**
```

---

## ⚙️ Конфигурация

### Environment Variables

```yaml
env:
  GO_VERSION: '1.22.x'
  GOLANGCI_LINT_VERSION: v1.59.1
```

### Concurrency

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

Отменяет предыдущие запуски при новом push.

---

## 🔐 Secrets

Для полной функциональности добавьте secrets в GitHub:

### Docker Hub (опционально)

```
DOCKERHUB_USERNAME - ваш Docker Hub username
DOCKERHUB_TOKEN    - Docker Hub access token
```

**Добавление:**
1. Settings → Secrets and variables → Actions
2. New repository secret
3. Добавить `DOCKERHUB_USERNAME` и `DOCKERHUB_TOKEN`

### Codecov (опционально)

```
CODECOV_TOKEN - токен для загрузки coverage
```

---

## 📈 Мониторинг

### GitHub Actions

Просмотр запусков:
```
Repository → Actions → CI/CD Pipeline
```

### Badges

Добавьте в README.md:

```markdown
![CI](https://github.com/username/oms/workflows/CI/badge.svg)
![Coverage](https://codecov.io/gh/username/oms/branch/main/graph/badge.svg)
```

---

## 🐛 Отладка

### Локальная проверка перед push

```bash
# Полная проверка как в CI
make fmt
make vet
make lint
make test-race
make build
```

### Проверка конкретного job

```bash
# Lint
make lint

# Tests
make test-race

# Build
make build

# Docker
make docker-build
```

### Act - локальный запуск GitHub Actions

Установка:
```bash
brew install act
```

Запуск:
```bash
# Запустить весь workflow
act

# Запустить конкретный job
act -j lint
act -j test
```

---

## 🚦 Статусы

### Success ✅

Все проверки пройдены. PR можно мержить.

### Failure ❌

Проверки не прошли. Исправьте ошибки:

1. Проверьте логи в GitHub Actions
2. Запустите проверки локально
3. Исправьте проблемы
4. Push исправлений

### Warning ⚠️

Security scan нашёл потенциальные проблемы. Проверьте отчёт.

---

## 📝 Best Practices

### 1. Перед коммитом

```bash
# Pre-commit hook автоматически запустит:
# - gofmt
# - go vet
# - go test -race -short
# - golangci-lint (если установлен)
```

### 2. Перед созданием PR

```bash
# Полная проверка
make test-race && make cover && make lint
```

### 3. При добавлении зависимостей

```bash
go mod tidy
go mod verify
```

### 4. При изменении Dockerfile

```bash
make docker-build
docker run --rm order-service:latest --version
```

---

## 🔄 Continuous Deployment

Для автоматического деплоя добавьте job:

```yaml
deploy:
  name: Deploy
  runs-on: ubuntu-latest
  needs: [build, docker]
  if: github.ref == 'refs/heads/main' && github.event_name == 'push'
  steps:
    - name: Deploy to staging
      run: |
        # Ваш скрипт деплоя
        kubectl set image deployment/oms oms=order-service:${{ github.sha }}
```

---

## 📊 Метрики

### Среднее время выполнения

| Job | Время |
|-----|-------|
| Lint | ~1 мин |
| Tests | ~2 мин |
| Build | ~1 мин |
| Docker | ~2 мин |
| Security | ~1 мин |
| **Итого** | **~7 мин** |

### Оптимизация

- ✅ Go modules cache
- ✅ Docker BuildKit cache
- ✅ Параллельные jobs
- ✅ Cancel in progress

---

## 🔗 Дополнительные ресурсы

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [golangci-lint](https://golangci-lint.run/)
- [Gosec](https://github.com/securego/gosec)
- [Docker Build Push Action](https://github.com/docker/build-push-action)

---

**✅ CI/CD Pipeline готов к использованию!**
