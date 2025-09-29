#########################
# Builder stage
#########################
FROM golang:1.25.1-alpine AS builder

# Build-time metadata
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
ARG LDFLAGS="-s -w -X github.com/vladislavdragonenkov/oms/internal/version.version=$VERSION -X github.com/vladislavdragonenkov/oms/internal/version.commit=$COMMIT -X github.com/vladislavdragonenkov/oms/internal/version.date=$DATE"

WORKDIR /src

# Enable module mode and cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy sources
COPY . .

# Build static binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags "$LDFLAGS" -o /out/order-service ./cmd/order-service

#########################
# Runtime stage
#########################
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Re-declare args for label interpolation
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

WORKDIR /app

# Default envs can be overridden at runtime
ENV OMS_GRPC_ADDR=:50051 \
    OMS_METRICS_ADDR=:9090

# Copy binary
COPY --from=builder /out/order-service /app/order-service

EXPOSE 50051 9090

USER nonroot

LABEL org.opencontainers.image.title="order-service" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$COMMIT" \
      org.opencontainers.image.created="$DATE" \
      org.opencontainers.image.source="https://github.com/vladislavdragonenkov/oms"

ENTRYPOINT ["/app/order-service"]