package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"delivery-system/internal/apperror"
	"delivery-system/internal/database"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/google/uuid"
)

// OrderService представляет сервис для работы с заказами
type OrderService struct {
	db      *database.DB
	log     *logger.Logger
	pricing *PricingService
	promo   *PromoService
}

// NewOrderService создает новый экземпляр сервиса заказов
func NewOrderService(db *database.DB, log *logger.Logger, pricing *PricingService, promo *PromoService) *OrderService {
	return &OrderService{
		db:      db,
		log:     log,
		pricing: pricing,
		promo:   promo,
	}
}

// CreateOrder создает новый заказ
func (s *OrderService) CreateOrder(ctx context.Context, req *models.CreateOrderRequest) (*models.Order, error) {
	// Проверяем, что координаты присутствуют (должны быть после валидации/геокодирования)
	if req.PickupLat == nil || req.PickupLon == nil || req.DeliveryLat == nil || req.DeliveryLon == nil {
		return nil, apperror.Validation("pickup and delivery coordinates are required for pricing", nil)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Расчет суммарной стоимости товаров
	var itemsTotal float64
	for _, item := range req.Items {
		itemsTotal += item.Price * float64(item.Quantity)
	}

	// Расчет стоимости доставки
	distanceKm := calculateDistance(*req.PickupLat, *req.PickupLon, *req.DeliveryLat, *req.DeliveryLon)
	deliveryCost := s.pricing.CalculateCost(distanceKm)

	// Применение промокода, если указан
	var discountAmount float64
	if req.PromoCode != nil && *req.PromoCode != "" {
		if s.promo == nil {
			return nil, apperror.Validation("promo codes are not supported", nil)
		}

		discountAmount, err = s.promo.ApplyPromoWithTx(ctx, tx, *req.PromoCode, itemsTotal, deliveryCost)
		if err != nil {
			return nil, err
		}
	}

	totalAmount := itemsTotal + deliveryCost - discountAmount
	if totalAmount < 0 {
		totalAmount = 0
	}

	// Создание заказа
	orderID := uuid.New()
	order := &models.Order{
		ID:              orderID,
		CustomerName:    req.CustomerName,
		CustomerPhone:   req.CustomerPhone,
		DeliveryAddress: req.DeliveryAddress,
		PickupAddress:   req.PickupAddress,
		PickupLat:       req.PickupLat,
		PickupLon:       req.PickupLon,
		DeliveryLat:     req.DeliveryLat,
		DeliveryLon:     req.DeliveryLon,
		TotalAmount:     totalAmount,
		DeliveryCost:    deliveryCost,
		DiscountAmount:  discountAmount,
		PromoCode:       req.PromoCode,
		Status:          models.OrderStatusCreated,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	query := `
		INSERT INTO orders (id, customer_name, customer_phone, delivery_address, pickup_address, pickup_lat, pickup_lon, delivery_lat, delivery_lon, total_amount, delivery_cost, discount_amount, promo_code, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err = tx.ExecContext(ctx, query, order.ID, order.CustomerName, order.CustomerPhone,
		order.DeliveryAddress, order.PickupAddress, order.PickupLat, order.PickupLon, order.DeliveryLat, order.DeliveryLon,
		order.TotalAmount, order.DeliveryCost, order.DiscountAmount, order.PromoCode, order.Status, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// Добавление товаров в заказ
	for _, item := range req.Items {
		itemID := uuid.New()
		itemQuery := `
			INSERT INTO order_items (id, order_id, name, quantity, price)
			VALUES ($1, $2, $3, $4, $5)
		`
		_, err = tx.ExecContext(ctx, itemQuery, itemID, orderID, item.Name, item.Quantity, item.Price)
		if err != nil {
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}

		order.Items = append(order.Items, models.OrderItem{
			ID:       itemID,
			OrderID:  orderID,
			Name:     item.Name,
			Quantity: item.Quantity,
			Price:    item.Price,
		})
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.WithFields(map[string]interface{}{
		"order_id":      order.ID,
		"customer_name": order.CustomerName,
		"total_amount":  order.TotalAmount,
	}).Info("Order created successfully")

	return order, nil
}

// GetOrder получает заказ по ID
func (s *OrderService) GetOrder(ctx context.Context, orderID uuid.UUID) (*models.Order, error) {
	order := &models.Order{}

	query := `
		SELECT id, customer_name, customer_phone, delivery_address, pickup_address, pickup_lat, pickup_lon, delivery_lat, delivery_lon, total_amount, delivery_cost, discount_amount, promo_code,
		       status, courier_id, rating, review_comment, created_at, updated_at, delivered_at
		FROM orders 
		WHERE id = $1
	`

	err := s.db.QueryRowContext(ctx, query, orderID).Scan(
		&order.ID, &order.CustomerName, &order.CustomerPhone, &order.DeliveryAddress, &order.PickupAddress,
		&order.PickupLat, &order.PickupLon, &order.DeliveryLat, &order.DeliveryLon, &order.TotalAmount, &order.DeliveryCost, &order.DiscountAmount, &order.PromoCode,
		&order.Status, &order.CourierID, &order.Rating, &order.ReviewComment,
		&order.CreatedAt, &order.UpdatedAt, &order.DeliveredAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperror.NotFound("order not found", err)
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Получение товаров заказа
	itemsQuery := `
		SELECT id, order_id, name, quantity, price
		FROM order_items
		WHERE order_id = $1
	`

	rows, err := s.db.QueryContext(ctx, itemsQuery, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item models.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.Name, &item.Quantity, &item.Price); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate order items: %w", err)
	}

	return order, nil
}

// CreateReview создает отзыв по заказу и обновляет рейтинг курьера
func (s *OrderService) CreateReview(ctx context.Context, orderID uuid.UUID, req *models.CreateReviewRequest) (*models.Review, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, apperror.Validation("rating must be between 1 and 5", nil)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Получаем заказ и привязанного курьера, проверяем статус и отсутствие отзыва
	var courierID uuid.UUID
	var status models.OrderStatus
	var existingRating sql.NullInt32

	query := `
		SELECT courier_id, status, rating
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`

	if err := tx.QueryRowContext(ctx, query, orderID).Scan(&courierID, &status, &existingRating); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperror.NotFound("order not found", err)
		}
		return nil, fmt.Errorf("failed to fetch order for review: %w", err)
	}

	if courierID == uuid.Nil {
		return nil, apperror.Conflict("order has no assigned courier", nil)
	}

	if status != models.OrderStatusDelivered {
		return nil, apperror.Conflict("order is not delivered yet", nil)
	}

	if existingRating.Valid {
		return nil, apperror.Conflict("review already exists for this order", nil)
	}

	reviewID := uuid.New()
	review := &models.Review{
		ID:        reviewID,
		OrderID:   orderID,
		CourierID: courierID,
		Rating:    req.Rating,
		Comment:   req.Comment,
		CreatedAt: time.Now(),
	}

	insertReviewQuery := `
		INSERT INTO reviews (id, order_id, courier_id, rating, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if _, err := tx.ExecContext(ctx, insertReviewQuery, review.ID, review.OrderID, review.CourierID, review.Rating, review.Comment, review.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to insert review: %w", err)
	}

	updateOrderQuery := `
		UPDATE orders
		SET rating = $1, review_comment = $2, updated_at = $3
		WHERE id = $4
	`
	if _, err := tx.ExecContext(ctx, updateOrderQuery, review.Rating, review.Comment, time.Now(), orderID); err != nil {
		return nil, fmt.Errorf("failed to update order with review: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit review transaction: %w", err)
	}

	s.log.WithFields(map[string]interface{}{
		"order_id":   orderID,
		"courier_id": courierID,
		"rating":     review.Rating,
	}).Info("Review created and courier rating update triggered")

	return review, nil
}

// UpdateOrderStatus обновляет статус заказа
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, req *models.UpdateOrderStatusRequest) error {
	if req == nil || req.Status == "" {
		return apperror.Validation("status is required", nil)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		currentStatus      models.OrderStatus
		currentCourierID   *uuid.UUID
		currentDeliveredAt sql.NullTime
	)

	selectQuery := `
		SELECT status, courier_id, delivered_at
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`
	if err := tx.QueryRowContext(ctx, selectQuery, orderID).Scan(&currentStatus, &currentCourierID, &currentDeliveredAt); err != nil {
		if err == sql.ErrNoRows {
			return apperror.NotFound("order not found", err)
		}
		return fmt.Errorf("failed to fetch order status: %w", err)
	}

	if !isValidOrderStatusTransition(currentStatus, req.Status) {
		return apperror.Conflict("invalid order status transition", nil)
	}

	newCourierID := currentCourierID
	if req.CourierID != nil {
		if *req.CourierID == uuid.Nil {
			return apperror.Validation("courier_id must be a valid UUID", nil)
		}
		newCourierID = req.CourierID
	}

	now := time.Now()
	var deliveredAt sql.NullTime
	if req.Status == models.OrderStatusDelivered {
		if currentStatus == models.OrderStatusDelivered && currentDeliveredAt.Valid {
			deliveredAt = currentDeliveredAt
		} else {
			deliveredAt = sql.NullTime{Time: now, Valid: true}
		}
	} else {
		deliveredAt = sql.NullTime{Valid: false}
	}

	updateQuery := `
		UPDATE orders
		SET status = $1, courier_id = $2, updated_at = $3, delivered_at = $4
		WHERE id = $5
	`
	result, err := tx.ExecContext(ctx, updateQuery, req.Status, newCourierID, now, deliveredAt, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return apperror.NotFound("order not found", nil)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit order status update: %w", err)
	}

	s.log.WithFields(map[string]interface{}{
		"order_id":   orderID,
		"new_status": req.Status,
		"courier_id": newCourierID,
	}).Info("Order status updated")

	return nil
}

// GetOrders получает список заказов с фильтрацией
func (s *OrderService) GetOrders(ctx context.Context, status *models.OrderStatus, courierID *uuid.UUID, limit, offset int) ([]*models.Order, error) {
	query := `
		SELECT id, customer_name, customer_phone, delivery_address, pickup_address, pickup_lat, pickup_lon, delivery_lat, delivery_lon, total_amount, delivery_cost, discount_amount, promo_code,
		       status, courier_id, rating, review_comment, created_at, updated_at, delivered_at
		FROM orders 
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	if courierID != nil {
		query += fmt.Sprintf(" AND courier_id = $%d", argIndex)
		args = append(args, *courierID)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
		argIndex++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	defer rows.Close()

	var orders []*models.Order
	for rows.Next() {
		order := &models.Order{}
		if err := rows.Scan(&order.ID, &order.CustomerName, &order.CustomerPhone,
			&order.DeliveryAddress, &order.PickupAddress, &order.PickupLat, &order.PickupLon, &order.DeliveryLat, &order.DeliveryLon,
			&order.TotalAmount, &order.DeliveryCost, &order.DiscountAmount, &order.PromoCode, &order.Status,
			&order.CourierID, &order.Rating, &order.ReviewComment,
			&order.CreatedAt, &order.UpdatedAt, &order.DeliveredAt); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate orders: %w", err)
	}

	return orders, nil
}

// GetCourierReviews возвращает отзывы по курьеру
func (s *OrderService) GetCourierReviews(ctx context.Context, courierID uuid.UUID, limit, offset int) ([]*models.Review, error) {
	query := `
		SELECT id, order_id, courier_id, rating, comment, created_at
		FROM reviews
		WHERE courier_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.QueryContext(ctx, query, courierID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get courier reviews: %w", err)
	}
	defer rows.Close()

	var reviews []*models.Review
	for rows.Next() {
		review := &models.Review{}
		if err := rows.Scan(&review.ID, &review.OrderID, &review.CourierID, &review.Rating, &review.Comment, &review.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan review: %w", err)
		}
		reviews = append(reviews, review)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate reviews: %w", err)
	}

	return reviews, nil
}

func isValidOrderStatusTransition(from, to models.OrderStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case models.OrderStatusCreated:
		return to == models.OrderStatusAccepted || to == models.OrderStatusCancelled
	case models.OrderStatusAccepted:
		return to == models.OrderStatusPreparing || to == models.OrderStatusCancelled
	case models.OrderStatusPreparing:
		return to == models.OrderStatusReady || to == models.OrderStatusCancelled
	case models.OrderStatusReady:
		return to == models.OrderStatusInDelivery || to == models.OrderStatusCancelled
	case models.OrderStatusInDelivery:
		return to == models.OrderStatusDelivered || to == models.OrderStatusCancelled
	case models.OrderStatusDelivered, models.OrderStatusCancelled:
		return false
	default:
		return false
	}
}
