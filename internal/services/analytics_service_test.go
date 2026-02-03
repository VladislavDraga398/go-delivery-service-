package services

import (
	"context"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/models"
	"delivery-system/internal/redis"

	"github.com/DATA-DOG/go-sqlmock"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func TestAnalyticsService_GetKPIs_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewAnalyticsService(db, nil, log, nil)

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	filter := &models.AnalyticsFilter{
		From:          from,
		To:            to,
		GroupBy:       models.AnalyticsGroupDay,
		TopItemsLimit: 3,
	}

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(total_amount\\), 0\\) AS revenue").
		WithArgs(from, to).
		WillReturnRows(sqlmock.NewRows([]string{"revenue", "orders_count", "avg_delivery_minutes", "average_check"}).
			AddRow(1250.50, 10, 42.0, 125.05))

	mock.ExpectQuery("SELECT date_trunc\\('day', delivered_at\\) AS period").
		WithArgs(from, to).
		WillReturnRows(sqlmock.NewRows([]string{"period", "revenue", "orders_count", "avg_delivery_minutes"}).
			AddRow(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 300.0, 3, 40.0).
			AddRow(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 950.5, 7, 43.0))

	mock.ExpectQuery("SELECT oi.name").
		WithArgs(from, to, 3).
		WillReturnRows(sqlmock.NewRows([]string{"name", "total_quantity", "revenue"}).
			AddRow("Pizza", 15, 800.0).
			AddRow("Soda", 10, 200.0))

	metrics, err := service.GetKPIs(context.Background(), filter)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if metrics.Revenue != 1250.50 || metrics.OrdersCount != 10 {
		t.Fatalf("unexpected metrics summary: %+v", metrics)
	}

	if len(metrics.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(metrics.Periods))
	}

	if len(metrics.TopItems) != 2 || metrics.TopItems[0].Name != "Pizza" {
		t.Fatalf("unexpected top items: %+v", metrics.TopItems)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAnalyticsService_GetCourierAnalytics_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewAnalyticsService(db, nil, log, nil)

	from := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 7, 23, 59, 59, 0, time.UTC)
	filter := &models.AnalyticsFilter{
		From:         from,
		To:           to,
		CourierLimit: 2,
	}

	courierID := uuid.New()

	mock.ExpectQuery("SELECT c.id").
		WithArgs(from, to, filter.CourierLimit).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rating", "deliveries", "revenue", "avg_delivery_minutes"}).
			AddRow(courierID, "John", 4.8, 12, 1500.0, 38.5))

	metrics, err := service.GetCourierAnalytics(context.Background(), filter)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 courier, got %d", len(metrics))
	}

	if metrics[0].CourierID != courierID || metrics[0].Deliveries != 12 {
		t.Fatalf("unexpected courier metrics: %+v", metrics[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAnalyticsService_GetKPIs_FromCache(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb, _ := redis.Connect(&config.RedisConfig{Host: "127.0.0.1", Port: mr.Port(), DB: 0}, newTestLogger())

	service := NewAnalyticsService(nil, rdb, newTestLogger(), &config.AnalyticsConfig{DefaultGroupBy: "none"})
	filter := &models.AnalyticsFilter{
		From:           time.Unix(0, 0),
		To:             time.Unix(0, 0),
		GroupBy:        models.AnalyticsGroupNone,
		TopItemsLimit:  DefaultTopItemsLimit,
		CourierLimit:   DefaultCourierLimit,
		IncludePeriods: false,
	}
	cacheKey := service.buildCacheKey("kpi", filter)
	expected := &models.KPIMetrics{OrdersCount: 5}
	_ = rdb.Set(context.Background(), cacheKey, expected, time.Minute)

	res, err := service.GetKPIs(context.Background(), filter)
	if err != nil {
		t.Fatalf("expected cache hit, got %v", err)
	}
	if res.OrdersCount != expected.OrdersCount {
		t.Fatalf("unexpected cache result")
	}
}

func TestAnalyticsService_SaveToCache(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb, _ := redis.Connect(&config.RedisConfig{Host: "127.0.0.1", Port: mr.Port(), DB: 0}, newTestLogger())

	svc := NewAnalyticsService(nil, rdb, newTestLogger(), &config.AnalyticsConfig{CacheTTLMinutes: 1})
	key := "analytics:test"
	svc.saveToCache(context.Background(), key, map[string]string{"ok": "yes"})

	if !mr.Exists(key) {
		t.Fatalf("expected key cached")
	}
	ttl := mr.TTL(key)
	if ttl <= 0 {
		t.Fatalf("expected ttl set, got %v", ttl)
	}
}

func TestFormatPeriod(t *testing.T) {
	date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if got := formatPeriod(date, models.AnalyticsGroupWeek); got != "2024-03-15" {
		t.Fatalf("unexpected week format: %s", got)
	}
	if got := formatPeriod(date, models.AnalyticsGroupMonth); got != "2024-03" {
		t.Fatalf("unexpected month format: %s", got)
	}
	if got := formatPeriod(date, models.AnalyticsGroupNone); got != "2024-03-15" {
		t.Fatalf("unexpected default format: %s", got)
	}
}
