package models

import "time"

// DiscountType описывает тип промокода.
type DiscountType string

const (
	DiscountTypeFixed        DiscountType = "fixed"
	DiscountTypePercent      DiscountType = "percent"
	DiscountTypeFreeDelivery DiscountType = "free_delivery"
)

// PromoCode представляет промокод в системе.
type PromoCode struct {
	Code         string       `json:"code" db:"code"`
	DiscountType DiscountType `json:"discount_type" db:"discount_type"`
	Amount       float64      `json:"amount" db:"amount"`
	MaxUses      int          `json:"max_uses" db:"max_uses"`
	UsedCount    int          `json:"used_count" db:"used_count"`
	ExpiresAt    *time.Time   `json:"expires_at,omitempty" db:"expires_at"`
	Active       bool         `json:"active" db:"active"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// CreatePromoCodeRequest описывает запрос на создание промокода.
type CreatePromoCodeRequest struct {
	Code         string       `json:"code"`
	DiscountType DiscountType `json:"discount_type"`
	Amount       float64      `json:"amount"`
	MaxUses      int          `json:"max_uses,omitempty"` // 0 = безлимит
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`
	Active       bool         `json:"active"`
}

// UpdatePromoCodeRequest описывает запрос на обновление промокода.
type UpdatePromoCodeRequest struct {
	DiscountType DiscountType `json:"discount_type"`
	Amount       float64      `json:"amount"`
	MaxUses      int          `json:"max_uses,omitempty"`
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`
	Active       bool         `json:"active"`
}
