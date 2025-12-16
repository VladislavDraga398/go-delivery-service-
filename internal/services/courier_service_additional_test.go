package services

import (
	"database/sql"
	"testing"
	"time"

	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestCourierService_CreateCourier_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	req := &models.CreateCourierRequest{
		Name:  "Test Courier",
		Phone: "+79001112233",
	}

	mock.ExpectExec("INSERT INTO couriers").
		WithArgs(sqlmock.AnyArg(), req.Name, req.Phone, models.CourierStatusOffline, 0.0, 0, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	courier, err := service.CreateCourier(req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if courier.Name != req.Name || courier.Phone != req.Phone {
		t.Fatalf("courier fields mismatch")
	}

	if courier.Status != models.CourierStatusOffline {
		t.Fatalf("expected status offline, got %v", courier.Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_CreateCourier_DuplicatePhone(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	req := &models.CreateCourierRequest{
		Name:  "Test Courier",
		Phone: "+79001112233",
	}

	mock.ExpectExec("INSERT INTO couriers").
		WithArgs(sqlmock.AnyArg(), req.Name, req.Phone, models.CourierStatusOffline, 0.0, 0, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)

	_, err := service.CreateCourier(req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_GetCourier_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	courierID := uuid.New()
	lat, lon := 55.75, 37.62

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon").
		WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at"}).
			AddRow(courierID, "Bob", "+79998887766", models.CourierStatusAvailable, lat, lon, 4.5, 10, time.Now(), time.Now(), time.Now()))

	courier, err := service.GetCourier(courierID)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if courier.ID != courierID {
		t.Fatalf("expected courier ID %v, got %v", courierID, courier.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_GetCourier_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	courierID := uuid.New()

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon").
		WithArgs(courierID).
		WillReturnError(sql.ErrNoRows)

	_, err := service.GetCourier(courierID)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_UpdateCourierStatus_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	courierID := uuid.New()
	lat, lon := 55.0, 37.0
	req := &models.UpdateCourierStatusRequest{
		Status:     models.CourierStatusAvailable,
		CurrentLat: &lat,
		CurrentLon: &lon,
	}

	mock.ExpectExec("UPDATE couriers SET status").
		WithArgs(req.Status, req.CurrentLat, req.CurrentLon, sqlmock.AnyArg(), sqlmock.AnyArg(), courierID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := service.UpdateCourierStatus(courierID, req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_UpdateCourierStatus_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	courierID := uuid.New()
	req := &models.UpdateCourierStatusRequest{
		Status: models.CourierStatusBusy,
	}

	mock.ExpectExec("UPDATE couriers SET status").
		WithArgs(req.Status, req.CurrentLat, req.CurrentLon, sqlmock.AnyArg(), sqlmock.AnyArg(), courierID).
		WillReturnResult(sqlmock.NewResult(1, 0))

	err := service.UpdateCourierStatus(courierID, req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_AssignOrderToCourier_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	orderID := uuid.New()
	courierID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM couriers WHERE id").
		WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).
			AddRow(models.CourierStatusAvailable))

	mock.ExpectExec("UPDATE orders SET courier_id").
		WithArgs(courierID, models.OrderStatusAccepted, sqlmock.AnyArg(), orderID, models.OrderStatusCreated).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("UPDATE couriers SET status").
		WithArgs(models.CourierStatusBusy, sqlmock.AnyArg(), courierID, models.CourierStatusAvailable).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	err := service.AssignOrderToCourier(orderID, courierID)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_AssignOrderToCourier_CourierNotAvailable(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	orderID := uuid.New()
	courierID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM couriers WHERE id").
		WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).
			AddRow(models.CourierStatusBusy))

	mock.ExpectRollback()

	err := service.AssignOrderToCourier(orderID, courierID)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_AssignOrderToCourier_CourierNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	orderID := uuid.New()
	courierID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM couriers WHERE id").
		WithArgs(courierID).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectRollback()

	err := service.AssignOrderToCourier(orderID, courierID)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_AssignOrderToCourier_OrderNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	orderID := uuid.New()
	courierID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM couriers WHERE id").
		WithArgs(courierID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).
			AddRow(models.CourierStatusAvailable))

	mock.ExpectExec("UPDATE orders SET courier_id").
		WithArgs(courierID, models.OrderStatusAccepted, sqlmock.AnyArg(), orderID, models.OrderStatusCreated).
		WillReturnResult(sqlmock.NewResult(1, 0))

	mock.ExpectRollback()

	err := service.AssignOrderToCourier(orderID, courierID)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_GetAvailableCouriers(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	rows := sqlmock.NewRows([]string{"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at"}).
		AddRow(uuid.New(), "Available Courier", "+79009998877", models.CourierStatusAvailable, 55.0, 37.0, 4.8, 5, time.Now(), time.Now(), time.Now())

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon, rating, total_reviews").
		WithArgs(models.CourierStatusAvailable).
		WillReturnRows(rows)

	couriers, err := service.GetAvailableCouriers()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(couriers) != 1 {
		t.Fatalf("expected 1 courier, got %d", len(couriers))
	}

	if couriers[0].Status != models.CourierStatusAvailable {
		t.Fatalf("expected status available, got %v", couriers[0].Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
