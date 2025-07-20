package models

import (
	"time"

	"github.com/google/uuid"
)

// CourierStatus представляет статус курьера
type CourierStatus string

const (
	CourierStatusOffline   CourierStatus = "offline"
	CourierStatusAvailable CourierStatus = "available"
	CourierStatusBusy      CourierStatus = "busy"
)

// Courier представляет курьера в системе
type Courier struct {
	ID         uuid.UUID     `json:"id" db:"id"`
	Name       string        `json:"name" db:"name"`
	Phone      string        `json:"phone" db:"phone"`
	Status     CourierStatus `json:"status" db:"status"`
	CurrentLat *float64      `json:"current_lat,omitempty" db:"current_lat"`
	CurrentLon *float64      `json:"current_lon,omitempty" db:"current_lon"`
	CreatedAt  time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at" db:"updated_at"`
	LastSeenAt *time.Time    `json:"last_seen_at,omitempty" db:"last_seen_at"`
}

// CreateCourierRequest представляет запрос на создание курьера
type CreateCourierRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// UpdateCourierStatusRequest представляет запрос на обновление статуса курьера
type UpdateCourierStatusRequest struct {
	Status     CourierStatus `json:"status"`
	CurrentLat *float64      `json:"current_lat,omitempty"`
	CurrentLon *float64      `json:"current_lon,omitempty"`
}

// CourierLocation представляет местоположение курьера
type CourierLocation struct {
	CourierID uuid.UUID `json:"courier_id"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Timestamp time.Time `json:"timestamp"`
}
