package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"delivery-system/internal/database"
	"delivery-system/internal/redis"

	"github.com/IBM/sarama"
)

// HealthHandler представляет обработчик для проверки здоровья системы
type HealthHandler struct {
	db           *database.DB
	redisClient  *redis.Client
	kafkaBrokers []string
}

// NewHealthHandler создает новый обработчик здоровья
func NewHealthHandler(db *database.DB, redisClient *redis.Client, kafkaBrokers []string) *HealthHandler {
	return &HealthHandler{
		db:           db,
		redisClient:  redisClient,
		kafkaBrokers: kafkaBrokers,
	}
}

// HealthResponse представляет ответ проверки здоровья
type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
	Version  string            `json:"version"`
	Uptime   string            `json:"uptime"`
}

var startTime = time.Now()

// Health проверяет состояние всех компонентов системы
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	services := make(map[string]string)
	overallStatus := "healthy"

	// Проверка базы данных
	if err := h.db.Health(); err != nil {
		services["database"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		services["database"] = "healthy"
	}

	// Проверка Redis
	if err := h.redisClient.Health(ctx); err != nil {
		services["redis"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		services["redis"] = "healthy"
	}

	// Проверка Kafka
	if err := checkKafkaHealth(h.kafkaBrokers); err != nil {
		services["kafka"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		services["kafka"] = "healthy"
	}

	response := HealthResponse{
		Status:   overallStatus,
		Services: services,
		Version:  "1.0.0",
		Uptime:   time.Since(startTime).String(),
	}

	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSONResponse(w, statusCode, response)
}

// Readiness проверяет готовность приложения к обработке запросов
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// Быстрая проверка основных компонентов
	if err := h.db.Health(); err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "Database not ready")
		return
	}

	if err := h.redisClient.Health(ctx); err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "Redis not ready")
		return
	}

	if err := checkKafkaHealth(h.kafkaBrokers); err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "Kafka not ready")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ready"})
}

// Liveness проверяет, что приложение живо
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{
		"status": "alive",
		"uptime": time.Since(startTime).String(),
	})
}

// checkKafkaHealth проверяет доступность Kafka брокеров
func checkKafkaHealth(brokers []string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("no brokers configured")
	}

	cfg := sarama.NewConfig()
	cfg.Net.DialTimeout = 3 * time.Second
	cfg.Net.ReadTimeout = 5 * time.Second
	cfg.Net.WriteTimeout = 5 * time.Second
	cfg.Metadata.Retry.Max = 1
	cfg.Metadata.Retry.Backoff = 500 * time.Millisecond

	client, err := sarama.NewClient(brokers, cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	return nil
}
