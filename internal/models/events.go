package models

import (
	"time"

	"github.com/google/uuid"
)

// EventType представляет тип события
type EventType string

const (
	EventTypeOrderCreated         EventType = "order.created"
	EventTypeOrderStatusChanged   EventType = "order.status_changed"
	EventTypeCourierAssigned      EventType = "courier.assigned"
	EventTypeCourierStatusChanged EventType = "courier.status_changed"
	EventTypeLocationUpdated      EventType = "location.updated"
)

// Event представляет базовое событие
type Event struct {
	ID        uuid.UUID   `json:"id"`
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// OrderCreatedEvent представляет событие создания заказа
type OrderCreatedEvent struct {
	OrderID         uuid.UUID `json:"order_id"`
	CustomerName    string    `json:"customer_name"`
	CustomerPhone   string    `json:"customer_phone"`
	DeliveryAddress string    `json:"delivery_address"`
	TotalAmount     float64   `json:"total_amount"`
}

// OrderStatusChangedEvent представляет событие изменения статуса заказа
type OrderStatusChangedEvent struct {
	OrderID   uuid.UUID   `json:"order_id"`
	OldStatus OrderStatus `json:"old_status"`
	NewStatus OrderStatus `json:"new_status"`
	CourierID *uuid.UUID  `json:"courier_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// CourierAssignedEvent представляет событие назначения курьера
type CourierAssignedEvent struct {
	OrderID   uuid.UUID `json:"order_id"`
	CourierID uuid.UUID `json:"courier_id"`
	Timestamp time.Time `json:"timestamp"`
}

// CourierStatusChangedEvent представляет событие изменения статуса курьера
type CourierStatusChangedEvent struct {
	CourierID uuid.UUID     `json:"courier_id"`
	OldStatus CourierStatus `json:"old_status"`
	NewStatus CourierStatus `json:"new_status"`
	Timestamp time.Time     `json:"timestamp"`
}

// LocationUpdatedEvent представляет событие обновления местоположения
type LocationUpdatedEvent struct {
	CourierID uuid.UUID `json:"courier_id"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Timestamp time.Time `json:"timestamp"`
}
