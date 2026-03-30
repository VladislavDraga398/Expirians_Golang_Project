ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS delivery_fee_minor BIGINT NOT NULL DEFAULT 0;

UPDATE orders AS o
SET delivery_fee_minor = GREATEST(
    o.amount_minor - COALESCE(
        (
            SELECT SUM(oi.qty * oi.price_minor)
            FROM order_items AS oi
            WHERE oi.order_id = o.id
        ),
        0
    ),
    0
)
WHERE o.delivery_fee_minor = 0;
