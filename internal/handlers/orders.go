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
	orderService *services.OrderService
	producer     *kafka.Producer
	redisClient  *redis.Client
	log          *logger.Logger
}

// NewOrderHandler создает новый обработчик заказов
func NewOrderHandler(orderService *services.OrderService, producer *kafka.Producer, redisClient *redis.Client, log *logger.Logger) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		producer:     producer,
		redisClient:  redisClient,
		log:          log,
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

	h.log.WithField("order_id", order.ID).Info("Order created successfully")
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

	return nil
}
