package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// --- override стоимости доставки ---

func TestOrderService_CreateOrder_DeliveryCostOverride(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	cost := 777.55
	req := &models.CreateOrderRequest{
		CustomerName:    "Test",
		CustomerPhone:   "+79990000000",
		DeliveryAddress: "delivery",
		PickupAddress:   "pickup",
		Items:           []models.CreateOrderItemRequest{{Name: "Item", Quantity: 1, Price: 100}},
		DeliveryCost:    &cost, // override: координаты не обязательны
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO orders").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO order_items").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	order, err := service.CreateOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if order.DeliveryCost != 777.55 {
		t.Fatalf("expected delivery_cost override 777.55, got %.2f", order.DeliveryCost)
	}
	// total = items(100) + delivery(777.55) - discount(0)
	if order.TotalAmount != 877.55 {
		t.Fatalf("expected total 877.55, got %.2f", order.TotalAmount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_CreateOrder_DeliveryCostOverride_Negative(t *testing.T) {
	db, _ := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	cost := -1.0
	req := &models.CreateOrderRequest{
		CustomerName:    "Test",
		CustomerPhone:   "+79990000000",
		DeliveryAddress: "delivery",
		PickupAddress:   "pickup",
		Items:           []models.CreateOrderItemRequest{{Name: "Item", Quantity: 1, Price: 100}},
		DeliveryCost:    &cost,
	}

	if _, err := service.CreateOrder(context.Background(), req); err == nil {
		t.Fatalf("expected validation error for negative delivery_cost")
	}
}

func TestOrderService_CreateOrder_NoCoordsNoOverride_ValidationError(t *testing.T) {
	db, _ := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	req := &models.CreateOrderRequest{
		CustomerName:    "Test",
		CustomerPhone:   "+79990000000",
		DeliveryAddress: "delivery",
		PickupAddress:   "pickup",
		Items:           []models.CreateOrderItemRequest{{Name: "Item", Quantity: 1, Price: 100}},
	}

	if _, err := service.CreateOrder(context.Background(), req); err == nil {
		t.Fatalf("expected validation error when no coords and no override")
	}
}

// --- OSM-геокодер ---

func newNominatimStub(t *testing.T, body string) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestGeocodingService_OSMGeocode_Success(t *testing.T) {
	rdb := newTestRedis(t)
	log := newTestLogger()
	service := NewGeocodingService(rdb, log, &config.GeocodingConfig{
		Provider:   "osm",
		OSMBaseURL: newNominatimStub(t, `[{"lat":"55.7558","lon":"37.6173"}]`),
	})

	lat, lon, err := service.Geocode(context.Background(), "Москва, Красная площадь")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if lat != 55.7558 || lon != 37.6173 {
		t.Fatalf("unexpected coords: lat=%f lon=%f", lat, lon)
	}
}

func TestGeocodingService_OSMGeocode_EmptyResults(t *testing.T) {
	rdb := newTestRedis(t)
	log := newTestLogger()
	service := NewGeocodingService(rdb, log, &config.GeocodingConfig{
		Provider:   "osm",
		OSMBaseURL: newNominatimStub(t, `[]`),
	})

	if _, _, err := service.Geocode(context.Background(), "nowhere"); err == nil {
		t.Fatalf("expected error for empty results")
	}
}

func TestGeocodingService_YandexWithoutKey_FallsBackToOSM(t *testing.T) {
	rdb := newTestRedis(t)
	log := newTestLogger()
	service := NewGeocodingService(rdb, log, &config.GeocodingConfig{
		Provider:   "yandex",
		OSMBaseURL: newNominatimStub(t, `[{"lat":"10.5","lon":"20.5"}]`),
	})

	lat, lon, err := service.Geocode(context.Background(), "addr")
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if lat != 10.5 || lon != 20.5 {
		t.Fatalf("unexpected coords: lat=%f lon=%f", lat, lon)
	}
}

// --- автоназначение: занятые курьеры и ёмкость ---

func sqlmockRowsCouriers(id uuid.UUID, status models.CourierStatus, lat, lon float64, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at",
	}).AddRow(id, "C", "p", status, lat, lon, 4.5, 0, now, now, nil)
}

func sqlmockRowsCount(n int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"count"}).AddRow(n)
}

func sqlmockRowsCourierStatus(status models.CourierStatus) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"status"}).AddRow(string(status))
}

func TestCourierAssignmentService_BusyCourierBecomesCandidate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	orderSvc := NewOrderService(db, log, newTestPricingService(), nil)
	courierSvc := NewCourierService(db, log)
	service := NewCourierAssignmentService(db, courierSvc, orderSvc, log)

	courierID := uuid.New()
	now := time.Now()
	orderID := uuid.New()

	expectOrderGet(mock, orderID, models.OrderStatusCreated, nil, now)

	// Кандидат — занятый курьер с координатами
	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon").
		WillReturnRows(sqlmockRowsCouriers(courierID, models.CourierStatusBusy, 55.0, 37.0, now))

	// 1 активный заказ
	mock.ExpectQuery("SELECT COUNT").WithArgs(courierID).
		WillReturnRows(sqlmockRowsCount(1))

	// Назначение: транзакция
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM couriers WHERE id =").
		WithArgs(courierID).
		WillReturnRows(sqlmockRowsCourierStatus(models.CourierStatusAvailable))
	mock.ExpectExec("UPDATE orders").
		WithArgs(courierID, models.OrderStatusAccepted, sqlmock.AnyArg(), orderID, models.OrderStatusCreated).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE couriers").
		WithArgs(models.CourierStatusBusy, sqlmock.AnyArg(), courierID, models.CourierStatusAvailable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// Повторное чтение курьера для ответа
	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon, rating, total_reviews, created_at, updated_at, last_seen_at FROM couriers").
		WithArgs(courierID).
		WillReturnRows(sqlmockRowsCouriers(courierID, models.CourierStatusBusy, 55.0, 37.0, now))

	courier, err := service.AutoAssignCourier(context.Background(), orderID, 55.1, 37.1)
	if err != nil {
		t.Fatalf("expected busy courier to be assignable, got error: %v", err)
	}
	if courier == nil || courier.ID != courierID {
		t.Fatalf("expected courier %v, got %+v", courierID, courier)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierAssignmentService_AllCouriersAtCapacity(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	orderSvc := NewOrderService(db, log, newTestPricingService(), nil)
	courierSvc := NewCourierService(db, log)
	service := NewCourierAssignmentService(db, courierSvc, orderSvc, log)

	now := time.Now()
	orderID := uuid.New()

	expectOrderGet(mock, orderID, models.OrderStatusCreated, nil, now)

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon").
		WillReturnRows(sqlmockRowsCouriers(uuid.New(), models.CourierStatusAvailable, 55.0, 37.0, now))

	// Курьер на пределе ёмкости: 5 активных заказов
	mock.ExpectQuery("SELECT COUNT").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmockRowsCount(maxActiveOrdersPerCourier))

	if _, err := service.AutoAssignCourier(context.Background(), orderID, 55.1, 37.1); err == nil {
		t.Fatalf("expected error when all couriers at capacity")
	}
}

// --- фильтры available-курьеров ---
