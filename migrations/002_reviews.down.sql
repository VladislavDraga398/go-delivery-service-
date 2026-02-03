-- Откат системы отзывов и рейтингов курьеров

-- Удаление триггера пересчёта рейтинга
DROP TRIGGER IF EXISTS trg_update_courier_rating_on_review ON reviews;
DROP FUNCTION IF EXISTS update_courier_rating_on_review();

-- Удаление таблицы отзывов
DROP TABLE IF EXISTS reviews;

-- Удаление дополнительных полей из couriers
ALTER TABLE couriers
    DROP COLUMN IF EXISTS rating,
    DROP COLUMN IF EXISTS total_reviews;

-- Удаление полей рейтинга и комментария из orders
ALTER TABLE orders
    DROP COLUMN IF EXISTS rating,
    DROP COLUMN IF EXISTS review_comment;