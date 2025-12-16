-- Промокоды и скидки

CREATE TABLE promo_codes (
    code VARCHAR(64) PRIMARY KEY,
    discount_type VARCHAR(20) NOT NULL CHECK (discount_type IN ('fixed', 'percent', 'free_delivery')),
    amount DECIMAL(10, 2) NOT NULL DEFAULT 0,
    max_uses INTEGER NOT NULL DEFAULT 0, -- 0 = безлимит
    used_count INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMP WITH TIME ZONE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Триггер для обновления updated_at
CREATE TRIGGER update_promo_codes_updated_at
    BEFORE UPDATE ON promo_codes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Дополнительные поля в orders для хранения применённого промокода
ALTER TABLE orders
    ADD COLUMN promo_code VARCHAR(64),
    ADD COLUMN discount_amount DECIMAL(10, 2) NOT NULL DEFAULT 0;
