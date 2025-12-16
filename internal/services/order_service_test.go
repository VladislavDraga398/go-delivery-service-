package services

import (
	"database/sql"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/database"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func newTestLogger() *logger.Logger {
	return logger.New(&config.LoggerConfig{Level: "debug", Format: "json"})
}

func newTestPricingService() *PricingService {
	return NewPricingService(100, 20, 150)
}

func newMockDB(t *testing.T) (*database.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	return &database.DB{DB: db}, mock
}

func TestOrderService_CreateReview_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	courierID := uuid.New()
	comment := "great delivery"
	req := &models.CreateReviewRequest{Rating: 5, Comment: &comment}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT courier_id, status, rating FROM orders").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"courier_id", "status", "rating"}).
			AddRow(courierID, models.OrderStatusDelivered, sql.NullInt32{}))

	mock.ExpectExec("INSERT INTO reviews").
		WithArgs(sqlmock.AnyArg(), orderID, courierID, req.Rating, req.Comment, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("UPDATE orders").
		WithArgs(req.Rating, req.Comment, sqlmock.AnyArg(), orderID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	review, err := service.CreateReview(orderID, req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if review.Rating != req.Rating || review.OrderID != orderID || review.CourierID != courierID {
		t.Fatalf("unexpected review result: %+v", review)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_CreateReview_OrderNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	req := &models.CreateReviewRequest{Rating: 4}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT courier_id, status, rating FROM orders").
		WithArgs(orderID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	if _, err := service.CreateReview(orderID, req); err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_CreateReview_NotDelivered(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	courierID := uuid.New()
	req := &models.CreateReviewRequest{Rating: 3}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT courier_id, status, rating FROM orders").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"courier_id", "status", "rating"}).
			AddRow(courierID, models.OrderStatusCreated, sql.NullInt32{}))
	mock.ExpectRollback()

	if _, err := service.CreateReview(orderID, req); err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_CreateReview_AlreadyExists(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	courierID := uuid.New()
	req := &models.CreateReviewRequest{Rating: 4}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT courier_id, status, rating FROM orders").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"courier_id", "status", "rating"}).
			AddRow(courierID, models.OrderStatusDelivered, sql.NullInt32{Valid: true, Int32: 5}))
	mock.ExpectRollback()

	if _, err := service.CreateReview(orderID, req); err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrderService_CreateReview_InvalidRating(t *testing.T) {
	db, _ := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	orderID := uuid.New()
	req := &models.CreateReviewRequest{Rating: 6}

	if _, err := service.CreateReview(orderID, req); err == nil {
		t.Fatalf("expected error for invalid rating")
	}
}

func TestOrderService_GetCourierReviews(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewOrderService(db, log, newTestPricingService(), nil)

	courierID := uuid.New()
	limit, offset := 10, 0

	rows := sqlmock.NewRows([]string{"id", "order_id", "courier_id", "rating", "comment", "created_at"}).
		AddRow(uuid.New(), uuid.New(), courierID, 5, "great", time.Now()).
		AddRow(uuid.New(), uuid.New(), courierID, 4, "ok", time.Now())

	mock.ExpectQuery("SELECT id, order_id, courier_id, rating, comment, created_at FROM reviews").
		WithArgs(courierID, limit, offset).
		WillReturnRows(rows)

	reviews, err := service.GetCourierReviews(courierID, limit, offset)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(reviews) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(reviews))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
