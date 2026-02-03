-- Откат промокодов и скидок

DROP TRIGGER IF EXISTS update_promo_codes_updated_at ON promo_codes;
DROP TABLE IF EXISTS promo_codes;

ALTER TABLE orders
    DROP COLUMN IF EXISTS promo_code,
    DROP COLUMN IF EXISTS discount_amount;
