-- Добавление системы отзывов и рейтингов курьеров

-- Таблица отзывов
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    courier_id UUID NOT NULL REFERENCES couriers(id) ON DELETE CASCADE,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reviews_courier_id ON reviews(courier_id);
CREATE INDEX idx_reviews_order_id ON reviews(order_id);

-- Дополнительные поля в таблице курьеров для хранения рейтинга
ALTER TABLE couriers
    ADD COLUMN rating DECIMAL(3, 2) NOT NULL DEFAULT 0,
    ADD COLUMN total_reviews INTEGER NOT NULL DEFAULT 0;

-- Поля рейтинга и комментария для заказов (хранение отзыва в заказе для быстрых выборок)
ALTER TABLE orders
    ADD COLUMN rating INTEGER,
    ADD COLUMN review_comment TEXT;

-- Триггер для автоматического пересчёта рейтинга курьера при вставке отзыва
CREATE OR REPLACE FUNCTION update_courier_rating_on_review()
RETURNS TRIGGER AS $$
DECLARE
    new_total INTEGER;
    new_rating DECIMAL(3, 2);
BEGIN
    SELECT total_reviews + 1, ((rating * total_reviews) + NEW.rating)::DECIMAL(5,2) / (total_reviews + 1)
    INTO new_total, new_rating
    FROM couriers
    WHERE id = NEW.courier_id
    FOR UPDATE;

    UPDATE couriers
    SET total_reviews = new_total,
        rating = ROUND(new_rating::NUMERIC, 2),
        updated_at = NOW()
    WHERE id = NEW.courier_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_courier_rating_on_review
AFTER INSERT ON reviews
FOR EACH ROW
EXECUTE FUNCTION update_courier_rating_on_review();