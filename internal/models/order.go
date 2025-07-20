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
	Items           []OrderItem `json:"items"`
	TotalAmount     float64     `json:"total_amount" db:"total_amount"`
	Status          OrderStatus `json:"status" db:"status"`
	CourierID       *uuid.UUID  `json:"courier_id,omitempty" db:"courier_id"`
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
	Items           []CreateOrderItemRequest `json:"items"`
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
