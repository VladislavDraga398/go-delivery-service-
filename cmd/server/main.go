package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/database"
	"delivery-system/internal/handlers"
	"delivery-system/internal/kafka"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"
	"delivery-system/internal/redis"
	"delivery-system/internal/services"
)

func main() {
	// Загрузка конфигурации
	cfg := config.Load()

	// Инициализация логгера
	log := logger.New(&cfg.Logger)
	log.Info("Starting delivery system server...")

	// Подключение к базе данных
	db, err := database.Connect(&cfg.Database, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Подключение к Redis
	redisClient, err := redis.Connect(&cfg.Redis, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to Redis")
	}
	defer redisClient.Close()

	// Создание Kafka producer
	producer, err := kafka.NewProducer(&cfg.Kafka, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kafka producer")
	}
	defer producer.Close()

	// Создание Kafka consumer
	consumer, err := kafka.NewConsumer(&cfg.Kafka, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kafka consumer")
	}
	defer consumer.Stop()

	// Инициализация сервисов
	orderService := services.NewOrderService(db, log)
	courierService := services.NewCourierService(db, log)

	// Инициализация handlers
	orderHandler := handlers.NewOrderHandler(orderService, producer, redisClient, log)
	courierHandler := handlers.NewCourierHandler(courierService, producer, redisClient, log)
	healthHandler := handlers.NewHealthHandler(db, redisClient)

	// Регистрация обработчиков событий Kafka
	registerEventHandlers(consumer, log)

	// Запуск Kafka consumer
	if err := consumer.Start(); err != nil {
		log.WithError(err).Fatal("Failed to start Kafka consumer")
	}

	// Настройка HTTP роутера
	mux := setupRoutes(orderHandler, courierHandler, healthHandler)

	// Создание HTTP сервера
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Запуск сервера в горутине
	go func() {
		log.WithField("address", server.Addr).Info("HTTP server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Server forced to shutdown")
	}

	log.Info("Server exited")
}

// setupRoutes настраивает маршруты HTTP сервера
func setupRoutes(orderHandler *handlers.OrderHandler, courierHandler *handlers.CourierHandler, healthHandler *handlers.HealthHandler) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", corsMiddleware(healthHandler.Health))
	mux.HandleFunc("/health/readiness", corsMiddleware(healthHandler.Readiness))
	mux.HandleFunc("/health/liveness", corsMiddleware(healthHandler.Liveness))

	// Order endpoints
	mux.HandleFunc("/api/orders", corsMiddleware(handleOrdersRoute(orderHandler)))
	mux.HandleFunc("/api/orders/", corsMiddleware(handleOrderRoute(orderHandler)))

	// Courier endpoints
	mux.HandleFunc("/api/couriers", corsMiddleware(handleCouriersRoute(courierHandler)))
	mux.HandleFunc("/api/couriers/", corsMiddleware(handleCourierRoute(courierHandler)))
	mux.HandleFunc("/api/couriers/available", corsMiddleware(courierHandler.GetAvailableCouriers))

	return mux
}

// handleOrdersRoute обрабатывает маршруты для коллекции заказов
func handleOrdersRoute(handler *handlers.OrderHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetOrders(w, r)
		case http.MethodPost:
			handler.CreateOrder(w, r)
		default:
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// handleOrderRoute обрабатывает маршруты для отдельного заказа
func handleOrderRoute(handler *handlers.OrderHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			// Обновление статуса заказа
			if r.Method == http.MethodPut {
				handler.UpdateOrderStatus(w, r)
			} else {
				writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			}
		} else {
			// Получение заказа по ID
			if r.Method == http.MethodGet {
				handler.GetOrder(w, r)
			} else {
				writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			}
		}
	}
}

// handleCouriersRoute обрабатывает маршруты для коллекции курьеров
func handleCouriersRoute(handler *handlers.CourierHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetCouriers(w, r)
		case http.MethodPost:
			handler.CreateCourier(w, r)
		default:
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// handleCourierRoute обрабатывает маршруты для отдельного курьера
func handleCourierRoute(handler *handlers.CourierHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			// Обновление статуса курьера
			if r.Method == http.MethodPut {
				handler.UpdateCourierStatus(w, r)
			} else {
				writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			}
		} else if strings.HasSuffix(r.URL.Path, "/assign") {
			// Назначение заказа курьеру
			if r.Method == http.MethodPost {
				handler.AssignOrderToCourier(w, r)
			} else {
				writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			}
		} else {
			// Получение курьера по ID
			if r.Method == http.MethodGet {
				handler.GetCourier(w, r)
			} else {
				writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			}
		}
	}
}

// registerEventHandlers регистрирует обработчики событий Kafka
func registerEventHandlers(consumer *kafka.Consumer, log *logger.Logger) {
	// Пример обработчика событий - можно расширить по необходимости
	consumer.RegisterHandler("order.created", func(ctx context.Context, event *models.Event) error {
		log.WithField("event_id", event.ID).Info("Processing order created event")
		// Здесь можно добавить дополнительную логику обработки
		return nil
	})

	consumer.RegisterHandler("order.status_changed", func(ctx context.Context, event *models.Event) error {
		log.WithField("event_id", event.ID).Info("Processing order status changed event")
		// Здесь можно добавить логику уведомлений, обновления статистики и т.д.
		return nil
	})
}

// corsMiddleware и другие helper функции
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, `{"error": "%s", "message": "%s"}`, http.StatusText(statusCode), message)
}
