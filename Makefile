.SHELL := /bin/bash

# -------- Конфигурация --------
GO              ?= go
APP_NAME        ?= order-service
CMD_PATH        ?= ./cmd/order-service
BIN_DIR         ?= bin
BIN             ?= $(BIN_DIR)/$(APP_NAME)

# Адреса по умолчанию можно переопределить: `make run OMS_GRPC_ADDR=":50052"`
OMS_GRPC_ADDR    ?= :50051
OMS_METRICS_ADDR ?= :9090

# Порты для docker-run
DOCKER_GRPC_PORT    ?= 50051
DOCKER_METRICS_PORT ?= 9090

.PHONY: all proto tidy deps build run test test-unit test-integration cover clean lint docker-build docker-run

# Цель по умолчанию
all: build

# -------- Codegen / deps --------
# Генерация gRPC/Protobuf кода из proto/oms/v1/order_service.proto
proto:
	protoc \
		--go_out=paths=source_relative:. \
		--go-grpc_out=paths=source_relative:. \
		proto/oms/v1/order_service.proto

# Обновление зависимостей модуля
tidy:
	$(GO) mod tidy

deps: tidy

# -------- Build / Run --------
build: $(BIN)

$(BIN):
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) $(CMD_PATH)

run:
	OMS_GRPC_ADDR=$(OMS_GRPC_ADDR) OMS_METRICS_ADDR=$(OMS_METRICS_ADDR) $(GO) run $(CMD_PATH)

# -------- Testing --------
test:
	$(GO) test ./...

test-unit:
	$(GO) test ./internal/... ./proto/... -count=1

test-integration:
	$(GO) test -v ./test/integration -count=1

cover:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report: coverage.html"

# -------- Maintenance --------
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

lint:
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| echo "golangci-lint не установлен: пропускаем lint"

# -------- Docker (опционально, нужен Dockerfile) --------
docker-build:
	docker build -t $(APP_NAME):latest .

docker-run:
	docker run --rm \
		-e OMS_GRPC_ADDR=:$(DOCKER_GRPC_PORT) \
		-e OMS_METRICS_ADDR=:$(DOCKER_METRICS_PORT) \
		-p $(DOCKER_GRPC_PORT):$(DOCKER_GRPC_PORT) \
		-p $(DOCKER_METRICS_PORT):$(DOCKER_METRICS_PORT) \
		$(APP_NAME):latest
docker-compose:
	docker-compose up

docker-compose-down:
	docker-compose down

docker-compose-restart:
	docker-compose restart

docker-compose-logs:
	docker-compose logs

docker-compose-logs-follow:
	docker-compose logs -f