package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"
)

const (
	defaultTopLimitFallback     = 5
	defaultCourierLimitFallback = 50
)

// AnalyticsHandler обрабатывает эндпоинты аналитики.
type AnalyticsHandler struct {
	service AnalyticsProvider
	log     *logger.Logger
	cfg     *config.AnalyticsConfig
}

// NewAnalyticsHandler создает новый обработчик аналитики.
func NewAnalyticsHandler(service AnalyticsProvider, log *logger.Logger, cfg *config.AnalyticsConfig) *AnalyticsHandler {
	return &AnalyticsHandler{
		service: service,
		log:     log,
		cfg:     cfg,
	}
}

// GetKPIs возвращает KPI с возможностью экспорта в CSV.
func (h *AnalyticsHandler) GetKPIs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	filter, format, err := parseAnalyticsFilter(r, h.cfg)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), analyticsTimeout(h.cfg))
	defer cancel()

	metrics, err := h.service.GetKPIs(ctx, filter)
	if err != nil {
		h.log.WithError(err).Error("Failed to load KPI metrics")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to load analytics")
		return
	}

	if format == "csv" {
		if err := writeKPICSV(w, metrics); err != nil {
			h.log.WithError(err).Warn("Failed to stream KPI CSV")
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, metrics)
}

// GetCourierAnalytics возвращает метрики по курьерам с опциональным CSV.
func (h *AnalyticsHandler) GetCourierAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	filter, format, err := parseAnalyticsFilter(r, h.cfg)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), analyticsTimeout(h.cfg))
	defer cancel()

	metrics, err := h.service.GetCourierAnalytics(ctx, filter)
	if err != nil {
		h.log.WithError(err).Error("Failed to load courier analytics")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to load analytics")
		return
	}

	if format == "csv" {
		if err := writeCourierCSV(w, metrics); err != nil {
			h.log.WithError(err).Warn("Failed to stream courier CSV")
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, metrics)
}

func parseAnalyticsFilter(r *http.Request, cfg *config.AnalyticsConfig) (*models.AnalyticsFilter, string, error) {
	query := r.URL.Query()
	now := time.Now().UTC()

	toParam := query.Get("to")
	fromParam := query.Get("from")

	to := endOfDay(now)
	if toParam != "" {
		parsed, err := time.Parse("2006-01-02", toParam)
		if err != nil {
			return nil, "", fmt.Errorf("invalid 'to' date, expected YYYY-MM-DD")
		}
		to = endOfDay(parsed)
	}

	maxRangeDays := 365
	if cfg != nil && cfg.MaxRangeDays > 0 {
		maxRangeDays = cfg.MaxRangeDays
	}

	from := startOfDay(now.AddDate(0, 0, -maxRangeDays+1))
	if fromParam != "" {
		parsed, err := time.Parse("2006-01-02", fromParam)
		if err != nil {
			return nil, "", fmt.Errorf("invalid 'from' date, expected YYYY-MM-DD")
		}
		from = startOfDay(parsed)
	}

	minAllowedFrom := to.AddDate(0, 0, -maxRangeDays+1)
	if from.Before(minAllowedFrom) {
		return nil, "", fmt.Errorf("date range too wide, max %d days", maxRangeDays)
	}

	if from.After(to) {
		return nil, "", fmt.Errorf("'from' date must be before 'to' date")
	}

	groupByStr := strings.ToLower(query.Get("group_by"))
	defaultGroupBy := models.AnalyticsGroupNone
	if cfg != nil {
		switch strings.ToLower(cfg.DefaultGroupBy) {
		case "day", "week", "month", "none":
			defaultGroupBy = models.AnalyticsGroupBy(strings.ToLower(cfg.DefaultGroupBy))
		}
	}

	groupBy := models.AnalyticsGroupBy(groupByStr)
	if groupByStr == "" {
		groupBy = defaultGroupBy
	} else if groupBy != models.AnalyticsGroupDay && groupBy != models.AnalyticsGroupWeek && groupBy != models.AnalyticsGroupMonth && groupBy != models.AnalyticsGroupNone {
		return nil, "", fmt.Errorf("group_by must be one of: day, week, month, none")
	}

	topDefault := defaultTopLimitFallback
	courierDefault := defaultCourierLimitFallback
	if cfg != nil {
		if cfg.DefaultTopLimit > 0 {
			topDefault = cfg.DefaultTopLimit
		}
		if cfg.DefaultCourierLimit > 0 {
			courierDefault = cfg.DefaultCourierLimit
		}
	}

	topLimit := parseIntWithDefault(query.Get("top_limit"), topDefault)
	courierLimit := parseIntWithDefault(query.Get("limit"), courierDefault)

	format := strings.ToLower(query.Get("format"))
	if format != "" && format != "json" && format != "csv" {
		return nil, "", fmt.Errorf("format must be json or csv")
	}

	filter := &models.AnalyticsFilter{
		From:          from,
		To:            to,
		GroupBy:       groupBy,
		TopItemsLimit: topLimit,
		CourierLimit:  courierLimit,
	}

	return filter, format, nil
}

func parseIntWithDefault(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}

	return parsed
}

func writeKPICSV(w http.ResponseWriter, metrics *models.KPIMetrics) error {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=kpi.csv")
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"section", "period", "revenue", "orders_count", "avg_delivery_time_minutes"})
	rangeLabel := fmt.Sprintf("%s..%s", metrics.From.Format("2006-01-02"), metrics.To.Format("2006-01-02"))
	_ = writer.Write([]string{"summary", rangeLabel, fmt.Sprintf("%.2f", metrics.Revenue), strconv.Itoa(metrics.OrdersCount), fmt.Sprintf("%.2f", metrics.AvgDeliveryTimeMinutes)})

	for _, period := range metrics.Periods {
		_ = writer.Write([]string{"period", period.Period, fmt.Sprintf("%.2f", period.Revenue), strconv.Itoa(period.OrdersCount), fmt.Sprintf("%.2f", period.AvgDeliveryTimeMinutes)})
	}

	_ = writer.Write([]string{})
	_ = writer.Write([]string{"section", "item_name", "quantity", "revenue"})
	for _, item := range metrics.TopItems {
		_ = writer.Write([]string{"top_item", item.Name, strconv.Itoa(item.Quantity), fmt.Sprintf("%.2f", item.Revenue)})
	}

	writer.Flush()
	return writer.Error()
}

func writeCourierCSV(w http.ResponseWriter, metrics []*models.CourierAnalytics) error {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=couriers.csv")
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"courier_id", "courier_name", "deliveries", "revenue", "rating", "avg_delivery_time_minutes"})

	for _, row := range metrics {
		_ = writer.Write([]string{
			row.CourierID.String(),
			row.CourierName,
			strconv.Itoa(row.Deliveries),
			fmt.Sprintf("%.2f", row.Revenue),
			fmt.Sprintf("%.2f", row.Rating),
			fmt.Sprintf("%.2f", row.AvgDeliveryTimeMinutes),
		})
	}

	writer.Flush()
	return writer.Error()
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Millisecond*999), time.UTC)
}

func analyticsTimeout(cfg *config.AnalyticsConfig) time.Duration {
	if cfg != nil && cfg.RequestTimeoutSeconds > 0 {
		return time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	}
	return 5 * time.Second
}
