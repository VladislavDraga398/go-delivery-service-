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

// OrderHandler представляет обработчик заказов
type OrderHandler struct {
	orderService      *services.OrderService
	assignmentService *services.CourierAssignmentService
	geocodingService  *services.GeocodingService
	producer          *kafka.Producer
	redisClient       *redis.Client
	log               *logger.Logger
}

// NewOrderHandler создает новый обработчик заказов
func NewOrderHandler(orderService *services.OrderService, assignmentService *services.CourierAssignmentService, geocodingService *services.GeocodingService, producer *kafka.Producer, redisClient *redis.Client, log *logger.Logger) *OrderHandler {
	return &OrderHandler{
		orderService:      orderService,
		assignmentService: assignmentService,
		geocodingService:  geocodingService,
		producer:          producer,
		redisClient:       redisClient,
		log:               log,
	}
}

// CreateOrder создает новый заказ
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация запроса
	if err := h.validateCreateOrderRequest(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Если координаты не переданы, пытаемся геокодировать адреса
	if req.PickupLat == nil || req.PickupLon == nil {
		lat, lon, err := h.geocodingService.Geocode(r.Context(), req.PickupAddress)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "Failed to geocode pickup address")
			return
		}
		req.PickupLat = &lat
		req.PickupLon = &lon
	}

	if req.DeliveryLat == nil || req.DeliveryLon == nil {
		lat, lon, err := h.geocodingService.Geocode(r.Context(), req.DeliveryAddress)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "Failed to geocode delivery address")
			return
		}
		req.DeliveryLat = &lat
		req.DeliveryLon = &lon
	}

	// Создание заказа
	order, err := h.orderService.CreateOrder(&req)
	if err != nil {
		h.log.WithError(err).Error("Failed to create order")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to create order")
		return
	}

	// Публикация события в Kafka
	if err := h.producer.PublishOrderCreated(order); err != nil {
		h.log.WithError(err).Error("Failed to publish order created event")
		// Не возвращаем ошибку клиенту, так как заказ уже создан
	}

	// Кеширование заказа в Redis
	cacheKey := redis.GenerateKey(redis.KeyPrefixOrder, order.ID.String())
	if err := h.redisClient.Set(r.Context(), cacheKey, order, defaultCacheTTL); err != nil {
		h.log.WithError(err).Error("Failed to cache order")
		// Не возвращаем ошибку клиенту
	}

	// Опциональное автоназначение курьера сразу после создания заказа
	var assignedCourier *models.Courier
	if req.AutoAssign {
		courier, err := h.assignmentService.AutoAssignCourier(order.ID, *req.DeliveryLat, *req.DeliveryLon)
		if err != nil {
			h.log.WithError(err).WithField("order_id", order.ID).Warn("Auto-assign failed after order creation")
		} else {
			assignedCourier = courier

			// Обновляем заказ из БД, чтобы вернуть актуальный статус/курьера
			updatedOrder, getErr := h.orderService.GetOrder(order.ID)
			if getErr == nil {
				order = updatedOrder
			} else {
				h.log.WithError(getErr).WithField("order_id", order.ID).Warn("Failed to reload order after auto-assign")
			}

			// Инвалидация кешей
			h.redisClient.Delete(r.Context(), cacheKey)
			courierCacheKey := redis.GenerateKey(redis.KeyPrefixCourier, courier.ID.String())
			h.redisClient.Delete(r.Context(), courierCacheKey)
		}
	}

	h.log.WithField("order_id", order.ID).Info("Order created successfully")

	// Формируем ответ: сохраняем обратную совместимость (возвращаем заказ), но при автоназначении добавляем доп. поля
	if req.AutoAssign {
		response := map[string]interface{}{
			"order": order,
		}
		if assignedCourier != nil {
			response["assigned_courier"] = assignedCourier
		}
		writeJSONResponse(w, http.StatusCreated, response)
		return
	}

	writeJSONResponse(w, http.StatusCreated, order)
}

// GetOrder получает заказ по ID
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	orderID, err := extractUUIDFromPath(r.URL.Path, "/api/orders/")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	// Попытка получить из кеша
	cacheKey := redis.GenerateKey(redis.KeyPrefixOrder, orderID.String())
	var order models.Order
	if err := h.redisClient.Get(r.Context(), cacheKey, &order); err == nil {
		h.log.WithField("order_id", orderID).Debug("Order retrieved from cache")
		writeJSONResponse(w, http.StatusOK, &order)
		return
	}

	// Получение из базы данных
	orderPtr, err := h.orderService.GetOrder(orderID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Order not found")
		} else {
			h.log.WithError(err).Error("Failed to get order")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get order")
		}
		return
	}

	// Кеширование заказа
	if err := h.redisClient.Set(r.Context(), cacheKey, orderPtr, defaultCacheTTL); err != nil {
		h.log.WithError(err).Error("Failed to cache order")
	}

	writeJSONResponse(w, http.StatusOK, orderPtr)
}

// UpdateOrderStatus обновляет статус заказа
func (h *OrderHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	orderID, err := extractUUIDFromPath(r.URL.Path, "/api/orders/")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var req models.UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Получение текущего заказа для определения старого статуса
	currentOrder, err := h.orderService.GetOrder(orderID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Order not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get order")
		}
		return
	}

	oldStatus := currentOrder.Status

	// Обновление статуса
	if err := h.orderService.UpdateOrderStatus(orderID, &req); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Order not found")
		} else {
			h.log.WithError(err).Error("Failed to update order status")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to update order status")
		}
		return
	}

	// Публикация события изменения статуса
	if err := h.producer.PublishOrderStatusChanged(orderID, oldStatus, req.Status, req.CourierID); err != nil {
		h.log.WithError(err).Error("Failed to publish order status changed event")
	}

	// Инвалидация кеша
	cacheKey := redis.GenerateKey(redis.KeyPrefixOrder, orderID.String())
	if err := h.redisClient.Delete(r.Context(), cacheKey); err != nil {
		h.log.WithError(err).Error("Failed to invalidate order cache")
	}

	h.log.WithField("order_id", orderID).WithField("new_status", req.Status).Info("Order status updated")
	writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Order status updated successfully"})
}

// GetOrders получает список заказов с фильтрацией
func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := r.URL.Query()

	// Парсинг параметров фильтрации
	var status *models.OrderStatus
	if statusStr := query.Get("status"); statusStr != "" {
		s := models.OrderStatus(statusStr)
		status = &s
	}

	var courierID *uuid.UUID
	if courierIDStr := query.Get("courier_id"); courierIDStr != "" {
		id, err := uuid.Parse(courierIDStr)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "Invalid courier ID")
			return
		}
		courierID = &id
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

	orders, err := h.orderService.GetOrders(status, courierID, limit, offset)
	if err != nil {
		h.log.WithError(err).Error("Failed to get orders")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get orders")
		return
	}

	writeJSONResponse(w, http.StatusOK, orders)
}

// CreateReview создает отзыв по доставленному заказу
func (h *OrderHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	orderID, err := extractUUIDFromPath(r.URL.Path, "/api/orders/")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var req models.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	review, err := h.orderService.CreateReview(orderID, &req)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not found"):
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		case strings.Contains(err.Error(), "not delivered") || strings.Contains(err.Error(), "no assigned") || strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "between 1 and 5"):
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		default:
			h.log.WithError(err).Error("Failed to create review")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to create review")
		}
		return
	}

	// Инвалидация кеша заказа
	cacheKey := redis.GenerateKey(redis.KeyPrefixOrder, orderID.String())
	h.redisClient.Delete(r.Context(), cacheKey)

	// Инвалидация кеша курьера
	courierCacheKey := redis.GenerateKey(redis.KeyPrefixCourier, review.CourierID.String())
	h.redisClient.Delete(r.Context(), courierCacheKey)

	writeJSONResponse(w, http.StatusCreated, review)
}

// validateCreateOrderRequest валидирует запрос на создание заказа
func (h *OrderHandler) validateCreateOrderRequest(req *models.CreateOrderRequest) error {
	if req.CustomerName == "" {
		return fmt.Errorf("customer name is required")
	}
	if req.CustomerPhone == "" {
		return fmt.Errorf("customer phone is required")
	}
	if req.DeliveryAddress == "" {
		return fmt.Errorf("delivery address is required")
	}
	if req.PickupAddress == "" {
		return fmt.Errorf("pickup address is required")
	}
	if len(req.Items) == 0 {
		return fmt.Errorf("order items are required")
	}

	for i, item := range req.Items {
		if item.Name == "" {
			return fmt.Errorf("item %d: name is required", i+1)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("item %d: quantity must be positive", i+1)
		}
		if item.Price < 0 {
			return fmt.Errorf("item %d: price cannot be negative", i+1)
		}
	}

	// Координаты: если указаны, валидируем; если нет — будут геокодированы позже
	if req.PickupLat != nil {
		if *req.PickupLat < -90 || *req.PickupLat > 90 {
			return fmt.Errorf("pickup_lat must be between -90 and 90")
		}
	}
	if req.PickupLon != nil {
		if *req.PickupLon < -180 || *req.PickupLon > 180 {
			return fmt.Errorf("pickup_lon must be between -180 and 180")
		}
	}
	if req.DeliveryLat != nil {
		if *req.DeliveryLat < -90 || *req.DeliveryLat > 90 {
			return fmt.Errorf("delivery_lat must be between -90 and 90")
		}
	}
	if req.DeliveryLon != nil {
		if *req.DeliveryLon < -180 || *req.DeliveryLon > 180 {
			return fmt.Errorf("delivery_lon must be between -180 and 180")
		}
	}

	// Проверка координат для автоназначения
	if req.AutoAssign {
		// координаты уже проверены выше
	}

	return nil
}

// AutoAssignCourierRequest представляет запрос на автоназначение курьера
type AutoAssignCourierRequest struct {
	DeliveryLat float64 `json:"delivery_lat"`
	DeliveryLon float64 `json:"delivery_lon"`
}

// AutoAssignCourier автоматически назначает оптимального курьера на заказ
func (h *OrderHandler) AutoAssignCourier(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	orderID, err := extractUUIDFromPath(r.URL.Path, "/api/orders/")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var req AutoAssignCourierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация координат
	if req.DeliveryLat < -90 || req.DeliveryLat > 90 {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid delivery latitude")
		return
	}
	if req.DeliveryLon < -180 || req.DeliveryLon > 180 {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid delivery longitude")
		return
	}

	// Автоматическое назначение курьера
	courier, err := h.assignmentService.AutoAssignCourier(orderID, req.DeliveryLat, req.DeliveryLon)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not found"):
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		case strings.Contains(err.Error(), "not in 'created' status") || strings.Contains(err.Error(), "already has assigned") || strings.Contains(err.Error(), "no available couriers") || strings.Contains(err.Error(), "no couriers with known location"):
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		default:
			h.log.WithError(err).Error("Failed to auto-assign courier")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to auto-assign courier")
		}
		return
	}

	// Инвалидация кеша заказа
	cacheKey := redis.GenerateKey(redis.KeyPrefixOrder, orderID.String())
	h.redisClient.Delete(r.Context(), cacheKey)

	// Инвалидация кеша курьера
	courierCacheKey := redis.GenerateKey(redis.KeyPrefixCourier, courier.ID.String())
	h.redisClient.Delete(r.Context(), courierCacheKey)

	h.log.WithField("order_id", orderID).WithField("courier_id", courier.ID).Info("Courier auto-assigned successfully")
	writeJSONResponse(w, http.StatusOK, courier)
}
