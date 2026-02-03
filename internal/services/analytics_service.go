package services

import (
	"context"
	"fmt"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/database"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"
	"delivery-system/internal/redis"
)

const (
	DefaultTopItemsLimit = 5
	DefaultCourierLimit  = 50
	defaultCacheTTL      = 10 * time.Minute
)

// AnalyticsService агрегирует бизнес-метрики и кеширует тяжёлые выборки.
type AnalyticsService struct {
	db              *database.DB
	redis           *redis.Client
	log             *logger.Logger
	cacheTTL        time.Duration
	defaultTopItems int
	defaultCouriers int
	defaultGroupBy  models.AnalyticsGroupBy
}

// NewAnalyticsService создает новый сервис аналитики.
func NewAnalyticsService(db *database.DB, redisClient *redis.Client, log *logger.Logger, cfg *config.AnalyticsConfig) *AnalyticsService {
	cacheTTL := defaultCacheTTL
	defaultTop := DefaultTopItemsLimit
	defaultCouriers := DefaultCourierLimit
	groupBy := models.AnalyticsGroupNone

	if cfg != nil {
		if cfg.CacheTTLMinutes > 0 {
			cacheTTL = time.Duration(cfg.CacheTTLMinutes) * time.Minute
		}
		if cfg.DefaultTopLimit > 0 {
			defaultTop = cfg.DefaultTopLimit
		}
		if cfg.DefaultCourierLimit > 0 {
			defaultCouriers = cfg.DefaultCourierLimit
		}
		switch models.AnalyticsGroupBy(cfg.DefaultGroupBy) {
		case models.AnalyticsGroupDay, models.AnalyticsGroupWeek, models.AnalyticsGroupMonth, models.AnalyticsGroupNone:
			groupBy = models.AnalyticsGroupBy(cfg.DefaultGroupBy)
		}
	}

	return &AnalyticsService{
		db:              db,
		redis:           redisClient,
		log:             log,
		cacheTTL:        cacheTTL,
		defaultTopItems: defaultTop,
		defaultCouriers: defaultCouriers,
		defaultGroupBy:  groupBy,
	}
}

// GetKPIs возвращает агрегированные KPI с опциональной группировкой и кешированием.
func (s *AnalyticsService) GetKPIs(ctx context.Context, filter *models.AnalyticsFilter) (*models.KPIMetrics, error) {
	filter = s.normalizeFilter(filter)
	cacheKey := s.buildCacheKey("kpi", filter)

	var cached models.KPIMetrics
	if s.tryGetFromCache(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	summary, err := s.fetchKPISummary(ctx, filter)
	if err != nil {
		return nil, err
	}

	periods, err := s.fetchKPIPeriods(ctx, filter)
	if err != nil {
		return nil, err
	}

	topItems, err := s.fetchTopItems(ctx, filter)
	if err != nil {
		return nil, err
	}

	result := &models.KPIMetrics{
		From:                   filter.From,
		To:                     filter.To,
		Revenue:                summary.Revenue,
		OrdersCount:            summary.OrdersCount,
		AvgDeliveryTimeMinutes: summary.AvgDeliveryTimeMinutes,
		AverageCheck:           summary.AverageCheck,
		TopItems:               topItems,
		Periods:                periods,
		GeneratedAt:            time.Now(),
		GroupBy:                string(filter.GroupBy),
	}

	s.saveToCache(ctx, cacheKey, result)
	return result, nil
}

// GetCourierAnalytics возвращает метрики по курьерам (доставки, выручка, рейтинг).
func (s *AnalyticsService) GetCourierAnalytics(ctx context.Context, filter *models.AnalyticsFilter) ([]*models.CourierAnalytics, error) {
	filter = s.normalizeFilter(filter)
	cacheKey := s.buildCacheKey("couriers", filter)

	var cached []*models.CourierAnalytics
	if s.tryGetFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	query := `
		SELECT c.id,
		       c.name,
		       c.rating,
		       COUNT(o.id) AS deliveries,
		       COALESCE(SUM(o.total_amount), 0) AS revenue,
		       COALESCE(AVG(EXTRACT(EPOCH FROM (o.delivered_at - o.created_at)) / 60), 0) AS avg_delivery_minutes
		FROM couriers c
		LEFT JOIN orders o ON o.courier_id = c.id 
			AND o.status = 'delivered'
			AND o.delivered_at BETWEEN $1 AND $2
	GROUP BY c.id, c.name, c.rating
	ORDER BY deliveries DESC, revenue DESC, c.rating DESC, c.name ASC
	`

	args := []interface{}{filter.From, filter.To}
	if filter.CourierLimit > 0 {
		query += " LIMIT $3"
		args = append(args, filter.CourierLimit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load courier analytics: %w", err)
	}
	defer rows.Close()

	var result []*models.CourierAnalytics
	for rows.Next() {
		item := &models.CourierAnalytics{}
		if err := rows.Scan(&item.CourierID, &item.CourierName, &item.Rating, &item.Deliveries, &item.Revenue, &item.AvgDeliveryTimeMinutes); err != nil {
			return nil, fmt.Errorf("failed to scan courier analytics: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate courier analytics: %w", err)
	}

	s.saveToCache(ctx, cacheKey, result)
	return result, nil
}

type kpiSummary struct {
	Revenue                float64
	OrdersCount            int
	AvgDeliveryTimeMinutes float64
	AverageCheck           float64
}

func (s *AnalyticsService) fetchKPISummary(ctx context.Context, filter *models.AnalyticsFilter) (*kpiSummary, error) {
	query := `
		SELECT COALESCE(SUM(total_amount), 0) AS revenue,
		       COUNT(*) AS orders_count,
		       COALESCE(AVG(EXTRACT(EPOCH FROM (delivered_at - created_at)) / 60), 0) AS avg_delivery_minutes,
		       COALESCE(AVG(total_amount), 0) AS average_check
	FROM orders
	WHERE status = 'delivered' AND delivered_at BETWEEN $1 AND $2
	`

	row := s.db.QueryRowContext(ctx, query, filter.From, filter.To)
	summary := &kpiSummary{}
	if err := row.Scan(&summary.Revenue, &summary.OrdersCount, &summary.AvgDeliveryTimeMinutes, &summary.AverageCheck); err != nil {
		return nil, fmt.Errorf("failed to load KPI summary: %w", err)
	}

	return summary, nil
}

func (s *AnalyticsService) fetchKPIPeriods(ctx context.Context, filter *models.AnalyticsFilter) ([]KPIPeriod, error) {
	if filter.GroupBy == models.AnalyticsGroupNone || !filter.IncludePeriods {
		return nil, nil
	}

	periodExpr := "date_trunc('day', delivered_at)"
	switch filter.GroupBy {
	case models.AnalyticsGroupWeek:
		periodExpr = "date_trunc('week', delivered_at)"
	case models.AnalyticsGroupMonth:
		periodExpr = "date_trunc('month', delivered_at)"
	}

	query := fmt.Sprintf(`
		SELECT %[1]s AS period,
		       COALESCE(SUM(total_amount), 0) AS revenue,
		       COUNT(*) AS orders_count,
		       COALESCE(AVG(EXTRACT(EPOCH FROM (delivered_at - created_at)) / 60), 0) AS avg_delivery_minutes
	FROM orders
	WHERE status = 'delivered' AND delivered_at BETWEEN $1 AND $2
	GROUP BY period
	ORDER BY period ASC
	`, periodExpr)

	rows, err := s.db.QueryContext(ctx, query, filter.From, filter.To)
	if err != nil {
		return nil, fmt.Errorf("failed to load KPI periods: %w", err)
	}
	defer rows.Close()

	var result []KPIPeriod
	for rows.Next() {
		var (
			periodTime time.Time
			item       KPIPeriod
		)
		if err := rows.Scan(&periodTime, &item.Revenue, &item.OrdersCount, &item.AvgDeliveryTimeMinutes); err != nil {
			return nil, fmt.Errorf("failed to scan KPI period: %w", err)
		}
		item.Period = formatPeriod(periodTime, filter.GroupBy)
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate KPI periods: %w", err)
	}

	return result, nil
}

func (s *AnalyticsService) fetchTopItems(ctx context.Context, filter *models.AnalyticsFilter) ([]models.TopItem, error) {
	query := `
		SELECT oi.name,
		       COALESCE(SUM(oi.quantity), 0) AS total_quantity,
		       COALESCE(SUM(oi.price * oi.quantity), 0) AS revenue
	FROM order_items oi
	JOIN orders o ON o.id = oi.order_id
	WHERE o.status = 'delivered' AND o.delivered_at BETWEEN $1 AND $2
	GROUP BY oi.name
	ORDER BY total_quantity DESC, revenue DESC, oi.name ASC
	LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, filter.From, filter.To, filter.TopItemsLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to load top items: %w", err)
	}
	defer rows.Close()

	var result []models.TopItem
	for rows.Next() {
		var item models.TopItem
		if err := rows.Scan(&item.Name, &item.Quantity, &item.Revenue); err != nil {
			return nil, fmt.Errorf("failed to scan top item: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate top items: %w", err)
	}

	return result, nil
}

// KPIPeriod описывает агрегированные метрики по выбранному интервалу.
type KPIPeriod = models.KPIPeriod

func (s *AnalyticsService) buildCacheKey(kind string, filter *models.AnalyticsFilter) string {
	return redis.GenerateKey(redis.KeyPrefixStats, fmt.Sprintf(
		"%s:%s:%s:%s:%d:%d:%t",
		kind,
		filter.From.Format("2006-01-02"),
		filter.To.Format("2006-01-02"),
		filter.GroupBy,
		filter.TopItemsLimit,
		filter.CourierLimit,
		filter.IncludePeriods,
	))
}

func (s *AnalyticsService) normalizeFilter(filter *models.AnalyticsFilter) *models.AnalyticsFilter {
	if filter.TopItemsLimit <= 0 {
		filter.TopItemsLimit = s.defaultTopItems
	}
	if filter.CourierLimit <= 0 {
		filter.CourierLimit = s.defaultCouriers
	}
	if filter.GroupBy == "" {
		filter.GroupBy = s.defaultGroupBy
	}
	filter.IncludePeriods = filter.GroupBy != models.AnalyticsGroupNone
	return filter
}

func (s *AnalyticsService) tryGetFromCache(ctx context.Context, key string, dest interface{}) bool {
	if s.redis == nil {
		return false
	}

	if err := s.redis.Get(ctx, key, dest); err != nil {
		return false
	}
	return true
}

func (s *AnalyticsService) saveToCache(ctx context.Context, key string, value interface{}) {
	if s.redis == nil {
		return
	}

	if err := s.redis.Set(ctx, key, value, s.cacheTTL); err != nil {
		s.log.WithError(err).WithField("key", key).Warn("Failed to cache analytics result")
	}
}

func formatPeriod(period time.Time, groupBy models.AnalyticsGroupBy) string {
	switch groupBy {
	case models.AnalyticsGroupWeek:
		return period.Format("2006-01-02") // начало недели
	case models.AnalyticsGroupMonth:
		return period.Format("2006-01")
	default:
		return period.Format("2006-01-02")
	}
}
