package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestOrderService_CreateOrder_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	req := &models.CreateOrderRequest{
		CustomerName:    "Test Customer",
		CustomerPhone:   "+79991234567",
		DeliveryAddress: "Moscow, Street 1",
		PickupAddress:   "Moscow, Warehouse 1",
		PickupLat:       floatPtr(55.75),
		PickupLon:       floatPtr(37.61),
		DeliveryLat:     floatPtr(55.80),
		DeliveryLon:     floatPtr(37.70),
		Items: []models.CreateOrderItemRequest{
			{Name: "Item1", Quantity: 2, Price: 100.0},
			{Name: "Item2", Quantity: 1, Price: 50.0},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO orders").
		WithArgs(sqlmock.AnyArg(), req.CustomerName, req.CustomerPhone, req.DeliveryAddress, req.PickupAddress, req.PickupLat, req.PickupLon, req.DeliveryLat, req.DeliveryLon, sqlmock.AnyArg(), sqlmock.AnyArg(), 0.0, nil, models.OrderStatusCreated, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO order_items").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Item1", 2, 100.0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO order_items").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Item2", 1, 50.0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	order, err := service.CreateOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(order.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(order.Items))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func TestOrderService_GetOrder_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	courierID := uuid.New()

	mock.ExpectQuery("SELECT id, customer_name, customer_phone, delivery_address, pickup_address, pickup_lat, pickup_lon, delivery_lat, delivery_lon, total_amount, delivery_cost, discount_amount, promo_code").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name", "customer_phone", "delivery_address", "pickup_address", "pickup_lat", "pickup_lon", "delivery_lat", "delivery_lon", "total_amount", "delivery_cost", "discount_amount", "promo_code", "status", "courier_id", "rating", "review_comment", "created_at", "updated_at", "delivered_at"}).
			AddRow(orderID, "John", "+79991234567", "Moscow", "Warehouse", 55.75, 37.61, 55.80, 37.70, 500.0, 200.0, 20.0, "SALE10", models.OrderStatusDelivered, courierID, 5, "good", time.Now(), time.Now(), time.Now()))

	mock.ExpectQuery("SELECT id, order_id, name, quantity, price FROM order_items").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "name", "quantity", "price"}).
			AddRow(uuid.New(), orderID, "Pizza", 1, 500.0))

	order, err := service.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if order.ID != orderID {
		t.Fatalf("expected order ID %v, got %v", orderID, order.ID)
	}

	if len(order.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(order.Items))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_GetOrder_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()

	mock.ExpectQuery("SELECT id, customer_name, customer_phone, delivery_address, pickup_address, pickup_lat, pickup_lon, delivery_lat, delivery_lon, total_amount, delivery_cost, discount_amount, promo_code").
		WithArgs(orderID).
		WillReturnError(sql.ErrNoRows)

	_, err := service.GetOrder(context.Background(), orderID)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_UpdateOrderStatus_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	courierID := uuid.New()
	req := &models.UpdateOrderStatusRequest{
		Status:    models.OrderStatusInDelivery,
		CourierID: &courierID,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, courier_id, delivered_at FROM orders").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "courier_id", "delivered_at"}).
			AddRow(models.OrderStatusReady, nil, nil))

	mock.ExpectExec("UPDATE orders SET status").
		WithArgs(req.Status, req.CourierID, sqlmock.AnyArg(), sqlmock.AnyArg(), orderID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	err := service.UpdateOrderStatus(context.Background(), orderID, req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_UpdateOrderStatus_Delivered(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	courierID := uuid.New()
	req := &models.UpdateOrderStatusRequest{
		Status:    models.OrderStatusDelivered,
		CourierID: &courierID,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, courier_id, delivered_at FROM orders").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "courier_id", "delivered_at"}).
			AddRow(models.OrderStatusInDelivery, courierID, nil))

	mock.ExpectExec("UPDATE orders SET status").
		WithArgs(req.Status, req.CourierID, sqlmock.AnyArg(), sqlmock.AnyArg(), orderID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	err := service.UpdateOrderStatus(context.Background(), orderID, req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_UpdateOrderStatus_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	req := &models.UpdateOrderStatusRequest{
		Status: models.OrderStatusCancelled,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, courier_id, delivered_at FROM orders").
		WithArgs(orderID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	err := service.UpdateOrderStatus(context.Background(), orderID, req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_GetOrders_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	status := models.OrderStatusCreated
	courierID := uuid.New()
	limit, offset := 10, 0

	rows := sqlmock.NewRows([]string{"id", "customer_name", "customer_phone", "delivery_address", "pickup_address", "pickup_lat", "pickup_lon", "delivery_lat", "delivery_lon", "total_amount", "delivery_cost", "discount_amount", "promo_code", "status", "courier_id", "rating", "review_comment", "created_at", "updated_at", "delivered_at"}).
		AddRow(uuid.New(), "Alice", "+79001234567", "Moscow", "Warehouse", 55.75, 37.61, 55.80, 37.70, 300.0, 180.0, 0.0, nil, status, courierID, nil, nil, time.Now(), time.Now(), nil)

	mock.ExpectQuery("SELECT id, customer_name, customer_phone, delivery_address, pickup_address, pickup_lat, pickup_lon, delivery_lat, delivery_lon, total_amount, delivery_cost, discount_amount, promo_code").
		WithArgs(status, courierID, limit).
		WillReturnRows(rows)

	orders, err := service.GetOrders(context.Background(), &status, &courierID, limit, offset)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_GetOrders_NoFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	rows := sqlmock.NewRows([]string{"id", "customer_name", "customer_phone", "delivery_address", "pickup_address", "pickup_lat", "pickup_lon", "delivery_lat", "delivery_lon", "total_amount", "delivery_cost", "discount_amount", "promo_code", "status", "courier_id", "rating", "review_comment", "created_at", "updated_at", "delivered_at"}).
		AddRow(uuid.New(), "Bob", "+79009876543", "SPb", "WH", 55.75, 37.61, 55.80, 37.70, 200.0, 170.0, 0.0, nil, models.OrderStatusCreated, nil, nil, nil, time.Now(), time.Now(), nil)

	mock.ExpectQuery("SELECT id, customer_name, customer_phone, delivery_address, pickup_address, pickup_lat, pickup_lon, delivery_lat, delivery_lon, total_amount, delivery_cost, discount_amount, promo_code").
		WillReturnRows(rows)

	orders, err := service.GetOrders(context.Background(), nil, nil, 0, 0)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
