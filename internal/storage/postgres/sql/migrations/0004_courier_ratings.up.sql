CREATE TABLE IF NOT EXISTS courier_ratings (
    id TEXT PRIMARY KEY,
    courier_id TEXT NOT NULL REFERENCES couriers (id) ON DELETE CASCADE,
    score SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    tags JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tags) = 'array'),
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_courier_ratings_courier_created
    ON courier_ratings (courier_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_courier_ratings_courier_score
    ON courier_ratings (courier_id, score);
