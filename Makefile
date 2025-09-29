.SHELL := /bin/bash

# =============================
#  OMS Makefile (дружественный)
#  Подсказка: выполните `make help`
# =============================

# -------- Конфигурация --------
GO              ?= go
APP_NAME        ?= order-service
CMD_PATH        ?= ./cmd/order-service
BIN_DIR         ?= bin
BIN             ?= $(BIN_DIR)/$(APP_NAME)
GOBIN           ?= $(shell $(GO) env GOPATH)/bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS ?= -s -w \
  -X github.com/vladislavdragonenkov/oms/internal/version.version=$(VERSION) \
  -X github.com/vladislavdragonenkov/oms/internal/version.commit=$(COMMIT) \
  -X github.com/vladislavdragonenkov/oms/internal/version.date=$(DATE)

.PHONY: all proto generate tidy deps build run test test-unit test-integration cover clean fmt vet lint lint-install staticcheck hadolint help docker-build docker-run compose-up compose-down compose-build-up buildx-create buildx-build buildx-push hooks-install commit-template ensure-grpcurl wait-health demo-run demo demo-down ensure-ghz load demo-refund demo-fail-reserve demo-fail-pay demo-success

# -------- Codegen / deps --------
# Генерация gRPC/Protobuf кода из proto/oms/v1/order_service.proto
proto: ## Генерация gRPC/Protobuf кода
	protoc \
		--go_out=paths=source_relative:. \
		--go-grpc_out=paths=source_relative:. \
		proto/oms/v1/order_service.proto

generate: proto ## Сгенерировать код и проверить чистоту git-дерева
	@git diff --quiet || (echo "\nЕсть несохранённые изменения после генерации. Добавьте их в git." && exit 1)


# -------- Build / Run --------
build: $(BIN) ## Собрать бинарник в папку bin/

$(BIN): ## Сборка бинаря с вшитыми метаданными версии
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BIN) $(CMD_PATH)

run: ## Запустить сервис локально (go run)
	OMS_GRPC_ADDR=$(OMS_GRPC_ADDR) OMS_METRICS_ADDR=$(OMS_METRICS_ADDR) $(GO) run $(CMD_PATH)

# -------- Testing --------
test: ## Запустить все тесты
	$(GO) test ./...

test-unit: ## Юнит-тесты внутренних пакетов
	$(GO) test ./internal/... ./proto/... -count=1

test-integration: ## Интеграционные тесты
	$(GO) test -v ./test/integration -count=1

cover: ## Отчёт покрытия тестами (txt + HTML)
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report: coverage.html"


vet: ## Статический анализ (go vet)
	$(GO) vet ./...

help: ## Показать список целей Makefile
	@echo "Доступные цели:"
	@awk -F':|##' '/^[a-zA-Z0-9_.-]+:/{printf "  %-22s %s\n", $$1, $$3}' $(MAKEFILE_LIST)

# -------- Docker --------
docker-build: ## Собрать Docker-образ
	docker build -t $(APP_NAME):$(VERSION) .

docker-run: ## Запустить локальный контейнер с пробросом портов
	docker run --rm \
		-e OMS_GRPC_ADDR=:$(OMS_GRPC_PORT) \
		-e OMS_METRICS_ADDR=:$(OMS_METRICS_PORT) \
		-p $(OMS_GRPC_PORT):$(OMS_GRPC_PORT) \
		-p $(OMS_METRICS_PORT):$(OMS_METRICS_PORT) \
		$(APP_NAME):$(VERSION)

# -------- Maintenance --------
clean: ## Удалить артефакты сборки и отчёты покрытия
	rm -rf $(BIN_DIR) coverage.out coverage.html

docker-compose: ## Запустить стек (legacy docker-compose)
	docker-compose up

docker-compose-down: ## Остановить стек (legacy docker-compose)
	docker-compose down

docker-compose-restart: ## Перезапуск стека (legacy docker-compose)
	docker-compose restart

docker-compose-logs: ## Логи стека (legacy docker-compose)
	docker-compose logs

docker-compose-logs-follow: ## Логи с последованием (legacy docker-compose)
	docker-compose logs -f

compose-up: ## Запустить стек (docker compose)
	docker compose up -d

compose-down: ## Остановить стек (docker compose)
	docker compose down

compose-build-up: ## Собрать Docker-образ и поднять стек (docker compose)
	$(MAKE) docker-build
	$(MAKE) compose-up

# -------- One-click Demo --------
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

# -------- Load & Extra Demos --------
ensure-ghz: ## Установить ghz (если отсутствует)
	@command -v ghz >/dev/null 2>&1 \
		|| (echo "Устанавливаю ghz через go install..."; $(GO) install github.com/bojand/ghz/cmd/ghz@latest)
	@echo "ghz OK: $$(command -v ghz || echo "установлен в $$($(GO) env GOPATH)/bin")"

load: ensure-ghz ## Нагрузочный прогон CreateOrder для метрик (n=100, c=10)
	env PATH="$$($(GO) env GOPATH)/bin:$$PATH" ghz --insecure \
		--proto proto/oms/v1/order_service.proto \
		--call oms.v1.OrderService.CreateOrder \
		--data '{"customer_id":"load","currency":"USD","items":[{"sku":"sku","qty":1,"price":{"currency":"USD","amount_minor":100}}]}' \
		-n 100 -c 10 localhost:50051
	@echo "Load complete. Проверьте Grafana и Prometheus."

demo-refund: ## Демо сценарий с RefundOrder (Create→Pay→Refund→Get)
	env PATH="$$($(GO) env GOPATH)/bin:$$PATH" ./scripts/saga_refund_demo.sh

demo-fail-reserve: ## Демо с принудительной ошибкой резерва (для тестирования Failed/s)
	@echo "Перезапускаю стек с OMS_FAIL_RESERVE=true..."
	$(MAKE) compose-down
	OMS_FAIL_RESERVE=true $(MAKE) compose-build-up
	$(MAKE) wait-health
	$(MAKE) ensure-grpcurl
	$(MAKE) demo-run
	@echo "Проверьте Grafana: Saga Failed/s должен показать ненулевые значения"

demo-fail-pay: ## Демо с принудительной ошибкой оплаты (для тестирования Failed/s)
	@echo "Перезапускаю стек с OMS_FAIL_PAY=true..."
	$(MAKE) compose-down
	OMS_FAIL_PAY=true $(MAKE) compose-build-up
	$(MAKE) wait-health
	$(MAKE) ensure-grpcurl
	$(MAKE) demo-run
	@echo "Проверьте Grafana: Saga Failed/s должен показать ненулевые значения"

demo-success: ## Демо успешного сценария (для тестирования Completed/s)
	@echo "Перезапускаю стек в нормальном режиме..."
	$(MAKE) compose-down
	$(MAKE) compose-build-up
	$(MAKE) wait-health
	$(MAKE) ensure-grpcurl
	$(MAKE) demo-run
	@echo "Проверьте Grafana: Saga Completed/s должен показать ненулевые значения"