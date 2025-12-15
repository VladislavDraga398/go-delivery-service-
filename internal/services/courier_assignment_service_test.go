package services

import (
	"testing"

	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

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

	log := newTestLogger()
	orderService := NewOrderService(db, log, newTestPricingService())
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
	score := assignmentService.calculateCourierScore(courier, targetLat, targetLon, weights)

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
