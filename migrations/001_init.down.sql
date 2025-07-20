-- Удаление триггеров
DROP TRIGGER IF EXISTS log_order_status_change_trigger ON orders;
DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;
DROP TRIGGER IF EXISTS update_couriers_updated_at ON couriers;

-- Удаление функций
DROP FUNCTION IF EXISTS log_order_status_change();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Удаление индексов
DROP INDEX IF EXISTS idx_order_status_history_order_id;
DROP INDEX IF EXISTS idx_couriers_status;
DROP INDEX IF EXISTS idx_order_items_order_id;
DROP INDEX IF EXISTS idx_orders_created_at;
DROP INDEX IF EXISTS idx_orders_courier_id;
DROP INDEX IF EXISTS idx_orders_status;

-- Удаление таблиц
DROP TABLE IF EXISTS order_status_history;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS couriers;

-- Удаление расширения (осторожно, может использоваться другими приложениями)
-- DROP EXTENSION IF EXISTS "uuid-ossp"; 