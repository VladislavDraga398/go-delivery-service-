-- Откат полей стоимости доставки и координат

ALTER TABLE orders
    DROP COLUMN IF EXISTS pickup_address,
    DROP COLUMN IF EXISTS pickup_lat,
    DROP COLUMN IF EXISTS pickup_lon,
    DROP COLUMN IF EXISTS delivery_lat,
    DROP COLUMN IF EXISTS delivery_lon,
    DROP COLUMN IF EXISTS delivery_cost;
