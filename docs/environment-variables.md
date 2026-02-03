# Переменные окружения

Этот файл содержит описание всех переменных окружения, используемых в системе.

## Пример файла .env

Создайте файл `.env` в корне проекта со следующим содержимым:

```bash
# Конфигурация сервера
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_READ_TIMEOUT=10
SERVER_WRITE_TIMEOUT=10

# База данных PostgreSQL
DB_HOST=localhost
DB_PORT=5432
DB_USER=delivery_user
DB_PASSWORD=delivery_pass
DB_NAME=delivery_system
DB_SSL_MODE=disable

# Redis кеш
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_GROUP_ID=delivery-service
KAFKA_TOPIC_ORDERS=orders
KAFKA_TOPIC_COURIERS=couriers
KAFKA_TOPIC_LOCATIONS=locations

# Логирование
LOG_LEVEL=info
LOG_FORMAT=json
LOG_FILE=

# Геокодер
GEOCODER_PROVIDER=offline                # offline | yandex
YANDEX_GEOCODER_API_KEY=
YANDEX_GEOCODER_BASE_URL=https://geocode-maps.yandex.ru/1.x
GEOCODER_TIMEOUT_SECONDS=5

# Тарифы доставки
PRICING_BASE_FARE=100
PRICING_PER_KM=20
PRICING_MIN_FARE=150

# Аналитика
ANALYTICS_CACHE_TTL_MINUTES=10
ANALYTICS_MAX_RANGE_DAYS=365
ANALYTICS_DEFAULT_GROUP_BY=none         # none | day | week | month
ANALYTICS_DEFAULT_TOP_LIMIT=5
ANALYTICS_DEFAULT_COURIER_LIMIT=50
ANALYTICS_REQUEST_TIMEOUT_SECONDS=5

# Rate limiting
RATE_LIMIT_ENABLED=false
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW_SECONDS=60
RATE_LIMIT_KEY_PREFIX=ratelimit
```

## Описание переменных

### Сервер
- `SERVER_HOST` - IP адрес для привязки сервера (по умолчанию: 0.0.0.0)
- `SERVER_PORT` - Порт для HTTP сервера (по умолчанию: 8080)
- `SERVER_READ_TIMEOUT` - Таймаут чтения в секундах (по умолчанию: 10)
- `SERVER_WRITE_TIMEOUT` - Таймаут записи в секундах (по умолчанию: 10)

### База данных
- `DB_HOST` - Хост PostgreSQL сервера (по умолчанию: localhost)
- `DB_PORT` - Порт PostgreSQL сервера (по умолчанию: 5432)
- `DB_USER` - Имя пользователя БД (по умолчанию: delivery_user)
- `DB_PASSWORD` - Пароль пользователя БД (по умолчанию: delivery_pass)
- `DB_NAME` - Имя базы данных (по умолчанию: delivery_system)
- `DB_SSL_MODE` - Режим SSL подключения (по умолчанию: disable)

### Redis
- `REDIS_HOST` - Хост Redis сервера (по умолчанию: localhost)
- `REDIS_PORT` - Порт Redis сервера (по умолчанию: 6379)
- `REDIS_PASSWORD` - Пароль Redis (по умолчанию: пустой)
- `REDIS_DB` - Номер базы данных Redis (по умолчанию: 0)

### Kafka
- `KAFKA_BROKERS` - Список брокеров Kafka через запятую (по умолчанию: localhost:9092)
- `KAFKA_GROUP_ID` - ID группы потребителей (по умолчанию: delivery-service)
- `KAFKA_TOPIC_ORDERS` - Топик для событий заказов (по умолчанию: orders)
- `KAFKA_TOPIC_COURIERS` - Топик для событий курьеров (по умолчанию: couriers)
- `KAFKA_TOPIC_LOCATIONS` - Топик для событий местоположения (по умолчанию: locations)

### Логирование
- `LOG_LEVEL` - Уровень логирования: debug, info, warn, error (по умолчанию: info)
- `LOG_FORMAT` - Формат логов: json, text (по умолчанию: json)
- `LOG_FILE` - Путь к файлу логов (по умолчанию: пустой, логи выводятся в stdout)

### Геокодер
- `GEOCODER_PROVIDER` - Провайдер геокодинга: `offline` или `yandex` (по умолчанию: offline)
- `YANDEX_GEOCODER_API_KEY` - API ключ Яндекс Геокодера (по умолчанию: пусто)
- `YANDEX_GEOCODER_BASE_URL` - Базовый URL Яндекс Геокодера (по умолчанию: https://geocode-maps.yandex.ru/1.x)
- `GEOCODER_TIMEOUT_SECONDS` - Таймаут http-запроса к провайдеру в секундах (по умолчанию: 5)

### Тарифы доставки
- `PRICING_BASE_FARE` - Базовая стоимость доставки (по умолчанию: 100)
- `PRICING_PER_KM` - Стоимость за километр (по умолчанию: 20)
- `PRICING_MIN_FARE` - Минимальная стоимость доставки (по умолчанию: 150)

### Аналитика
- `ANALYTICS_CACHE_TTL_MINUTES` - TTL кеша аналитики в минутах (по умолчанию: 10)
- `ANALYTICS_MAX_RANGE_DAYS` - Максимальный диапазон дат в запросе аналитики (по умолчанию: 365)
- `ANALYTICS_DEFAULT_GROUP_BY` - Группировка по умолчанию: `none|day|week|month` (по умолчанию: none)
- `ANALYTICS_DEFAULT_TOP_LIMIT` - Кол-во top items по умолчанию (по умолчанию: 5)
- `ANALYTICS_DEFAULT_COURIER_LIMIT` - Лимит курьеров по умолчанию (по умолчанию: 50)
- `ANALYTICS_REQUEST_TIMEOUT_SECONDS` - Таймаут запроса аналитики (по умолчанию: 5)

### Rate limiting
- `RATE_LIMIT_ENABLED` - Включить rate limiting (по умолчанию: false)
- `RATE_LIMIT_REQUESTS` - Лимит запросов на окно (по умолчанию: 100)
- `RATE_LIMIT_WINDOW_SECONDS` - Длина окна в секундах (по умолчанию: 60)
- `RATE_LIMIT_KEY_PREFIX` - Префикс ключей в Redis (по умолчанию: ratelimit)

## Для продакшена

В продакшене рекомендуется:

1. Использовать сильные пароли для БД и Redis
2. Включить SSL для базы данных (`DB_SSL_MODE=require`)
3. Настроить аутентификацию в Redis
4. Использовать защищенные соединения с Kafka
5. Настроить уровень логирования на `warn` или `error`
6. Сохранять логи в файлы с ротацией 
