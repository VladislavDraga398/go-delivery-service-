package handlers

import (
	"testing"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"
)

// validateCreateOrderRequest uses no external deps, safe to call.
func TestValidateCreateOrderRequest(t *testing.T) {
	h := &OrderHandler{log: logger.New(&config.LoggerConfig{Level: "error", Format: "json"})}

	req := &models.CreateOrderRequest{
		CustomerName:    "Name",
		CustomerPhone:   "+7999",
		DeliveryAddress: "Addr",
		PickupAddress:   "Pickup",
		Items: []models.CreateOrderItemRequest{
			{Name: "Item", Quantity: 1, Price: 10},
		},
	}
	if err := h.validateCreateOrderRequest(req); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}

	req.Items[0].Quantity = 0
	if err := h.validateCreateOrderRequest(req); err == nil {
		t.Fatalf("expected error for zero quantity")
	}

	lat := 100.0
	req.Items[0].Quantity = 1
	req.PickupLat = &lat
	if err := h.validateCreateOrderRequest(req); err == nil {
		t.Fatalf("expected error for invalid pickup lat")
	}

	lon := 200.0
	lat = 0
	req.PickupLat = &lat
	req.PickupLon = &lon
	if err := h.validateCreateOrderRequest(req); err == nil {
		t.Fatalf("expected error for invalid pickup lon")
	}
}

func TestInvalidateStatsCache_NoRedis(t *testing.T) {
	h := &OrderHandler{}
	if err := h.invalidateStatsCache(nil); err != nil {
		t.Fatalf("expected nil error when redis is nil")
	}
}
