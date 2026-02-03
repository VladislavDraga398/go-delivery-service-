package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/google/uuid"
)

type stubAnalyticsService struct {
	kpi      *models.KPIMetrics
	couriers []*models.CourierAnalytics
	err      error
}

func (s *stubAnalyticsService) GetKPIs(ctx context.Context, filter *models.AnalyticsFilter) (*models.KPIMetrics, error) {
	return s.kpi, s.err
}

func (s *stubAnalyticsService) GetCourierAnalytics(ctx context.Context, filter *models.AnalyticsFilter) ([]*models.CourierAnalytics, error) {
	return s.couriers, s.err
}

func TestAnalyticsHandler_GetKPIs_JSON(t *testing.T) {
	cfg := &config.AnalyticsConfig{
		MaxRangeDays: 30,
	}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	kpi := &models.KPIMetrics{
		From:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2024, 1, 2, 23, 59, 59, 0, time.UTC),
		Revenue:     1000,
		OrdersCount: 5,
		TopItems: []models.TopItem{
			{Name: "Pizza", Quantity: 3, Revenue: 600},
		},
	}
	h := NewAnalyticsHandler(&stubAnalyticsService{kpi: kpi}, log, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-01&to=2024-01-02&group_by=none", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp models.KPIMetrics
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.OrdersCount != kpi.OrdersCount || resp.Revenue != kpi.Revenue {
		t.Fatalf("unexpected KPI response: %+v", resp)
	}
}

func TestAnalyticsHandler_GetCourierAnalytics_CSV(t *testing.T) {
	cfg := &config.AnalyticsConfig{
		MaxRangeDays: 30,
	}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	couriers := []*models.CourierAnalytics{
		{
			CourierID:              testUUID("11111111-1111-1111-1111-111111111111"),
			CourierName:            "John",
			Deliveries:             10,
			Revenue:                500,
			Rating:                 4.7,
			AvgDeliveryTimeMinutes: 35.5,
		},
	}
	h := NewAnalyticsHandler(&stubAnalyticsService{couriers: couriers}, log, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/couriers?from=2024-01-01&to=2024-01-02&format=csv", nil)
	rr := httptest.NewRecorder()

	h.GetCourierAnalytics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("expected text/csv content type, got %s", ct)
	}

	if body := rr.Body.String(); !strings.Contains(body, "John") || !strings.Contains(body, "courier_name") {
		t.Fatalf("unexpected CSV body: %s", body)
	}
}

func TestAnalyticsHandler_MaxRange_TooWide(t *testing.T) {
	cfg := &config.AnalyticsConfig{
		MaxRangeDays: 7,
	}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	h := NewAnalyticsHandler(&stubAnalyticsService{}, log, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-01&to=2024-02-01", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestAnalyticsHandler_MethodNotAllowed(t *testing.T) {
	h := NewAnalyticsHandler(&stubAnalyticsService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodPost, "/api/analytics/kpi", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestAnalyticsHandler_InvalidGroupBy(t *testing.T) {
	h := NewAnalyticsHandler(&stubAnalyticsService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-01&to=2024-01-02&group_by=year", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyticsHandler_InvalidFormat(t *testing.T) {
	h := NewAnalyticsHandler(&stubAnalyticsService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-01&to=2024-01-02&format=xml", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyticsHandler_InvalidToDate(t *testing.T) {
	h := NewAnalyticsHandler(&stubAnalyticsService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?to=2024-99-99", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyticsHandler_FromAfterTo(t *testing.T) {
	h := NewAnalyticsHandler(&stubAnalyticsService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-10&to=2024-01-01", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestParseAnalyticsFilter_DefaultsFromConfig(t *testing.T) {
	cfg := &config.AnalyticsConfig{
		MaxRangeDays:        30,
		DefaultGroupBy:      "day",
		DefaultTopLimit:     7,
		DefaultCourierLimit: 11,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-01&to=2024-01-02&top_limit=bad&limit=-1", nil)
	filter, format, err := parseAnalyticsFilter(req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "" {
		t.Fatalf("expected empty format, got %q", format)
	}
	if filter.GroupBy != models.AnalyticsGroupDay {
		t.Fatalf("expected group_by day, got %s", filter.GroupBy)
	}
	if filter.TopItemsLimit != 7 || filter.CourierLimit != 11 {
		t.Fatalf("unexpected defaults: top=%d courier=%d", filter.TopItemsLimit, filter.CourierLimit)
	}
}

func TestAnalyticsHandler_ServiceError(t *testing.T) {
	h := NewAnalyticsHandler(&stubAnalyticsService{err: fmt.Errorf("service error")}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-01&to=2024-01-02", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestAnalyticsHandler_CourierServiceError(t *testing.T) {
	h := NewAnalyticsHandler(&stubAnalyticsService{err: fmt.Errorf("service error")}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/couriers?from=2024-01-01&to=2024-01-02", nil)
	rr := httptest.NewRecorder()

	h.GetCourierAnalytics(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestAnalyticsHandler_GetKPIs_CSV(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	kpi := &models.KPIMetrics{
		From:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2024, 1, 2, 23, 0, 0, 0, time.UTC),
		Revenue:     100,
		OrdersCount: 2,
		Periods: []models.KPIPeriod{
			{Period: "2024-01-01", Revenue: 50, OrdersCount: 1, AvgDeliveryTimeMinutes: 10},
		},
		TopItems: []models.TopItem{{Name: "Item", Quantity: 1, Revenue: 50}},
	}
	h := NewAnalyticsHandler(&stubAnalyticsService{kpi: kpi}, log, &config.AnalyticsConfig{MaxRangeDays: 30})
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/kpi?from=2024-01-01&to=2024-01-02&format=csv", nil)
	rr := httptest.NewRecorder()

	h.GetKPIs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "summary") {
		t.Fatalf("expected csv content, got %s", rr.Body.String())
	}
}

func TestParseIntWithDefault(t *testing.T) {
	if v := parseIntWithDefault("", 5); v != 5 {
		t.Fatalf("expected default 5, got %d", v)
	}
	if v := parseIntWithDefault("10", 1); v != 10 {
		t.Fatalf("expected 10, got %d", v)
	}
	if v := parseIntWithDefault("bad", 3); v != 3 {
		t.Fatalf("expected fallback 3, got %d", v)
	}
}

func TestAnalyticsTimeout(t *testing.T) {
	if d := analyticsTimeout(nil); d != 5*time.Second {
		t.Fatalf("expected default 5s, got %v", d)
	}
	cfg := &config.AnalyticsConfig{RequestTimeoutSeconds: 2}
	if d := analyticsTimeout(cfg); d != 2*time.Second {
		t.Fatalf("expected configured timeout, got %v", d)
	}
}

func testUUID(val string) uuid.UUID {
	id, _ := uuid.Parse(val)
	return id
}
