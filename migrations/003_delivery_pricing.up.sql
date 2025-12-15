-- Добавление полей для расчета стоимости доставки и координат

ALTER TABLE orders
    ADD COLUMN pickup_address TEXT NOT NULL DEFAULT '',
    ADD COLUMN pickup_lat DECIMAL(10, 8),
    ADD COLUMN pickup_lon DECIMAL(11, 8),
    ADD COLUMN delivery_lat DECIMAL(10, 8),
    ADD COLUMN delivery_lon DECIMAL(11, 8),
    ADD COLUMN delivery_cost DECIMAL(10, 2) NOT NULL DEFAULT 0;

-- Удаляем значение по умолчанию для pickup_address, чтобы требовать заполнение новых записей
ALTER TABLE orders ALTER COLUMN pickup_address DROP DEFAULT;
