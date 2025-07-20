package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"delivery-system/internal/kafka"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"
	"delivery-system/internal/redis"
	"delivery-system/internal/services"

	"github.com/google/uuid"
)

// CourierHandler представляет обработчик курьеров
type CourierHandler struct {
	courierService *services.CourierService
	producer       *kafka.Producer
	redisClient    *redis.Client
	log            *logger.Logger
}

// NewCourierHandler создает новый обработчик курьеров
func NewCourierHandler(courierService *services.CourierService, producer *kafka.Producer, redisClient *redis.Client, log *logger.Logger) *CourierHandler {
	return &CourierHandler{
		courierService: courierService,
		producer:       producer,
		redisClient:    redisClient,
		log:            log,
	}
}

// CreateCourier создает нового курьера
func (h *CourierHandler) CreateCourier(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.CreateCourierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация запроса
	if err := h.validateCreateCourierRequest(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Создание курьера
	courier, err := h.courierService.CreateCourier(&req)
	if err != nil {
		h.log.WithError(err).Error("Failed to create courier")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to create courier")
		return
	}

	// Кеширование курьера в Redis
	cacheKey := redis.GenerateKey(redis.KeyPrefixCourier, courier.ID.String())
	if err := h.redisClient.Set(r.Context(), cacheKey, courier, defaultCacheTTL); err != nil {
		h.log.WithError(err).Error("Failed to cache courier")
	}

	h.log.WithField("courier_id", courier.ID).Info("Courier created successfully")
	writeJSONResponse(w, http.StatusCreated, courier)
}

// GetCourier получает курьера по ID
func (h *CourierHandler) GetCourier(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	courierID, err := extractUUIDFromPath(r.URL.Path, "/api/couriers/")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid courier ID")
		return
	}

	// Попытка получить из кеша
	cacheKey := redis.GenerateKey(redis.KeyPrefixCourier, courierID.String())
	var courier models.Courier
	if err := h.redisClient.Get(r.Context(), cacheKey, &courier); err == nil {
		h.log.WithField("courier_id", courierID).Debug("Courier retrieved from cache")
		writeJSONResponse(w, http.StatusOK, &courier)
		return
	}

	// Получение из базы данных
	courierPtr, err := h.courierService.GetCourier(courierID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Courier not found")
		} else {
			h.log.WithError(err).Error("Failed to get courier")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get courier")
		}
		return
	}

	// Кеширование курьера
	if err := h.redisClient.Set(r.Context(), cacheKey, courierPtr, defaultCacheTTL); err != nil {
		h.log.WithError(err).Error("Failed to cache courier")
	}

	writeJSONResponse(w, http.StatusOK, courierPtr)
}

// UpdateCourierStatus обновляет статус курьера
func (h *CourierHandler) UpdateCourierStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	courierID, err := extractUUIDFromPath(r.URL.Path, "/api/couriers/")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid courier ID")
		return
	}

	var req models.UpdateCourierStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Получение текущего курьера для определения старого статуса
	currentCourier, err := h.courierService.GetCourier(courierID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Courier not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get courier")
		}
		return
	}

	oldStatus := currentCourier.Status

	// Обновление статуса
	if err := h.courierService.UpdateCourierStatus(courierID, &req); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Courier not found")
		} else {
			h.log.WithError(err).Error("Failed to update courier status")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to update courier status")
		}
		return
	}

	// Публикация события изменения статуса курьера
	if err := h.producer.PublishCourierStatusChanged(courierID, oldStatus, req.Status); err != nil {
		h.log.WithError(err).Error("Failed to publish courier status changed event")
	}

	// Публикация события обновления местоположения (если предоставлены координаты)
	if req.CurrentLat != nil && req.CurrentLon != nil {
		if err := h.producer.PublishLocationUpdated(courierID, *req.CurrentLat, *req.CurrentLon); err != nil {
			h.log.WithError(err).Error("Failed to publish location updated event")
		}
	}

	// Инвалидация кеша
	cacheKey := redis.GenerateKey(redis.KeyPrefixCourier, courierID.String())
	if err := h.redisClient.Delete(r.Context(), cacheKey); err != nil {
		h.log.WithError(err).Error("Failed to invalidate courier cache")
	}

	h.log.WithField("courier_id", courierID).WithField("new_status", req.Status).Info("Courier status updated")
	writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Courier status updated successfully"})
}

// GetCouriers получает список курьеров с фильтрацией
func (h *CourierHandler) GetCouriers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := r.URL.Query()

	// Парсинг параметров фильтрации
	var status *models.CourierStatus
	if statusStr := query.Get("status"); statusStr != "" {
		s := models.CourierStatus(statusStr)
		status = &s
	}

	limit := 50 // По умолчанию
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	couriers, err := h.courierService.GetCouriers(status, limit, offset)
	if err != nil {
		h.log.WithError(err).Error("Failed to get couriers")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get couriers")
		return
	}

	writeJSONResponse(w, http.StatusOK, couriers)
}

// GetAvailableCouriers получает список доступных курьеров
func (h *CourierHandler) GetAvailableCouriers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	couriers, err := h.courierService.GetAvailableCouriers()
	if err != nil {
		h.log.WithError(err).Error("Failed to get available couriers")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get available couriers")
		return
	}

	writeJSONResponse(w, http.StatusOK, couriers)
}

// AssignOrderToCourier назначает заказ курьеру
func (h *CourierHandler) AssignOrderToCourier(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	courierID, err := extractUUIDFromPath(r.URL.Path, "/api/couriers/")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid courier ID")
		return
	}

	var req struct {
		OrderID uuid.UUID `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.OrderID == uuid.Nil {
		writeErrorResponse(w, http.StatusBadRequest, "Order ID is required")
		return
	}

	// Назначение заказа курьеру
	if err := h.courierService.AssignOrderToCourier(req.OrderID, courierID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		} else if strings.Contains(err.Error(), "not available") {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		} else {
			h.log.WithError(err).Error("Failed to assign order to courier")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to assign order to courier")
		}
		return
	}

	// Публикация события назначения курьера
	if err := h.producer.PublishCourierAssigned(req.OrderID, courierID); err != nil {
		h.log.WithError(err).Error("Failed to publish courier assigned event")
	}

	// Инвалидация кеша курьера и заказа
	courierCacheKey := redis.GenerateKey(redis.KeyPrefixCourier, courierID.String())
	orderCacheKey := redis.GenerateKey(redis.KeyPrefixOrder, req.OrderID.String())

	h.redisClient.Delete(r.Context(), courierCacheKey)
	h.redisClient.Delete(r.Context(), orderCacheKey)

	h.log.WithField("order_id", req.OrderID).WithField("courier_id", courierID).Info("Order assigned to courier")
	writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Order assigned to courier successfully"})
}

// validateCreateCourierRequest валидирует запрос на создание курьера
func (h *CourierHandler) validateCreateCourierRequest(req *models.CreateCourierRequest) error {
	if req.Name == "" {
		return fmt.Errorf("courier name is required")
	}
	if req.Phone == "" {
		return fmt.Errorf("courier phone is required")
	}
	return nil
}
