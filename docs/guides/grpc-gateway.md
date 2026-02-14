# gRPC-Gateway

REST API поверх gRPC через gRPC-Gateway.

> Статус на 2026-02-12: в runtime OMS сейчас включён только gRPC сервер; REST gateway остаётся как планируемое расширение.

## Обзор

gRPC-Gateway автоматически генерирует RESTful HTTP API из gRPC сервиса, позволяя клиентам использовать как gRPC, так и REST.

### Преимущества

- Единый источник истины (proto файл)
- Автоматическая генерация REST endpoints
- JSON  Protobuf конвертация
- Swagger/OpenAPI документация
- Обратная совместимость

## Установка

### Предварительные требования

```bash
# Установить protoc
brew install protobuf

# Установить плагины
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
```

### Генерация кода

```bash
# Генерация gRPC + gRPC-Gateway
make proto

# Или вручную
protoc \
  -I. \
  -I proto \
  --go_out=paths=source_relative:. \
  --go-grpc_out=paths=source_relative:. \
  --grpc-gateway_out=paths=source_relative:. \
  proto/oms/v1/order_service.proto
```

## REST API Endpoints

### CreateOrder

**gRPC:**
```bash
grpcurl -plaintext -d '{...}' localhost:50051 oms.v1.OrderService/CreateOrder
```

**REST:**
```bash
curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
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
  }'
```

### GetOrder

**gRPC:**
```bash
grpcurl -plaintext -d '{"order_id": "order-123"}' \
  localhost:50051 oms.v1.OrderService/GetOrder
```

**REST:**
```bash
curl http://localhost:8080/v1/orders/order-123
```

### ListOrders

**gRPC:**
```bash
grpcurl -plaintext -d '{"page_size": 10}' \
  localhost:50051 oms.v1.OrderService/ListOrders
```

**REST:**
```bash
curl "http://localhost:8080/v1/orders?page_size=10&status=ORDER_STATUS_CONFIRMED"
```

### PayOrder

**gRPC:**
```bash
grpcurl -plaintext -d '{"order_id": "order-123"}' \
  localhost:50051 oms.v1.OrderService/PayOrder
```

**REST:**
```bash
curl -X POST http://localhost:8080/v1/orders/order-123/pay \
  -H "Content-Type: application/json" \
  -d '{}'
```

### CancelOrder

**gRPC:**
```bash
grpcurl -plaintext -d '{"order_id": "order-123", "reason": "Customer request"}' \
  localhost:50051 oms.v1.OrderService/CancelOrder
```

**REST:**
```bash
curl -X POST http://localhost:8080/v1/orders/order-123/cancel \
  -H "Content-Type: application/json" \
  -d '{"reason": "Customer request"}'
```

### RefundOrder

**gRPC:**
```bash
grpcurl -plaintext -d '{"order_id": "order-123"}' \
  localhost:50051 oms.v1.OrderService/RefundOrder
```

**REST:**
```bash
curl -X POST http://localhost:8080/v1/orders/order-123/refund \
  -H "Content-Type: application/json" \
  -d '{
    "amount": {
      "currency": "USD",
      "amount_minor": 5000
    },
    "reason": "Partial refund"
  }'
```

## Конфигурация

### Proto файл

```protobuf
service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse) {
    option (google.api.http) = {
      post: "/v1/orders"
      body: "*"
    };
  }
  
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse) {
    option (google.api.http) = {
      get: "/v1/orders/{order_id}"
    };
  }
}
```

### HTTP Mapping

| gRPC Method | HTTP Method | Path |
|-------------|-------------|------|
| CreateOrder | POST | `/v1/orders` |
| GetOrder | GET | `/v1/orders/{order_id}` |
| ListOrders | GET | `/v1/orders` |
| PayOrder | POST | `/v1/orders/{order_id}/pay` |
| CancelOrder | POST | `/v1/orders/{order_id}/cancel` |
| RefundOrder | POST | `/v1/orders/{order_id}/refund` |

## Запуск

### Локально

```go
package main

import (
    "context"
    "net/http"
    
    "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    
    omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

func main() {
    ctx := context.Background()
    
    // Создаём gRPC-Gateway mux
    mux := runtime.NewServeMux()
    
    // Регистрируем сервис
    opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
    err := omsv1.RegisterOrderServiceHandlerFromEndpoint(
        ctx,
        mux,
        "localhost:50051", // gRPC endpoint
        opts,
    )
    if err != nil {
        panic(err)
    }
    
    // Запускаем HTTP сервер
    http.ListenAndServe(":8080", mux)
}
```

### Docker

```dockerfile
# Добавить в Dockerfile
EXPOSE 8080

# Запустить оба сервера
CMD ["sh", "-c", "./order-service & ./gateway-service"]
```

## OpenAPI/Swagger

### Генерация OpenAPI спецификации

```bash
protoc \
  -I. \
  -I proto \
  --openapiv2_out=. \
  --openapiv2_opt=logtostderr=true \
  proto/oms/v1/order_service.proto
```

Создаст файл `proto/oms/v1/order_service.swagger.json`

### Swagger UI

```yaml
# docker-compose.yml
services:
  swagger-ui:
    image: swaggerapi/swagger-ui
    ports:
      - "8081:8080"
    environment:
      SWAGGER_JSON: /swagger/order_service.swagger.json
    volumes:
      - ./proto/oms/v1:/swagger
```

Доступ: http://localhost:8081

## Отладка

### Логирование

```go
mux := runtime.NewServeMux(
    runtime.WithErrorHandler(customErrorHandler),
    runtime.WithIncomingHeaderMatcher(customHeaderMatcher),
)
```

### CORS

```go
func allowCORS(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        h.ServeHTTP(w, r)
    })
}

http.ListenAndServe(":8080", allowCORS(mux))
```

## Best Practices

### 1. Версионирование API

```protobuf
option (google.api.http) = {
  post: "/v1/orders"  // Версия в URL
};
```

### 2. Обработка ошибок

```go
func customErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(runtime.HTTPStatusFromCode(status.Code(err)))
    
    json.NewEncoder(w).Encode(map[string]string{
        "error": err.Error(),
        "code":  status.Code(err).String(),
    })
}
```

### 3. Метаданные

```go
runtime.WithMetadata(func(ctx context.Context, req *http.Request) metadata.MD {
    return metadata.New(map[string]string{
        "x-request-id": req.Header.Get("X-Request-ID"),
    })
})
```

### 4. Кастомные маршруты

```go
mux.HandlePath("GET", "/health", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
})
```

## Тестирование

### Unit тесты

```go
func TestGateway(t *testing.T) {
    ctx := context.Background()
    mux := runtime.NewServeMux()
    
    // Mock gRPC server
    // ...
    
    req := httptest.NewRequest("GET", "/v1/orders/test-123", nil)
    w := httptest.NewRecorder()
    
    mux.ServeHTTP(w, req)
    
    if w.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", w.Code)
    }
}
```

### Integration тесты

```bash
# Запустить сервисы
make run

# Тестировать REST API
curl http://localhost:8080/v1/orders
```

## Дополнительные ресурсы

- [gRPC-Gateway Documentation](https://grpc-ecosystem.github.io/grpc-gateway/)
- [Google API Design Guide](https://cloud.google.com/apis/design)
- [HTTP/JSON to gRPC Mapping](https://github.com/googleapis/googleapis/blob/master/google/api/http.proto)

---

** gRPC-Gateway готов к использованию!**
