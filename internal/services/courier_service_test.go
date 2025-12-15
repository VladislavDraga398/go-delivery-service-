package services

import (
	"testing"
	"time"

	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestCourierService_GetCouriers_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	status := models.CourierStatusAvailable
	minRating := 4.5
	limit, offset := 10, 0

	rows := sqlmock.NewRows([]string{"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at"}).
		AddRow(uuid.New(), "John Doe", "+7000", status, 55.0, 37.0, 4.6, 3, time.Now(), time.Now(), time.Now())

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon, rating, total_reviews,\\s+created_at, updated_at, last_seen_at\\s+FROM couriers").
		WithArgs(status, minRating, limit).
		WillReturnRows(rows)

	couriers, err := service.GetCouriers(&status, &minRating, limit, offset, "created_at")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(couriers) != 1 {
		t.Fatalf("expected 1 courier, got %d", len(couriers))
	}

	if couriers[0].Rating < minRating {
		t.Fatalf("expected rating >= %v, got %v", minRating, couriers[0].Rating)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCourierService_GetCouriers_NoFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewCourierService(db, log)

	rows := sqlmock.NewRows([]string{"id", "name", "phone", "status", "current_lat", "current_lon", "rating", "total_reviews", "created_at", "updated_at", "last_seen_at"}).
		AddRow(uuid.New(), "Alice", "+7001", models.CourierStatusOffline, nil, nil, 0.0, 0, time.Now(), time.Now(), nil)

	mock.ExpectQuery("SELECT id, name, phone, status, current_lat, current_lon, rating, total_reviews,\\s+created_at, updated_at, last_seen_at\\s+FROM couriers").
		WillReturnRows(rows)

	couriers, err := service.GetCouriers(nil, nil, 0, 0, "created_at")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(couriers) != 1 {
		t.Fatalf("expected 1 courier, got %d", len(couriers))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
