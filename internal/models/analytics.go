package models

import (
	"time"

	"github.com/google/uuid"
)

// AnalyticsGroupBy описывает доступные варианты группировки периодов.
type AnalyticsGroupBy string

const (
	AnalyticsGroupNone  AnalyticsGroupBy = "none"
	AnalyticsGroupDay   AnalyticsGroupBy = "day"
	AnalyticsGroupWeek  AnalyticsGroupBy = "week"
	AnalyticsGroupMonth AnalyticsGroupBy = "month"
)

// AnalyticsFilter задает временной интервал и параметры агрегации.
type AnalyticsFilter struct {
	From           time.Time
	To             time.Time
	GroupBy        AnalyticsGroupBy
	TopItemsLimit  int
	CourierLimit   int
	IncludePeriods bool
}

// KPIMetrics описывает бизнес-показатели за период.
type KPIMetrics struct {
	From                   time.Time   `json:"from"`
	To                     time.Time   `json:"to"`
	Revenue                float64     `json:"revenue"`
	OrdersCount            int         `json:"orders_count"`
	AvgDeliveryTimeMinutes float64     `json:"avg_delivery_time_minutes"`
	AverageCheck           float64     `json:"average_check"`
	TopItems               []TopItem   `json:"top_items"`
	Periods                []KPIPeriod `json:"periods,omitempty"`
	GeneratedAt            time.Time   `json:"generated_at"`
	GroupBy                string      `json:"group_by,omitempty"`
}

// KPIPeriod хранит агрегированные метрики по периоду.
type KPIPeriod struct {
	Period                 string  `json:"period"`
	Revenue                float64 `json:"revenue"`
	OrdersCount            int     `json:"orders_count"`
	AvgDeliveryTimeMinutes float64 `json:"avg_delivery_time_minutes"`
}

// TopItem описывает популярный товар в заказах.
type TopItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Revenue  float64 `json:"revenue"`
}

// CourierAnalytics агрегирует метрики по курьерам.
type CourierAnalytics struct {
	CourierID              uuid.UUID `json:"courier_id"`
	CourierName            string    `json:"courier_name"`
	Rating                 float64   `json:"rating"`
	Deliveries             int       `json:"deliveries"`
	Revenue                float64   `json:"revenue"`
	AvgDeliveryTimeMinutes float64   `json:"avg_delivery_time_minutes"`
}
