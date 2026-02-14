CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    status TEXT NOT NULL,
    currency TEXT NOT NULL,
    amount_minor BIGINT NOT NULL,
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orders_customer_created_at
    ON orders (customer_id, created_at DESC);

CREATE TABLE IF NOT EXISTS order_items (
    id TEXT PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    sku TEXT NOT NULL,
    qty INTEGER NOT NULL,
    price_minor BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id
    ON order_items (order_id);

CREATE TABLE IF NOT EXISTS timeline_events (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    occurred TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_timeline_order_occurred
    ON timeline_events (order_id, occurred, id);

CREATE TABLE IF NOT EXISTS outbox_messages (
    id TEXT PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload BYTEA NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_outbox_status_created_at
    ON outbox_messages (status, created_at);
