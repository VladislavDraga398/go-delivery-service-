package models

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus представляет статус заказа
type OrderStatus string

const (
	OrderStatusCreated    OrderStatus = "created"
	OrderStatusAccepted   OrderStatus = "accepted"
	OrderStatusPreparing  OrderStatus = "preparing"
	OrderStatusReady      OrderStatus = "ready"
	OrderStatusInDelivery OrderStatus = "in_delivery"
	OrderStatusDelivered  OrderStatus = "delivered"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

// Order представляет заказ в системе
type Order struct {
	ID              uuid.UUID   `json:"id" db:"id"`
	CustomerName    string      `json:"customer_name" db:"customer_name"`
	CustomerPhone   string      `json:"customer_phone" db:"customer_phone"`
	DeliveryAddress string      `json:"delivery_address" db:"delivery_address"`
	PickupAddress   string      `json:"pickup_address" db:"pickup_address"`
	PickupLat       *float64    `json:"pickup_lat,omitempty" db:"pickup_lat"`
	PickupLon       *float64    `json:"pickup_lon,omitempty" db:"pickup_lon"`
	DeliveryLat     *float64    `json:"delivery_lat,omitempty" db:"delivery_lat"`
	DeliveryLon     *float64    `json:"delivery_lon,omitempty" db:"delivery_lon"`
	Items           []OrderItem `json:"items"`
	TotalAmount     float64     `json:"total_amount" db:"total_amount"`
	DeliveryCost    float64     `json:"delivery_cost" db:"delivery_cost"`
	DiscountAmount  float64     `json:"discount_amount" db:"discount_amount"`
	PromoCode       *string     `json:"promo_code,omitempty" db:"promo_code"`
	Status          OrderStatus `json:"status" db:"status"`
	CourierID       *uuid.UUID  `json:"courier_id,omitempty" db:"courier_id"`
	Rating          *int        `json:"rating,omitempty" db:"rating"`
	ReviewComment   *string     `json:"review_comment,omitempty" db:"review_comment"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
	DeliveredAt     *time.Time  `json:"delivered_at,omitempty" db:"delivered_at"`
}

// OrderItem представляет товар в заказе
type OrderItem struct {
	ID       uuid.UUID `json:"id" db:"id"`
	OrderID  uuid.UUID `json:"order_id" db:"order_id"`
	Name     string    `json:"name" db:"name"`
	Quantity int       `json:"quantity" db:"quantity"`
	Price    float64   `json:"price" db:"price"`
}

// CreateOrderRequest представляет запрос на создание заказа
type CreateOrderRequest struct {
	CustomerName    string                   `json:"customer_name"`
	CustomerPhone   string                   `json:"customer_phone"`
	DeliveryAddress string                   `json:"delivery_address"`
	PickupAddress   string                   `json:"pickup_address"`
	Items           []CreateOrderItemRequest `json:"items"`
	AutoAssign      bool                     `json:"auto_assign,omitempty"`
	PickupLat       *float64                 `json:"pickup_lat,omitempty"`
	PickupLon       *float64                 `json:"pickup_lon,omitempty"`
	DeliveryLat     *float64                 `json:"delivery_lat,omitempty"`
	DeliveryLon     *float64                 `json:"delivery_lon,omitempty"`
	PromoCode       *string                  `json:"promo_code,omitempty"`
}

// CreateOrderItemRequest представляет запрос на создание товара в заказе
type CreateOrderItemRequest struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// UpdateOrderStatusRequest представляет запрос на обновление статуса заказа
type UpdateOrderStatusRequest struct {
	Status    OrderStatus `json:"status"`
	CourierID *uuid.UUID  `json:"courier_id,omitempty"`
}

// Review представляет отзыв о заказе/курьере
type Review struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrderID   uuid.UUID `json:"order_id" db:"order_id"`
	CourierID uuid.UUID `json:"courier_id" db:"courier_id"`
	Rating    int       `json:"rating" db:"rating"`
	Comment   *string   `json:"comment,omitempty" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CreateReviewRequest представляет запрос на создание отзыва по заказу
type CreateReviewRequest struct {
	Rating  int     `json:"rating"`
	Comment *string `json:"comment,omitempty"`
}
