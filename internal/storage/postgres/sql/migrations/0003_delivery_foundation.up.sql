CREATE TABLE IF NOT EXISTS couriers (
    id TEXT PRIMARY KEY,
    phone TEXT NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    vehicle_type TEXT NOT NULL CHECK (vehicle_type IN ('scooter', 'bike', 'car')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_couriers_vehicle_type_active
    ON couriers (vehicle_type, is_active);

CREATE TABLE IF NOT EXISTS courier_zones (
    courier_id TEXT NOT NULL REFERENCES couriers (id) ON DELETE CASCADE,
    zone_id TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (courier_id, zone_id)
);

CREATE INDEX IF NOT EXISTS idx_courier_zones_zone_id
    ON courier_zones (zone_id);

CREATE INDEX IF NOT EXISTS idx_courier_zones_courier_priority
    ON courier_zones (courier_id, is_primary DESC, zone_id ASC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_courier_zones_one_primary
    ON courier_zones (courier_id)
    WHERE is_primary;

CREATE TABLE IF NOT EXISTS courier_slots (
    id TEXT PRIMARY KEY,
    courier_id TEXT NOT NULL REFERENCES couriers (id) ON DELETE CASCADE,
    slot_start TIMESTAMPTZ NOT NULL,
    slot_end TIMESTAMPTZ NOT NULL,
    duration_hours INTEGER NOT NULL CHECK (duration_hours IN (4, 8, 12)),
    status TEXT NOT NULL CHECK (status IN ('planned', 'active', 'completed', 'canceled')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT courier_slots_slot_range_chk CHECK (slot_end > slot_start)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_courier_slots_unique_start
    ON courier_slots (courier_id, slot_start);

CREATE INDEX IF NOT EXISTS idx_courier_slots_lookup
    ON courier_slots (courier_id, slot_start, slot_end);

CREATE TABLE IF NOT EXISTS courier_vehicle_capabilities (
    vehicle_type TEXT PRIMARY KEY CHECK (vehicle_type IN ('scooter', 'bike', 'car')),
    max_weight_grams INTEGER NOT NULL CHECK (max_weight_grams > 0),
    max_volume_cm3 INTEGER NOT NULL CHECK (max_volume_cm3 > 0),
    max_orders_per_trip INTEGER NOT NULL CHECK (max_orders_per_trip > 0),
    updated_at TIMESTAMPTZ NOT NULL
);

INSERT INTO courier_vehicle_capabilities (vehicle_type, max_weight_grams, max_volume_cm3, max_orders_per_trip, updated_at)
VALUES
    ('scooter', 5000, 35000, 2, NOW()),
    ('bike', 10000, 65000, 3, NOW()),
    ('car', 25000, 250000, 10, NOW())
ON CONFLICT (vehicle_type) DO NOTHING;
