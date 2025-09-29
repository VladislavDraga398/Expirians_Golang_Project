-- Инициализация базовой схемы OMS (Фаза 1)
-- Создаёт ключевые таблицы: orders, order_items, payments, inventory_reservations,
-- outbox и idempotency_keys. При дальнейшем развитии проекта будут добавлены
-- дополнительные миграции.

CREATE TABLE orders (
    id UUID PRIMARY KEY,
    customer_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'reserved', 'paid', 'confirmed', 'canceled', 'refunded')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    currency CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_customer_created ON orders (customer_id, created_at DESC);
CREATE INDEX idx_orders_created ON orders (created_at DESC);

CREATE TABLE order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    sku TEXT NOT NULL,
    qty INTEGER NOT NULL CHECK (qty > 0),
    price_minor BIGINT NOT NULL CHECK (price_minor >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order ON order_items (order_id);
CREATE INDEX idx_order_items_sku ON order_items (sku);

CREATE TABLE payments (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    external_id TEXT UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'authorized', 'captured', 'refunded', 'failed')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order ON payments (order_id);
CREATE INDEX idx_payments_status ON payments (status);
CREATE INDEX idx_payments_provider_ext ON payments (provider, external_id);

CREATE TABLE inventory_reservations (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    sku TEXT NOT NULL,
    qty INTEGER NOT NULL CHECK (qty > 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'reserved', 'released', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reservations_order ON inventory_reservations (order_id);
CREATE INDEX idx_reservations_sku ON inventory_reservations (sku);
CREATE INDEX idx_reservations_status ON inventory_reservations (status);

CREATE TABLE outbox (
    id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'sent', 'failed')) DEFAULT 'pending',
    attempt_cnt INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_status_created ON outbox (status, created_at);
CREATE INDEX idx_outbox_aggregate ON outbox (aggregate_type, aggregate_id);

CREATE TABLE idempotency_keys (
    key TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    response_body JSONB,
    http_status INTEGER,
    status TEXT NOT NULL CHECK (status IN ('processing', 'done', 'failed')),
    ttl_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_idempotency_ttl ON idempotency_keys (ttl_at);
CREATE INDEX idx_idempotency_status ON idempotency_keys (status);
