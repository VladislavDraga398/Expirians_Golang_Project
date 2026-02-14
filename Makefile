.SHELL := /bin/bash

# ========================================================================
# OMS (Order Management System) Makefile
# Подсказка: выполните `make help` для списка всех команд
# ========================================================================

# ========================================================================
# КОНФИГУРАЦИЯ
# ========================================================================
GO              ?= go
APP_NAME        ?= order-service
CMD_PATH        ?= ./cmd/order-service
BIN_DIR         ?= bin
BIN             ?= $(BIN_DIR)/$(APP_NAME)
GOBIN           ?= $(shell $(GO) env GOPATH)/bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS ?= -s -w \
  -X github.com/vladislavdragonenkov/oms/internal/version.version=$(VERSION) \
  -X github.com/vladislavdragonenkov/oms/internal/version.commit=$(COMMIT) \
  -X github.com/vladislavdragonenkov/oms/internal/version.date=$(DATE)

.PHONY: all help clean clean-all \
        proto generate tidy deps \
        build run migrate-up migrate-down migrate-status \
        test test-v test-race test-race-v test-unit test-integration test-saga test-kafka test-grpc test-short test-count test-failfast \
        cover cover-race bench \
        fmt vet lint lint-install staticcheck \
        docker-build docker-run \
        compose-up compose-down compose-build-up \
        k8s-validate k8s-apply k8s-delete k8s-status k8s-logs k8s-describe \
        helm-lint helm-template helm-install helm-upgrade helm-uninstall helm-status helm-dry-run \
        ensure-grpcurl wait-health demo-run demo demo-down load load-stress load-soak demo-refund demo-success

# ========================================================================
# ОСНОВНЫЕ КОМАНДЫ
# ========================================================================

all: build ## Сборка проекта (по умолчанию)

help: ## Показать список всех доступных команд
	@echo "═══════════════════════════════════════════════════════════════════════"
	@echo "OMS Makefile - Доступные команды"
	@echo "═══════════════════════════════════════════════════════════════════════"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""

# ========================================================================
# КОДОГЕНЕРАЦИЯ И ЗАВИСИМОСТИ
# ========================================================================
# Генерация gRPC/Protobuf кода из proto/oms/v1/order_service.proto
proto: ## Генерация gRPC/Protobuf кода
	@echo "Генерация gRPC и gRPC-Gateway кода..."
	protoc \
		-I. \
		-I proto \
		--go_out=paths=source_relative:. \
		--go-grpc_out=paths=source_relative:. \
		--grpc-gateway_out=paths=source_relative:. \
		--grpc-gateway_opt=logtostderr=true \
		--grpc-gateway_opt=generate_unbound_methods=true \
		proto/oms/v1/order_service.proto
	@echo "Генерация завершена"

generate: proto ## Сгенерировать код и проверить чистоту git-дерева
	@git diff --quiet || (echo "\nЕсть несохранённые изменения после генерации. Добавьте их в git." && exit 1)

tidy: ## Обновить go.mod и go.sum
	$(GO) mod tidy

deps: ## Скачать зависимости
	$(GO) mod download

# ========================================================================
# СБОРКА И ЗАПУСК
# ========================================================================
build: $(BIN) ## Собрать бинарник в папку bin/

$(BIN): ## Сборка бинаря с вшитыми метаданными версии
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BIN) $(CMD_PATH)

run: ## Запустить сервис локально (go run)
	OMS_GRPC_ADDR=$(OMS_GRPC_ADDR) \
	OMS_METRICS_ADDR=$(OMS_METRICS_ADDR) \
	OMS_STORAGE_DRIVER=$(OMS_STORAGE_DRIVER) \
	OMS_POSTGRES_DSN=$(OMS_POSTGRES_DSN) \
	OMS_POSTGRES_AUTO_MIGRATE=$(OMS_POSTGRES_AUTO_MIGRATE) \
	$(GO) run $(CMD_PATH)

migrate-up: ## Применить SQL миграции (up)
	OMS_POSTGRES_DSN="$(OMS_POSTGRES_DSN)" $(GO) run ./cmd/migrate -direction up

migrate-down: ## Откатить SQL миграции на N шагов (MIGRATE_STEPS=1 по умолчанию)
	OMS_POSTGRES_DSN="$(OMS_POSTGRES_DSN)" $(GO) run ./cmd/migrate -direction down -steps $${MIGRATE_STEPS:-1}

migrate-status: ## Показать статус SQL миграций
	OMS_POSTGRES_DSN="$(OMS_POSTGRES_DSN)" $(GO) run ./cmd/migrate -direction status

# ========================================================================
# ТЕСТИРОВАНИЕ
# ========================================================================

##@ Базовое тестирование
test: ## Запустить все тесты
	GO=$(GO) ./test/run/all.sh

test-v: ## Запустить все тесты с verbose output
	$(GO) test -v ./...

test-race: ## Запустить тесты с race detector (поиск race conditions)
	@echo "Running tests with race detector..."
	GO=$(GO) ./test/run/race.sh
	@echo "No race conditions detected"

test-race-v: ## Запустить тесты с race detector и verbose output
	$(GO) test -race -v ./...

test-unit: ## Юнит-тесты внутренних пакетов
	GO=$(GO) ./test/run/unit.sh

test-integration: ## Интеграционные тесты
	GO=$(GO) ./test/run/integration.sh

test-saga: ## Тесты saga orchestrator
	$(GO) test -v ./internal/service/saga

test-kafka: ## Тесты Kafka integration
	$(GO) test -v ./internal/messaging/kafka

test-grpc: ## Тесты gRPC service
	$(GO) test -v ./internal/service/grpc

test-short: ## Быстрые тесты (пропускает длинные)
	$(GO) test -short ./...

test-count: ## Запустить тесты N раз для проверки стабильности (по умолчанию 10)
	$(GO) test -count=10 ./...

test-failfast: ## Остановить при первой ошибке
	$(GO) test -failfast ./...

##@ Coverage и бенчмарки

cover: ## Отчёт покрытия тестами (txt + HTML)
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report: coverage.html"
cover-race: ## Отчёт покрытия с race detector
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@echo "Coverage report with race detection complete"

bench: ## Запустить бенчмарки производительности
	$(GO) test -bench=. -benchmem ./...

# ========================================================================
# ЛИНТИНГ И СТАТИЧЕСКИЙ АНАЛИЗ
# ========================================================================

fmt: ## Форматирование кода (gofmt)
	$(GO) fmt ./...

vet: ## Статический анализ (go vet)
	$(GO) vet ./...

lint: vet ## Полный линтинг (vet + дополнительные проверки)
	@echo "Линтинг завершён"

# ========================================================================
# DOCKER
# ========================================================================
docker-build: ## Собрать Docker-образ
	docker build -t $(APP_NAME):$(VERSION) .

docker-run: ## Запустить локальный контейнер с пробросом портов
	docker run --rm \
		-e OMS_METRICS_ADDR=:$(OMS_METRICS_PORT) \
		-p $(OMS_GRPC_PORT):$(OMS_GRPC_PORT) \
		-p $(OMS_METRICS_PORT):$(OMS_METRICS_PORT) \
		$(APP_NAME):$(VERSION)

# ========================================================================
# DOCKER COMPOSE
# ========================================================================

compose-up: ## Запустить стек (docker compose)
	docker compose up -d

compose-down: ## Остановить стек (docker compose)
	docker compose down

compose-build-up: ## Собрать Docker-образ и поднять стек (docker compose)
	$(MAKE) docker-build
	$(MAKE) compose-up

# ========================================================================
# ДЕМОНСТРАЦИЯ И ТЕСТОВЫЕ СЦЕНАРИИ
# ========================================================================
ensure-grpcurl: ## Установить grpcurl (если отсутствует)
	@command -v grpcurl >/dev/null 2>&1 \
		|| (echo "Устанавливаю grpcurl через go install..."; $(GO) install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)
	@echo "grpcurl OK: $$(command -v grpcurl || echo "установлен в $$($(GO) env GOPATH)/bin")"
wait-health: ## Дождаться готовности сервиса (healthz)
	@echo "Ожидаю http://localhost:9090/healthz ..."; \
	for i in $$(seq 1 60); do \
		if curl -sf http://localhost:9090/healthz >/dev/null 2>&1; then \
			echo "Сервис готов"; exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Таймаут ожидания healthz"; exit 1

demo-run: ## Выполнить демонстрационный сценарий саги (Create→Pay→Get→Cancel)
	env PATH="$$($(GO) env GOPATH)/bin:$$PATH" ./scripts/saga_demo.sh
demo: ## Полный прогон: build + compose up + health + grpc сценарий
	$(MAKE) compose-build-up
	$(MAKE) wait-health
	$(MAKE) ensure-grpcurl
	$(MAKE) demo-run
	@echo "Prometheus: http://localhost:9091"
	@echo "Grafana:    http://localhost:3000 (admin/admin) → OMS → OMS Saga Overview"

demo-down: ## Остановить демо-стек
	$(MAKE) compose-down

##@ Дополнительные демо-сценарии
load: ## Нагрузочный прогон через internal runner (n=100, c=10)
	$(GO) run ./cmd/loadtest \
		-addr $${ADDR:-localhost:50051} \
		-mode $${MODE:-create} \
		-total $${TOTAL:-100} \
		-concurrency $${CONCURRENCY:-10} \
		-connections $${CONNECTIONS:-10} \
		-timeout $${TIMEOUT:-5s}

load-stress: ## Стресс-тест через internal runner (n=1000, c=50)
	$(GO) run ./cmd/loadtest \
		-addr $${ADDR:-localhost:50051} \
		-mode $${MODE:-create} \
		-total $${TOTAL:-1000} \
		-concurrency $${CONCURRENCY:-50} \
		-connections $${CONNECTIONS:-20} \
		-timeout $${TIMEOUT:-5s}
	@echo "Load complete. Проверьте Grafana и Prometheus."

load-soak: ## Time-based soak-тест (по умолчанию 10m)
	$(GO) run ./cmd/loadtest \
		-addr $${ADDR:-localhost:50051} \
		-mode $${MODE:-create-pay-cancel} \
		-duration $${DURATION:-10m} \
		-concurrency $${CONCURRENCY:-80} \
		-connections $${CONNECTIONS:-40} \
		-timeout $${TIMEOUT:-5s}
	@echo "Soak complete. Проверьте Grafana и Prometheus."

demo-refund: ## Демо сценарий с RefundOrder (Create→Pay→Refund→Get)
	env PATH="$$($(GO) env GOPATH)/bin:$$PATH" ./scripts/saga_refund_demo.sh

demo-success: ## Демо успешного сценария (для тестирования Completed/s)
	@echo "Перезапускаю стек в нормальном режиме..."
	$(MAKE) compose-down
	$(MAKE) compose-build-up
	$(MAKE) wait-health
	$(MAKE) ensure-grpcurl
	$(MAKE) demo-run
	@echo "Проверьте Grafana: Saga Completed/s должен показать ненулевые значения"

# ========================================================================
# KUBERNETES
# ========================================================================

k8s-validate: ## Валидация Kubernetes манифестов
	@echo "Проверка K8s манифестов..."
	@for file in deploy/k8s/*.yaml; do \
		echo "Checking $$file..."; \
		grep -q "apiVersion" $$file && echo "OK: $$file" || echo "FAIL: $$file"; \
	done

k8s-apply: ## Применить K8s манифесты (kubectl apply)
	kubectl apply -f deploy/k8s/

k8s-delete: ## Удалить K8s ресурсы
	kubectl delete -f deploy/k8s/

k8s-status: ## Статус pods в namespace oms
	kubectl get pods,svc,hpa,pdb -n oms

k8s-logs: ## Логи pods
	kubectl logs -n oms -l app=oms --tail=100 -f

k8s-describe: ## Описание deployment
	kubectl describe deployment oms -n oms

# ========================================================================
# HELM
# ========================================================================

helm-lint: ## Проверка Helm chart
	@command -v helm >/dev/null 2>&1 || (echo "helm не установлен. Установите: brew install helm" && exit 1)
	helm lint deploy/helm/oms

helm-template: ## Рендеринг Helm templates
	@command -v helm >/dev/null 2>&1 || (echo "helm не установлен" && exit 1)
	helm template oms deploy/helm/oms -n oms

helm-install: ## Установить через Helm
	@command -v helm >/dev/null 2>&1 || (echo "helm не установлен" && exit 1)
	helm install oms deploy/helm/oms -n oms --create-namespace

helm-upgrade: ## Обновить Helm release
	@command -v helm >/dev/null 2>&1 || (echo "helm не установлен" && exit 1)
	helm upgrade oms deploy/helm/oms -n oms

helm-uninstall: ## Удалить Helm release
	@command -v helm >/dev/null 2>&1 || (echo "helm не установлен" && exit 1)
	helm uninstall oms -n oms

helm-status: ## Статус Helm release
	@command -v helm >/dev/null 2>&1 || (echo "helm не установлен" && exit 1)
	helm status oms -n oms

helm-dry-run: ## Dry-run установки
	@command -v helm >/dev/null 2>&1 || (echo "helm не установлен" && exit 1)
	helm install oms deploy/helm/oms -n oms --dry-run --debug

# ========================================================================
# УТИЛИТЫ И ОБСЛУЖИВАНИЕ
# ========================================================================

clean: ## Удалить артефакты сборки и отчёты покрытия
	rm -rf $(BIN_DIR) coverage.out coverage.html

clean-all: clean ## Полная очистка (включая Docker images)
	docker rmi order-service:latest || true
	docker system prune -f
