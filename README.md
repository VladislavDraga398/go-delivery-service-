# Система учета доставки заказов

Полнофункциональная система управления доставкой заказов, построенная на Go с использованием современных технологий: PostgreSQL, Redis, Kafka и Docker.

## 📋 Содержание

- [Описание проекта](#описание-проекта)
- [Архитектура](#архитектура)
- [Технологии](#технологии)
- [Требования](#требования)
- [Быстрый старт](#быстрый-старт)
- [API документация](#api-документация)
- [Конфигурация](#конфигурация)
- [Развертывание](#развертывание)
- [Мониторинг](#мониторинг)
- [Разработка](#разработка)
- [Задачи для доработки](#задачи-для-доработки)

## 🎯 Описание проекта

Система учета доставки заказов - это учебный проект, демонстрирующий лучшие практики разработки на Go. Система позволяет:

- **Управлять заказами**: создание, отслеживание статусов, обновление
- **Управлять курьерами**: регистрация, назначение заказов, отслеживание местоположения
- **Обрабатывать события**: асинхронная обработка через Kafka
- **Кешировать данные**: быстрый доступ через Redis
- **Мониторить состояние**: health checks и метрики

## 🏗 Архитектура

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │────│   API Gateway   │────│   Load Balancer │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                 │
                        ┌─────────────────┐
                        │   HTTP Server   │
                        │   (Go std lib)  │
                        └─────────────────┘
                                 │
                ┌────────────────┼────────────────┐
                │                │                │
        ┌───────────────┐ ┌─────────────┐ ┌─────────────┐
        │   Handlers    │ │  Services   │ │ Middleware  │
        └───────────────┘ └─────────────┘ └─────────────┘
                │                │                │
        ┌───────────────┐ ┌─────────────┐ ┌─────────────┐
        │   Kafka       │ │ PostgreSQL  │ │    Redis    │
        │  (Events)     │ │(Primary DB) │ │   (Cache)   │
        └───────────────┘ └─────────────┘ └─────────────┘
```

### Компоненты системы:

1. **HTTP API**: RESTful API без внешних фреймворков
2. **Business Logic**: Сервисы для обработки бизнес-логики
3. **Data Layer**: PostgreSQL для персистентности, Redis для кеширования
4. **Event System**: Kafka для асинхронной обработки событий
5. **Monitoring**: Health checks и логирование

## 🛠 Технологии

- **Язык**: Go 1.21+
- **База данных**: PostgreSQL 15
- **Кеш**: Redis 7
- **Очереди**: Apache Kafka
- **Контейнеризация**: Docker & Docker Compose
- **Логирование**: Structured logging (JSON)

### Основные зависимости:

```go
github.com/IBM/sarama v1.41.2          // Kafka client
github.com/go-redis/redis/v8 v8.11.5   // Redis client
github.com/lib/pq v1.10.9              // PostgreSQL driver
github.com/google/uuid v1.3.1          // UUID generation
github.com/sirupsen/logrus v1.9.3      // Structured logging
```

## 📋 Требования

- **Go**: версия 1.21 или выше
- **Docker**: версия 20.0 или выше
- **Docker Compose**: версия 2.0 или выше
- **Make**: для удобства разработки (опционально)

## 🚀 Быстрый старт

### 1. Клонирование репозитория

```bash
git clone <repository-url>
cd delivery-system
```

### 2. Запуск инфраструктуры

```bash
# Запуск всех сервисов (PostgreSQL, Redis, Kafka, приложение)
docker-compose up -d

# Или запуск только инфраструктуры для локальной разработки
docker-compose up -d postgres redis kafka zookeeper
```

### 3. Запуск приложения локально

```bash
# Установка зависимостей
go mod download

# Запуск приложения
go run cmd/server/main.go
```

### 4. Проверка работоспособности

```bash
# Health check
curl http://localhost:8080/health

# Создание курьера
curl -X POST http://localhost:8080/api/couriers \
  -H "Content-Type: application/json" \
  -d '{"name": "Иван Петров", "phone": "+7(999)123-45-67"}'

# Создание заказа
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_name": "Анна Смирнова",
    "customer_phone": "+7(999)987-65-43",
    "pickup_address": "Москва, ул. Производственная, д. 1",
    "delivery_address": "Москва, ул. Ленина, д. 10, кв. 5",
    "items": [
      {"name": "Пицца Маргарита", "quantity": 1, "price": 500.00},
      {"name": "Кока-кола 0.5л", "quantity": 2, "price": 100.00}
    ]
  }'
```

## 📚 API документация

### Заказы (Orders)

#### Создание заказа
```http
POST /api/orders
Content-Type: application/json

{
  "customer_name": "Имя клиента",
  "customer_phone": "+7(999)123-45-67",
  "pickup_address": "Адрес забора заказа",
  "delivery_address": "Адрес доставки",
  "items": [
    {
      "name": "Название товара",
      "quantity": 1,
      "price": 100.50
    }
  ],
  "delivery_cost": 250.00
}
```

`pickup_address` обязателен (нужен для геокодирования и расчёта доставки).
`delivery_cost` — необязательный ручной override стоимости доставки; если не указан,
стоимость считается по расстоянию (тарифы `PRICING_*`). Координаты можно передать
явно (`pickup_lat/pickup_lon`, `delivery_lat/delivery_lon`) — иначе адреса геокодируются
через Nominatim (OSM) или Яндекс (`GEOCODER_PROVIDER`).

#### Получение заказа
```http
GET /api/orders/{order_id}
```

#### Получение списка заказов
```http
GET /api/orders?status=created&courier_id={uuid}&limit=20&offset=0
```

#### Обновление статуса заказа
```http
PUT /api/orders/{order_id}/status
Content-Type: application/json

{
  "status": "in_delivery",
  "courier_id": "uuid-курьера"
}
```

### Курьеры (Couriers)

#### Создание курьера
```http
POST /api/couriers
Content-Type: application/json

{
  "name": "Имя курьера",
  "phone": "+7(999)123-45-67"
}
```

#### Получение курьера
```http
GET /api/couriers/{courier_id}
```

#### Получение списка курьеров
```http
GET /api/couriers?status=available&min_rating=4.5&limit=20&offset=0
```

#### Получение доступных курьеров
```http
GET /api/couriers/available?min_rating=4.5&order_by=rating
```

#### Обновление статуса курьера
```http
PUT /api/couriers/{courier_id}/status
Content-Type: application/json

{
  "status": "available",
  "current_lat": 55.7558,
  "current_lon": 37.6176
}
```

#### Назначение заказа курьеру
```http
POST /api/couriers/{courier_id}/assign
Content-Type: application/json

{
  "order_id": "uuid-заказа"
}
```

### Статусы

#### Статусы заказов:
- `created` - создан
- `accepted` - принят
- `preparing` - готовится
- `ready` - готов к доставке
- `in_delivery` - в доставке
- `delivered` - доставлен
- `cancelled` - отменен

#### Статусы курьеров:
- `offline` - не в сети
- `available` - доступен
- `busy` - занят

### Health Check

```http
GET /health              # Полная проверка всех компонентов
GET /health/readiness    # Проверка готовности к обработке запросов
GET /health/liveness     # Проверка жизнеспособности приложения
```

## ⚙️ Конфигурация

Конфигурация осуществляется через переменные окружения:

### Сервер
```bash
SERVER_HOST=0.0.0.0          # Хост сервера
SERVER_PORT=8080             # Порт сервера
SERVER_READ_TIMEOUT=10       # Таймаут чтения (сек)
SERVER_WRITE_TIMEOUT=10      # Таймаут записи (сек)
```

### База данных
```bash
DB_HOST=localhost            # Хост PostgreSQL
DB_PORT=5432                # Порт PostgreSQL
DB_USER=delivery_user       # Пользователь БД
DB_PASSWORD=delivery_pass   # Пароль БД
DB_NAME=delivery_system     # Название БД
DB_SSL_MODE=disable         # Режим SSL
```

### Redis
```bash
REDIS_HOST=localhost        # Хост Redis
REDIS_PORT=6379            # Порт Redis
REDIS_PASSWORD=            # Пароль Redis (если есть)
REDIS_DB=0                 # Номер БД Redis
```

### Kafka
```bash
KAFKA_BROKERS=localhost:9092              # Брокеры Kafka
KAFKA_GROUP_ID=delivery-service           # ID группы потребителей
KAFKA_TOPIC_ORDERS=orders                 # Топик для заказов
KAFKA_TOPIC_COURIERS=couriers             # Топик для курьеров
KAFKA_TOPIC_LOCATIONS=locations           # Топик для местоположений
```

### Логирование
```bash
LOG_LEVEL=info             # Уровень логирования (debug, info, warn, error)
LOG_FORMAT=json            # Формат логов (json, text)
LOG_FILE=                  # Файл логов (пустой = stdout)
```

## 🐳 Развертывание

### Локальная разработка

1. **Запуск инфраструктуры**:
```bash
docker-compose up -d postgres redis kafka zookeeper
```

2. **Миграции БД**:
```bash
# Выполняются автоматически при запуске PostgreSQL
# Файлы миграций находятся в ./migrations/
```

3. **Запуск приложения**:
```bash
go run cmd/server/main.go
```

### Production

1. **Полный запуск через Docker Compose**:
```bash
docker-compose up -d
```

2. **Проверка статуса**:
```bash
docker-compose ps
curl http://localhost:8080/health
```

3. **Просмотр логов**:
```bash
docker-compose logs -f delivery-app
```

### Kubernetes (для продакшена)

```yaml
# Пример deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: delivery-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: delivery-system
  template:
    metadata:
      labels:
        app: delivery-system
    spec:
      containers:
      - name: delivery-system
        image: delivery-system:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          value: "postgres-service"
        - name: REDIS_HOST
          value: "redis-service"
        - name: KAFKA_BROKERS
          value: "kafka-service:9092"
        livenessProbe:
          httpGet:
            path: /health/liveness
            port: 8080
          initialDelaySeconds: 30
        readinessProbe:
          httpGet:
            path: /health/readiness
            port: 8080
          initialDelaySeconds: 5
```

## 📊 Мониторинг

### Health Checks

Система предоставляет несколько эндпоинтов для мониторинга:

- `/health` - полная проверка здоровья всех компонентов
- `/health/readiness` - готовность к обслуживанию запросов
- `/health/liveness` - жизнеспособность приложения

### Логирование

Система использует структурированное логирование в формате JSON:

```json
{
  "level": "info",
  "msg": "Order created successfully",
  "order_id": "123e4567-e89b-12d3-a456-426614174000",
  "customer_name": "Анна Смирнова",
  "total_amount": 700,
  "time": "2024-01-15T10:30:00Z"
}
```

### Метрики (рекомендуемые для добавления)

- Количество созданных заказов
- Среднее время доставки
- Количество активных курьеров
- Производительность API (latency, throughput)

## 👨‍💻 Разработка

### Структура проекта

```
delivery-system/
├── cmd/
│   └── server/           # Точка входа приложения
├── internal/
│   ├── config/          # Конфигурация
│   ├── database/        # Работа с БД
│   ├── handlers/        # HTTP обработчики
│   ├── kafka/           # Kafka producer/consumer
│   ├── logger/          # Логирование
│   ├── models/          # Модели данных
│   ├── redis/           # Redis клиент
│   └── services/        # Бизнес-логика
├── migrations/          # SQL миграции
├── docker/             # Docker файлы
├── docs/               # Документация
├── docker-compose.yml  # Локальная разработка
├── Dockerfile          # Production образ
├── go.mod              # Go модули
└── README.md
```

### Соглашения по коду

1. **Именование**: camelCase для переменных, PascalCase для типов
2. **Обработка ошибок**: всегда обрабатывайте ошибки явно
3. **Логирование**: используйте структурированные логи с контекстом
4. **Тесты**: покрытие должно быть не менее 80%

### Добавление новых API

1. Создайте модель в `internal/models/`
2. Добавьте бизнес-логику в `internal/services/`
3. Создайте HTTP обработчик в `internal/handlers/`
4. Зарегистрируйте маршрут в `cmd/server/main.go`

### Миграции БД

Для добавления новой миграции:

1. Создайте файлы `XXX_name.up.sql` и `XXX_name.down.sql` в папке `migrations/`
2. Перезапустите PostgreSQL контейнер

## 🎯 Задачи для доработки

### 1. Система рейтингов курьеров и отзывов клиентов
**Сложность**: Средняя  
**Время**: 6-8 часов

**Описание**: Реализовать систему оценки качества доставки для повышения сервиса.

**Задачи**:
- Добавить таблицы `reviews` и обновить модель `couriers` (добавить поля `rating`, `total_reviews`)
- Создать API для оставления отзыва после доставки (`POST /api/orders/{id}/review`)
- Реализовать расчет среднего рейтинга курьера и обновление в реальном времени
- Добавить API для получения отзывов курьера (`GET /api/couriers/{id}/reviews`)
- Добавить фильтрацию курьеров по рейтингу в API получения доступных курьеров

**Критерии готовности**:
- Клиенты могут оставлять оценки (1-5 звезд) и текстовые отзывы
- Рейтинг курьера автоматически пересчитывается при новых отзывах
- API возвращает топ-курьеров по рейтингу для приоритетного назначения

### 2. Автоматическое назначение оптимального курьера
**Сложность**: Средняя  
**Время**: 7-8 часов

**Описание**: Реализовать умный алгоритм назначения курьеров на основе местоположения и рейтинга.

**Задачи**:
- Создать сервис `CourierAssignmentService` с алгоритмом выбора оптимального курьера
- Реализовать расчет расстояния от курьера до точки получения заказа
- Добавить весовые коэффициенты: расстояние (40%), рейтинг (30%), загруженность (30%)
- Добавить API автоназначения (`POST /api/orders/{id}/auto-assign`)
- Интегрировать автоназначение в процесс создания заказа как опцию

**Критерии готовности**:
- Система автоматически выбирает лучшего курьера на основе критериев
- Учитывается текущая загруженность курьеров
- Логирование причин выбора конкретного курьера для прозрачности

### 3. Расчет стоимости доставки на основе расстояния
**Сложность**: Легкая  
**Время**: 4-6 часов

**Описание**: Реализовать автоматический расчет стоимости доставки.

**Задачи**:
- Добавить поля `pickup_address`, `delivery_cost` в модель Order
- Создать сервис для геокодирования адресов (например, через Yandex Maps API)
- Реализовать алгоритм расчета стоимости на основе расстояния
- Добавить настройки тарифов в конфигурацию
- Обновить API создания заказа для автоматического расчета стоимости

**Критерии готовности**:
- Стоимость доставки рассчитывается автоматически при создании заказа
- Кеширование результатов геокодирования в Redis
- Возможность ручного override стоимости доставки

### 4. Система промокодов и скидок
**Сложность**: Средняя  
**Время**: 6-7 часов

**Описание**: Добавить маркетинговый инструмент для привлечения и удержания клиентов.

**Задачи**:
- Создать таблицу `promo_codes` с полями: код, тип скидки, размер, срок действия, лимит использований
- Добавить поле `promo_code` и `discount_amount` в модель Order  
- Реализовать API управления промокодами (`POST/GET/PUT/DELETE /api/promo-codes`)
- Добавить валидацию промокода при создании заказа
- Реализовать разные типы скидок: фиксированная сумма, процент, бесплатная доставка

**Критерии готовности**:
- Администраторы могут создавать и управлять промокодами
- Клиенты могут применять промокоды при оформлении заказа
- Система отслеживает использование промокодов и блокирует истекшие

### 5. Система отчетности и бизнес-аналитики
**Сложность**: Средняя  
**Время**: 7-8 часов

**Описание**: Реализовать дашборд с ключевыми бизнес-метриками для принятия решений.

**Задачи**:
- Создать API эндпоинты для бизнес-метрик (`GET /api/analytics/*`)
- Реализовать расчет KPI: выручка, количество заказов, среднее время доставки, популярные товары
- Добавить группировку метрик по периодам (день, неделя, месяц)
- Создать отчеты по курьерам: рейтинг, количество доставок, заработок
- Реализовать кеширование тяжелых аналитических запросов в Redis

**Критерии готовности**:
- API возвращает все ключевые бизнес-метрики с фильтрацией по датам
- Отчеты генерируются быстро благодаря кешированию
- Данные можно экспортировать в JSON/CSV формате для дальнейшего анализа



---

## Отчёт по реализации ТЗ (1–5)

Этот раздел — итог для ревьюера: что реализовано по пунктам 1–5, где это находится в коде и как быстро проверить.
Примечание: последующие блоки `Уже реализованно/Не реализовано/Статус реализации...` оставлены как часть исходного README и могут быть неактуальны — ориентируйтесь на этот отчёт.

### 1) Рейтинги и отзывы
- **БД/данные**: `reviews`, `couriers.rating/total_reviews`, `orders.rating/review_comment`, триггер пересчёта рейтинга (`migrations/002_reviews.up.sql`).
- **API**: `POST /api/orders/{id}/review`, `GET /api/couriers/{id}/reviews` (роуты в `cmd/server/main.go`).
- **Логика**: рейтинг 1–5, отзыв только после `delivered`, запрет повторного отзыва (`internal/services/order_service.go`).

### 2) Автоназначение курьера
- **API**: `POST /api/orders/{id}/auto-assign` + `auto_assign` в `POST /api/orders` (`internal/handlers/orders.go`).
- **Алгоритм**: scoring по расстоянию/рейтингу/нагрузке (веса `0.40/0.30/0.30`); расстояние считается от курьера до **точки получения (pickup)**; кандидаты — курьеры `available` и `busy`, курьер на пределе ёмкости (5 активных заказов) исключается (`internal/services/courier_assignment_service.go`, `internal/services/courier_service.go`).
- **Kafka**: при автоназначении и ручном назначении публикуются `courier.assigned` и `order.status_changed` (best effort) (`internal/handlers/orders.go`, `internal/handlers/couriers.go`).
- **Фильтр по рейтингу**: `GET /api/couriers/available?min_rating=4.5&order_by=rating` (`internal/handlers/couriers.go`).

### 3) Стоимость доставки и геокодинг
- **Расчёт**: distance (haversine) → `PricingService.CalculateCost` (base/per_km/min_fare) (`internal/services/pricing_service.go`, `internal/services/order_service.go`).
- **Геокодер**: `osm` (Nominatim, по умолчанию) или `yandex` (опционально, при ошибке fallback на OSM), кеш в Redis (`internal/services/geocoding_service.go`).
- **Контракт API**: `pickup_address` обязателен при создании заказа (валидация в `internal/handlers/orders.go`).
- **Override**: ручная стоимость доставки через `delivery_cost` в `POST /api/orders` (`internal/services/order_service.go`).

### 4) Промокоды и скидки
- **БД**: `promo_codes` + поля `orders.promo_code/discount_amount` (`migrations/004_promo_codes.up.sql`).
- **API**: `/api/promo-codes` (CRUD) (`internal/handlers/promo_codes.go`).
- **Логика**: применение скидки в транзакции (`SELECT ... FOR UPDATE` + `used_count++`) (`internal/services/promo_service.go`, `internal/services/order_service.go`).

### 5) Аналитика
- **API**: `/api/analytics/kpi`, `/api/analytics/couriers` (JSON/CSV через `format=csv`) (`internal/handlers/analytics.go`).
- **Кеш**: кеширование в Redis + инвалидация stats-cache при смене статуса заказа и при создании review (best effort) (`internal/services/analytics_service.go`, `internal/handlers/orders.go`).

## Как проверить (сценарий для ТЗ 1–5)

Ниже шаги идут в **логическом порядке проверки**, а в заголовках указано, к какому пункту ТЗ относится шаг.

### Шаг 1 — Поднять окружение (Kafka/Redis/PostgreSQL)
```bash
make up && make health

# Ожидаем 200 и status=healthy
curl -s -i http://localhost:8080/health
```

### Шаг 2 — ТЗ‑2: Подготовить курьера с локацией (нужно для автоназначения)
```bash
phone="+7999$(date +%H%M%S)"
courier_id=$(curl -s -X POST http://localhost:8080/api/couriers \
  -H "Content-Type: application/json" \
  -d '{"name":"Иван","phone":"'"$phone"'"}' | jq -r .id)

curl -s -X PUT "http://localhost:8080/api/couriers/${courier_id}/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"available","current_lat":55.7558,"current_lon":37.6173}' | jq
```

### Шаг 3 — ТЗ‑4: Создать промокод (опционально)
```bash
promo_code="SALE$(date +%H%M%S)"
curl -s -X POST http://localhost:8080/api/promo-codes \
  -H "Content-Type: application/json" \
  -d '{"code":"'"$promo_code"'","discount_type":"percent","amount":10,"max_uses":10,"active":true}' | jq
```

### Шаг 4 — ТЗ‑3 (+ТЗ‑2): Создать заказ (геокодинг + стоимость) и автоназначить курьера
```bash
resp=$(curl -s -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_name":"Анна",
    "customer_phone":"+79990000002",
    "pickup_address":"Москва, склад №1",
    "delivery_address":"Москва, ул. Ленина, 10",
    "items":[{"name":"Пицца","quantity":1,"price":500}],
    "promo_code":"'"$promo_code"'",
    "auto_assign":true
  }')

echo "$resp" | jq
order_id=$(echo "$resp" | jq -r '.order.id')
assigned_courier_id=$(echo "$resp" | jq -r '.assigned_courier.id')

# Если автоназначение не сработало (например, нет available курьеров), назначаем вручную через /auto-assign:
if [ -z "$assigned_courier_id" ] || [ "$assigned_courier_id" = "null" ]; then
  assigned_courier_id=$(curl -s -X POST "http://localhost:8080/api/orders/${order_id}/auto-assign" \
    -H "Content-Type: application/json" \
    -d '{}' | jq -r .id)
fi
```

### Шаг 5 — ТЗ‑1: Доставить заказ и оставить отзыв
Важно: при смене статуса передаём `courier_id`, чтобы не “стереть” назначение.
```bash
curl -s -X PUT "http://localhost:8080/api/orders/${order_id}/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"delivered","courier_id":"'"$assigned_courier_id"'"}' | jq

curl -s -X POST "http://localhost:8080/api/orders/${order_id}/review" \
  -H "Content-Type: application/json" \
  -d '{"rating":5,"comment":"быстро и аккуратно"}' | jq

curl -s "http://localhost:8080/api/couriers/${assigned_courier_id}/reviews?limit=10&offset=0" | jq
```

### Шаг 6 — ТЗ‑5: Проверить аналитику (JSON и CSV)
```bash
# Для "живых" цифр используем диапазон, включающий сегодняшнюю дату
to=$(date -u +%Y-%m-%d 2>/dev/null || date +%Y-%m-%d)
from=$(date -u -d '30 days ago' +%Y-%m-%d 2>/dev/null || date -u -v-30d +%Y-%m-%d)

curl -s "http://localhost:8080/api/analytics/kpi?from=${from}&to=${to}&group_by=day" | jq
curl -s "http://localhost:8080/api/analytics/kpi?from=${from}&to=${to}&group_by=day&format=csv" | head

curl -s "http://localhost:8080/api/analytics/couriers?from=${from}&to=${to}&format=csv" | head
```

### Kafka (опционально: посмотреть события в топиках)
```bash
docker exec -i kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic orders --from-beginning --max-messages 5
docker exec -i kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic couriers --from-beginning --max-messages 5
```

### Rate limiting (опционально)
По умолчанию выключен (см. `RATE_LIMIT_ENABLED` в конфиге). Статус:
```bash
curl -s http://localhost:8080/api/rate-limit/status | jq
```

## Код-стайл и качество
- **Структура**: `handlers → services → db/redis/kafka`, без web-фреймворков (stdlib `net/http`) (`cmd/server/main.go`).
- **Ошибки**: типизированные ошибки `internal/apperror` + единый маппинг ошибок сервисов в HTTP (убраны `strings.Contains(err.Error())`) (`internal/handlers/error_map.go`).
- **Контексты**: `context.Context` прокинут через handler → service; DB на `QueryContext/ExecContext/BeginTx` (ключевые сервисы в `internal/services/*`).

## Тесты и покрытие
- Покрытие: `82%+` (локально фиксировалось `82.5%`).
- Новые/ключевые тесты:
  - `internal/handlers/orders_handler_test.go` (создание заказа, review, auto-assign, валидации/ошибки)
  - `internal/handlers/couriers_handler_test.go` (UpdateCourierStatus, ручное назначение)
  - `internal/handlers/promo_codes_test.go` (CRUD + валидации промокодов)
  - `internal/handlers/analytics_test.go` (JSON/CSV, валидация дат/параметров, timeout)
  - `internal/services/*_test.go` (promo_service, courier_assignment_service, geocoding_service.FirstPos и др.)
- Проверка:
  - `go test ./...`
  - `go test ./... -coverprofile=/tmp/cover.out && go tool cover -func=/tmp/cover.out | tail -n 1`

## Быстрая проверка
1) `make up && make health` (ожидаем `status=healthy`).
2) `curl "http://localhost:8080/api/analytics/kpi?from=$(date +%Y-%m-01)&to=$(date +%Y-%m-%d)&group_by=day"` (ожидаем `200` + JSON).
3) CI/CD: GitHub Actions workflows в `.github/workflows/` (CI + integration + release).

## Troubleshooting (если что-то не поднялось)
- Проверить, что контейнеры запущены и порты проброшены: `docker compose ps`
- Посмотреть, что именно не здорово (в ответе `/health` есть статусы `database/redis/kafka`): `curl -i http://localhost:8080/health`
- Логи приложения: `docker compose logs --tail=200 delivery-app`
- Логи зависимостей:
  - Postgres: `docker compose logs --tail=200 postgres`
  - Kafka: `docker compose logs --tail=200 kafka`
  - Redis: `docker compose logs --tail=200 redis`
- Если миграции не применились/данные “битые” (редко): `docker compose down -v && docker compose up -d --build` (удалит volumes с данными)

