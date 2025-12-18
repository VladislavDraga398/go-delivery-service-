package handlers

import (
	"context"
	"time"

	"delivery-system/internal/models"

	"github.com/google/uuid"
)

// ----- Orders -----

type OrderService interface {
	CreateOrder(ctx context.Context, req *models.CreateOrderRequest) (*models.Order, error)
	GetOrder(ctx context.Context, orderID uuid.UUID) (*models.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, req *models.UpdateOrderStatusRequest) error
	GetOrders(ctx context.Context, status *models.OrderStatus, courierID *uuid.UUID, limit, offset int) ([]*models.Order, error)
	CreateReview(ctx context.Context, orderID uuid.UUID, req *models.CreateReviewRequest) (*models.Review, error)
	GetCourierReviews(ctx context.Context, courierID uuid.UUID, limit, offset int) ([]*models.Review, error)
}

type AssignmentService interface {
	AutoAssignCourier(ctx context.Context, orderID uuid.UUID, deliveryLat, deliveryLon float64) (*models.Courier, error)
}

type GeocodingService interface {
	Geocode(ctx context.Context, address string) (float64, float64, error)
}

type EventProducer interface {
	PublishOrderCreated(order *models.Order) error
	PublishOrderStatusChanged(orderID uuid.UUID, oldStatus, newStatus models.OrderStatus, courierID *uuid.UUID) error
	PublishCourierStatusChanged(courierID uuid.UUID, oldStatus, newStatus models.CourierStatus) error
	PublishLocationUpdated(courierID uuid.UUID, lat, lon float64) error
	PublishCourierAssigned(orderID, courierID uuid.UUID) error
}

type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
	DeleteByPrefix(ctx context.Context, prefix string) error
}

// ----- Couriers -----

type CourierService interface {
	CreateCourier(ctx context.Context, req *models.CreateCourierRequest) (*models.Courier, error)
	GetCourier(ctx context.Context, courierID uuid.UUID) (*models.Courier, error)
	UpdateCourierStatus(ctx context.Context, courierID uuid.UUID, req *models.UpdateCourierStatusRequest) error
	GetCouriers(ctx context.Context, status *models.CourierStatus, minRating *float64, limit, offset int, orderBy string) ([]*models.Courier, error)
	GetAvailableCouriers(ctx context.Context) ([]*models.Courier, error)
	AssignOrderToCourier(ctx context.Context, orderID, courierID uuid.UUID) error
}

// ----- Promo -----

type PromoService interface {
	CreatePromoCode(ctx context.Context, req *models.CreatePromoCodeRequest) (*models.PromoCode, error)
	GetPromoCode(ctx context.Context, code string) (*models.PromoCode, error)
	UpdatePromoCode(ctx context.Context, code string, req *models.UpdatePromoCodeRequest) (*models.PromoCode, error)
	DeletePromoCode(ctx context.Context, code string) error
	ListPromoCodes(ctx context.Context, limit, offset int) ([]*models.PromoCode, error)
}

// ----- Analytics -----

type AnalyticsProvider interface {
	GetKPIs(ctx context.Context, filter *models.AnalyticsFilter) (*models.KPIMetrics, error)
	GetCourierAnalytics(ctx context.Context, filter *models.AnalyticsFilter) ([]*models.CourierAnalytics, error)
}

// ----- Health -----

type DBHealth interface {
	Health() error
}

type RedisHealth interface {
	Health(ctx context.Context) error
}
