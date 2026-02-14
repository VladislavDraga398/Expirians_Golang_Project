# Makefile Guide

Полное руководство по использованию Makefile для проекта OMS.

## Быстрый старт

```bash
# Показать все доступные команды
make help

# Запустить тесты с race detector
make test-race

# Собрать и запустить проект
make build
make run
```

---

## Разделы Makefile

### Кодогенерация и зависимости

| Команда | Описание |
|---------|----------|
| `make proto` | Генерация gRPC/Protobuf кода |
| `make generate` | Сгенерировать код и проверить чистоту git |
| `make tidy` | Обновить go.mod и go.sum |
| `make deps` | Скачать зависимости |

### Сборка и запуск

| Команда | Описание |
|---------|----------|
| `make build` | Собрать бинарник в папку bin/ |
| `make run` | Запустить сервис локально |
| `make migrate-up` | Применить SQL миграции |
| `make migrate-down` | Откатить SQL миграции (по умолчанию 1 шаг) |
| `make migrate-status` | Показать статус SQL миграций |

### Тестирование

#### Базовое тестирование

| Команда | Описание |
|---------|----------|
| `make test` | Запустить все тесты |
| `make test-v` | Тесты с verbose output |
| `make test-race` | **Тесты с race detector**  |
| `make test-race-v` | Race detector + verbose |

#### Тестирование по компонентам

| Команда | Описание |
|---------|----------|
| `make test-unit` | Только юнит-тесты |
| `make test-integration` | Только интеграционные тесты |
| `make test-saga` | Тесты saga orchestrator |
| `make test-kafka` | Тесты Kafka integration |
| `make test-grpc` | Тесты gRPC service |

#### Специальные режимы

| Команда | Описание |
|---------|----------|
| `make test-short` | Быстрые тесты (пропускает длинные) |
| `make test-count` | Запустить тесты 10 раз (проверка стабильности) |
| `make test-failfast` | Остановить при первой ошибке |

#### Coverage и бенчмарки

| Команда | Описание |
|---------|----------|
| `make cover` | Отчёт покрытия (txt + HTML) |
| `make cover-race` | Coverage с race detector |
| `make bench` | Запустить бенчмарки производительности |

> Базовые тестовые цели (`test`, `test-unit`, `test-integration`, `test-race`) используют централизованные скрипты из `test/run/`.

### Линтинг и статический анализ

| Команда | Описание |
|---------|----------|
| `make fmt` | Форматирование кода (gofmt) |
| `make vet` | Статический анализ (go vet) |
| `make lint` | Полный линтинг |

### Docker

| Команда | Описание |
|---------|----------|
| `make docker-build` | Собрать Docker-образ |
| `make docker-run` | Запустить контейнер локально |

### Docker Compose

| Команда | Описание |
|---------|----------|
| `make compose-up` | Запустить стек |
| `make compose-down` | Остановить стек |
| `make compose-build-up` | Собрать образ и поднять стек |

### Демонстрация

| Команда | Описание |
|---------|----------|
| `make demo` | Полный прогон: build + compose + health + grpc |
| `make demo-run` | Выполнить демо-сценарий саги |
| `make demo-down` | Остановить демо-стек |
| `make demo-refund` | Демо с RefundOrder |
| `make demo-success` | Демо успешного сценария |
| `make load` | Нагрузочное тестирование через `cmd/loadtest` (100 req) |
| `make load-stress` | Стресс-тест через `cmd/loadtest` (1000 req) |
| `make load-soak` | Длительный time-based soak-тест (по умолчанию 10m) |

### Утилиты

| Команда | Описание |
|---------|----------|
| `make clean` | Удалить артефакты сборки |
| `make help` | Показать все команды |

---

## Рекомендуемые команды

### Перед коммитом

```bash
# 1. Проверить race conditions
make test-race

# 2. Проверить coverage
make cover

# 3. Линтинг
make lint

# 4. Форматирование
make fmt
```

### При разработке

```bash
# Быстрая проверка конкретного компонента
make test-saga

# Verbose output для отладки
make test-v

# Остановить при первой ошибке
make test-failfast
```

### CI/CD

```bash
# Полная проверка
make test-race && make cover && make lint
```

---

## Примеры использования

### Пример 1: Разработка новой фичи

```bash
# 1. Запустить зависимости
make compose-up

# 2. Запустить сервис
make run

# 3. В другом терминале - тесты
make test-saga

# 4. Проверить race conditions
make test-race
```

### Пример 2: Проверка перед PR

```bash
# Полная проверка
make test-race
make cover
make lint

# Если всё ОК - коммит
git add .
git commit -m "feat: add new feature"
```

### Пример 3: Демонстрация проекта

```bash
# Запустить полное демо
make demo

# Открыть Grafana
open http://localhost:3000

# Нагрузочное тестирование
make load

# Длительный soak-тест (пример 15 минут)
DURATION=15m make load-soak
```

---

## Структура Makefile

```
Makefile
├──  КОНФИГУРАЦИЯ
├──  ОСНОВНЫЕ КОМАНДЫ
├──  КОДОГЕНЕРАЦИЯ И ЗАВИСИМОСТИ
├──  СБОРКА И ЗАПУСК
├──  ТЕСТИРОВАНИЕ
│   ├── Базовое тестирование
│   ├── Coverage и бенчмарки
├──  ЛИНТИНГ И СТАТИЧЕСКИЙ АНАЛИЗ
├──  DOCKER
├──  DOCKER COMPOSE
├──  ДЕМОНСТРАЦИЯ
└──  УТИЛИТЫ
```

---

## Горячие клавиши (рекомендуемые алиасы)

Добавьте в `~/.bashrc` или `~/.zshrc`:

```bash
alias mt='make test'
alias mtr='make test-race'
alias mc='make cover'
alias ml='make lint'
alias md='make demo'
```

Использование:

```bash
mtr  # вместо make test-race
mc   # вместо make cover
```

---

**Готово! Makefile полностью реорганизован и готов к использованию.** 
