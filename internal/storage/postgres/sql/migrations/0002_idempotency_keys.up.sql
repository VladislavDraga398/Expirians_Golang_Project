CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    response_body BYTEA,
    http_status INTEGER,
    status TEXT NOT NULL CHECK (status IN ('processing', 'done', 'failed')),
    ttl_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_ttl_at
    ON idempotency_keys (ttl_at);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_status
    ON idempotency_keys (status);
