package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"delivery-system/internal/database"
	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func expectOrderGet(mock sqlmock.Sqlmock, orderID uuid.UUID, status models.OrderStatus, courierID interface{}, now time.Time) {
	mock.ExpectQuery("SELECT id, customer_name").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "customer_name", "customer_phone", "delivery_address", "pickup_address", "pickup_lat", "pickup_lon", "delivery_lat", "delivery_lon",
			"total_amount", "delivery_cost", "discount_amount", "promo_code", "status", "courier_id", "rating", "review_comment", "created_at", "updated_at", "delivered_at",
		}).AddRow(orderID, "Name", "Phone", "Addr", "Pickup", 55.0, 37.0, 56.0, 38.0, 100.0, 10.0, 0.0, nil, status, courierID, nil, nil, now, now, nil))

	mock.ExpectQuery("SELECT id, order_id, name, quantity, price FROM order_items").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "name", "quantity", "price"}))
}

func TestCalculateDistance(t *testing.T) {
	// Москва: 55.7558, 37.6173
	// Санкт-Петербург: 59.9343, 30.3351
	// Примерное расстояние: ~634 км
	distance := calculateDistance(55.7558, 37.6173, 59.9343, 30.3351)

	// Проверяем, что расстояние примерно 634 км (с погрешностью ±10 км)
	if distance < 624 || distance > 644 {
		t.Fatalf("expected distance ~634 km, got %.2f km", distance)
	}
}

func TestCalculateDistance_SamePoint(t *testing.T) {
	distance := calculateDistance(55.7558, 37.6173, 55.7558, 37.6173)
	if distance != 0 {
		t.Fatalf("expected 0 km for same point, got %.2f km", distance)
	}
}

func TestCourierAssignmentService_CalculateCourierScore(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	log := newTestLogger()
	orderService := NewOrderService(db, log, newTestPricingService(), nil)
	courierService := NewCourierService(db, log)
	assignmentService := NewCourierAssignmentService(db, courierService, orderService, log)

	courierID := uuid.New()
	lat, lon := 55.7558, 37.6173
	courier := &models.Courier{
		ID:         courierID,
		Name:       "Test Courier",
		Status:     models.CourierStatusAvailable,
		CurrentLat: &lat,
		CurrentLon: &lon,
		Rating:     4.5,
	}

	// Mock для подсчёта активных заказов
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Целевая точка доставки (близко к курьеру: ~5 км)
	targetLat, targetLon := 55.8, 37.6

	weights := DefaultWeights()
	score := assignmentService.calculateCourierScore(ctx, courier, targetLat, targetLon, weights)

	if score.CourierID != courierID {
		t.Fatalf("expected courier ID %v, got %v", courierID, score.CourierID)
	}

	// Проверяем, что расстояние рассчитано
	if score.Distance <= 0 {
		t.Fatalf("expected positive distance, got %.2f", score.Distance)
	}

	// Проверяем рейтинг score (4.5/5 = 0.9)
	expectedRatingScore := 0.9
	if score.RatingScore != expectedRatingScore {
		t.Fatalf("expected rating score %.2f, got %.2f", expectedRatingScore, score.RatingScore)
	}

	// Проверяем workload score (1 активный заказ из макс 5: 1 - 1/5 = 0.8)
	expectedWorkloadScore := 0.8
	if score.WorkloadScore != expectedWorkloadScore {
		t.Fatalf("expected workload score %.2f, got %.2f", expectedWorkloadScore, score.WorkloadScore)
	}

	// TotalScore должен быть взвешенной суммой
	expectedTotal := (score.DistanceScore * weights.Distance) +
		(score.RatingScore * weights.Rating) +
		(score.WorkloadScore * weights.Workload)
	if score.TotalScore != expectedTotal {
		t.Fatalf("expected total score %.2f, got %.2f", expectedTotal, score.TotalScore)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()

	if weights.Distance != 0.40 {
		t.Fatalf("expected distance weight 0.40, got %.2f", weights.Distance)
	}
	if weights.Rating != 0.30 {
		t.Fatalf("expected rating weight 0.30, got %.2f", weights.Rating)
	}
	if weights.Workload != 0.30 {
		t.Fatalf("expected workload weight 0.30, got %.2f", weights.Workload)
	}

	// Проверяем, что сумма весов равна 1.0
	total := weights.Distance + weights.Rating + weights.Workload
	if total != 1.0 {
		t.Fatalf("expected total weight 1.0, got %.2f", total)
	}
}

type stubOrderSvcAssign struct {
	order *models.Order
	err   error
}

func (s *stubOrderSvcAssign) GetOrder(id uuid.UUID) (*models.Order, error) { return s.order, s.err }

type stubCourierSvcAssign struct {
	list       []*models.Courier
	assignedID uuid.UUID
	assignErr  error
}

func (s *stubCourierSvcAssign) GetAvailableCouriers() ([]*models.Courier, error) { return s.list, nil }
func (s *stubCourierSvcAssign) AssignOrderToCourier(orderID, courierID uuid.UUID) error {
	s.assignedID = courierID
	return s.assignErr
}

func TestCourierAssignmentService_AutoAssign(t *testing.T) {
	log := newTestLogger()
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	db := &database.DB{DB: sqlDB}

	orderID := uuid.New()
	courierID := uuid.New()
	now := time.Now()

	orderRows := sqlmock.NewRows([]string{
		"id", "customer_name", "customer_phone", "delivery_address", "pickup_address", "pickup_lat", "pickup_lon", "delivery_lat", "delivery_lon",
		"total_amount", "delivery_cost", "discount_amount", "promo_code", "status", "courier_id", "rating", "review_comment", "created_at", "updated_at", "delivered_at",
	}).AddRow(orderID, "Name", "Phone", "Addr", "Pickup", 55.0, 37.0, 56.0, 38.0, 100.0, 10.0, 0.0, nil, models.OrderStatusCreated, nil, nil, nil, now, now, nil)
	mock.ExpectQuery("SELECT id, customer_name").WithArgs(orderID).WillReturnRows(orderRows)
	mock.ExpectQuery("SELECT id, order_id, name, quantity, price FROM order_items").WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "name", "quantity", "price"}))

	courierRows := sqlmock.NewRows([]string{
		"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at",
	}).AddRow(courierID, "C", "p", models.CourierStatusAvailable, 55.0, 37.0, 4.5, 0, now, now, nil)
	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon").WillReturnRows(courierRows)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM orders").WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM couriers WHERE id = \\$1 FOR UPDATE").
		WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(string(models.CourierStatusAvailable)))
	mock.ExpectExec("UPDATE orders").
		WithArgs(courierID, models.OrderStatusAccepted, sqlmock.AnyArg(), orderID, models.OrderStatusCreated).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE couriers").
		WithArgs(models.CourierStatusBusy, sqlmock.AnyArg(), courierID, models.CourierStatusAvailable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon, rating, total_reviews, created_at, updated_at, last_seen_at FROM couriers").
		WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at"}).
			AddRow(courierID, "C", "p", models.CourierStatusAvailable, 55.0, 37.0, 4.5, 0, now, now, nil))

	orderSvc := NewOrderService(db, log, newTestPricingService(), nil)
	courierSvc := NewCourierService(db, log)
	service := NewCourierAssignmentService(db, courierSvc, orderSvc, log)

	courier, err := service.AutoAssignCourier(context.Background(), orderID, 56.0, 38.0)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if courier == nil || courier.ID != courierID {
		t.Fatalf("expected courier %v, got %+v", courierID, courier)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierAssignmentService_AutoAssign_OrderNotCreated(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	log := newTestLogger()
	orderSvc := NewOrderService(db, log, newTestPricingService(), nil)
	courierSvc := NewCourierService(db, log)
	service := NewCourierAssignmentService(db, courierSvc, orderSvc, log)

	orderID := uuid.New()
	now := time.Now()
	expectOrderGet(mock, orderID, models.OrderStatusDelivered, nil, now)

	if _, err := service.AutoAssignCourier(ctx, orderID, 56.0, 38.0); err == nil {
		t.Fatalf("expected error for non-created order status")
	}
}

func TestCourierAssignmentService_AutoAssign_OrderAlreadyAssigned(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	log := newTestLogger()
	orderSvc := NewOrderService(db, log, newTestPricingService(), nil)
	courierSvc := NewCourierService(db, log)
	service := NewCourierAssignmentService(db, courierSvc, orderSvc, log)

	orderID := uuid.New()
	now := time.Now()
	courierID := uuid.New()
	expectOrderGet(mock, orderID, models.OrderStatusCreated, courierID, now)

	if _, err := service.AutoAssignCourier(ctx, orderID, 56.0, 38.0); err == nil {
		t.Fatalf("expected error for already assigned order")
	}
}

func TestCourierAssignmentService_AutoAssign_NoAvailableCouriers(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	log := newTestLogger()
	orderSvc := NewOrderService(db, log, newTestPricingService(), nil)
	courierSvc := NewCourierService(db, log)
	service := NewCourierAssignmentService(db, courierSvc, orderSvc, log)

	orderID := uuid.New()
	now := time.Now()
	expectOrderGet(mock, orderID, models.OrderStatusCreated, nil, now)

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at",
		}))

	if _, err := service.AutoAssignCourier(ctx, orderID, 56.0, 38.0); err == nil {
		t.Fatalf("expected error for empty courier list")
	}
}

func TestCourierAssignmentService_AutoAssign_NoCouriersWithLocation(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	log := newTestLogger()
	orderSvc := NewOrderService(db, log, newTestPricingService(), nil)
	courierSvc := NewCourierService(db, log)
	service := NewCourierAssignmentService(db, courierSvc, orderSvc, log)

	orderID := uuid.New()
	now := time.Now()
	expectOrderGet(mock, orderID, models.OrderStatusCreated, nil, now)

	courierID := uuid.New()
	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at",
		}).AddRow(courierID, "C", "p", models.CourierStatusAvailable, nil, nil, 4.5, 0, now, now, nil))

	if _, err := service.AutoAssignCourier(ctx, orderID, 56.0, 38.0); err == nil {
		t.Fatalf("expected error for couriers without location")
	}
}

func TestCourierAssignmentService_getActiveCourierOrders_ErrorReturnsZero(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := &CourierAssignmentService{db: db, log: newTestLogger()}
	courierID := uuid.New()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(courierID).
		WillReturnError(errors.New("db error"))

	if v := svc.getActiveCourierOrders(ctx, courierID); v != 0 {
		t.Fatalf("expected 0 on error, got %d", v)
	}
}
