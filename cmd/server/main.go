package main

import (
	"context"
	"encoding/json"
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

// Фабричные функции для подключения внешних сервисов (подменяемые в тестах).
var (
	dbConnect        = database.Connect
	redisConnect     = redis.Connect
	newKafkaProducer = kafka.NewProducer
	newKafkaConsumer = kafka.NewConsumer
	kafkaHealthCheck = handlers.CheckKafkaHealth
	loadConfig       = config.Load
	newLogger        = logger.New
)

// application агрегирует собранные зависимости.
type application struct {
	cfg      *config.Config
	log      *logger.Logger
	db       *database.DB
	redis    *redis.Client
	producer *kafka.Producer
	consumer *kafka.Consumer
	mux      *http.ServeMux
	server   *http.Server
}

func main() {
	app, err := buildApplication()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build app: %v\n", err)
		os.Exit(1)
	}
	app.log.Info("Starting delivery system server...")

	go func() {
		app.log.WithField("address", app.server.Addr).Info("HTTP server starting")
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.log.WithError(err).Fatal("HTTP server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = app.consumer.Stop()
	if err := app.server.Shutdown(ctx); err != nil {
		app.log.WithError(err).Error("Server forced to shutdown")
	}
	_ = app.producer.Close()
	_ = app.redis.Close()
	_ = app.db.Close()
	app.log.Info("Server exited")
}

// buildApplication создает все зависимости (подменяемые в тестах).
func buildApplication() (*application, error) {
	cfg := loadConfig()
	log := newLogger(&cfg.Logger)

	db, err := dbConnect(&cfg.Database, log)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}

	redisClient, err := redisConnect(&cfg.Redis, log)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("redis connect: %w", err)
	}

	producer, err := newKafkaProducer(&cfg.Kafka, log)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		return nil, fmt.Errorf("kafka producer: %w", err)
	}

	consumer, err := newKafkaConsumer(&cfg.Kafka, log)
	if err != nil {
		_ = producer.Close()
		_ = redisClient.Close()
		_ = db.Close()
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}

	pricingService := services.NewPricingService(cfg.Pricing.BaseFare, cfg.Pricing.PerKm, cfg.Pricing.MinFare)
	promoService := services.NewPromoService(db, log)

	orderService := services.NewOrderService(db, log, pricingService, promoService)
	courierService := services.NewCourierService(db, log)
	assignmentService := services.NewCourierAssignmentService(db, courierService, orderService, log)
	geocodingService := services.NewGeocodingService(redisClient, log, &cfg.Geocoding)
	analyticsService := services.NewAnalyticsService(db, redisClient, log, &cfg.Analytics)
	rateLimiter := services.NewRateLimiter(redisClient, log, &cfg.RateLimit)

	orderHandler := handlers.NewOrderHandler(orderService, assignmentService, geocodingService, producer, redisClient, log)
	courierHandler := handlers.NewCourierHandler(courierService, orderService, producer, redisClient, log)
	promoHandler := handlers.NewPromoHandler(promoService, log)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService, log, &cfg.Analytics)
	healthHandler := handlers.NewHealthHandler(db, redisClient, cfg.Kafka.Brokers, kafkaHealthCheck)
	rateLimitHandler := handlers.NewRateLimitHandler(rateLimiter, log, &cfg.RateLimit)

	registerEventHandlers(consumer, log)
	if err := consumer.Start(); err != nil {
		_ = consumer.Stop()
		_ = producer.Close()
		_ = redisClient.Close()
		_ = db.Close()
		return nil, fmt.Errorf("kafka consumer start: %w", err)
	}

	mux := setupRoutes(orderHandler, courierHandler, healthHandler, promoHandler, analyticsHandler, rateLimitHandler, rateLimiter, log)
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	return &application{
		cfg:      cfg,
		log:      log,
		db:       db,
		redis:    redisClient,
		producer: producer,
		consumer: consumer,
		mux:      mux,
		server:   server,
	}, nil
}

// setupRoutes настраивает маршруты HTTP сервера
func setupRoutes(orderHandler *handlers.OrderHandler, courierHandler *handlers.CourierHandler, healthHandler *handlers.HealthHandler, promoHandler *handlers.PromoHandler, analyticsHandler *handlers.AnalyticsHandler, rateLimitHandler *handlers.RateLimitHandler, rateLimiter *services.RateLimiter, log *logger.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	applyAPI := func(h http.HandlerFunc) http.HandlerFunc {
		return corsMiddleware(handlers.RateLimitMiddleware(rateLimiter, log, h))
	}

	// Health check endpoints
	mux.HandleFunc("/health", corsMiddleware(healthHandler.Health))
	mux.HandleFunc("/health/readiness", corsMiddleware(healthHandler.Readiness))
	mux.HandleFunc("/health/liveness", corsMiddleware(healthHandler.Liveness))

	// Order endpoints
	mux.HandleFunc("/api/orders", applyAPI(handleOrdersRoute(orderHandler)))
	mux.HandleFunc("/api/orders/", applyAPI(handleOrderRoute(orderHandler)))

	// Courier endpoints
	mux.HandleFunc("/api/couriers", applyAPI(handleCouriersRoute(courierHandler)))
	mux.HandleFunc("/api/couriers/", applyAPI(handleCourierRoute(courierHandler)))
	mux.HandleFunc("/api/couriers/available", applyAPI(courierHandler.GetAvailableCouriers))

	// Promo codes endpoints
	mux.HandleFunc("/api/promo-codes", applyAPI(handlePromoCodesRoute(promoHandler)))
	mux.HandleFunc("/api/promo-codes/", applyAPI(handlePromoCodeRoute(promoHandler)))

	// Analytics endpoints
	mux.HandleFunc("/api/analytics/kpi", applyAPI(analyticsHandler.GetKPIs))
	mux.HandleFunc("/api/analytics/couriers", applyAPI(analyticsHandler.GetCourierAnalytics))

	// Rate limit status
	mux.HandleFunc("/api/rate-limit/status", applyAPI(rateLimitHandler.Status))

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
		} else if strings.HasSuffix(r.URL.Path, "/review") {
			// Создание отзыва по заказу
			if r.Method == http.MethodPost {
				handler.CreateReview(w, r)
			} else {
				writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			}
		} else if strings.HasSuffix(r.URL.Path, "/auto-assign") {
			// Автоназначение курьера на заказ
			if r.Method == http.MethodPost {
				handler.AutoAssignCourier(w, r)
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
		} else if strings.HasSuffix(r.URL.Path, "/reviews") {
			// Получение отзывов курьера
			if r.Method == http.MethodGet {
				handler.GetCourierReviews(w, r)
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

// handlePromoCodesRoute обрабатывает коллекцию промокодов
func handlePromoCodesRoute(handler *handlers.PromoHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListPromoCodes(w, r)
		case http.MethodPost:
			handler.CreatePromoCode(w, r)
		default:
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// handlePromoCodeRoute обрабатывает отдельный промокод
func handlePromoCodeRoute(handler *handlers.PromoHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler.GetPromoCode(w, r)
			return
		}
		if r.Method == http.MethodPut {
			handler.UpdatePromoCode(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			handler.DeletePromoCode(w, r)
			return
		}
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
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
	type errorResponse struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}
