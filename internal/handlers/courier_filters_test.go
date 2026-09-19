package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseCourierFilters(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/couriers/available?min_rating=4.5&order_by=rating", nil)
	status, minRating, orderBy, err := parseCourierFilters(req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status, got %v", *status)
	}
	if minRating == nil || *minRating != 4.5 {
		t.Fatalf("expected min_rating 4.5")
	}
	if orderBy != "rating" {
		t.Fatalf("expected order_by rating, got %s", orderBy)
	}
}

func TestParseCourierFilters_WithStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/couriers?status=busy", nil)
	status, _, _, err := parseCourierFilters(req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if status == nil || *status != "busy" {
		t.Fatalf("expected status busy")
	}
}

func TestParseCourierFilters_InvalidRating(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/couriers/available?min_rating=9", nil)
	if _, _, _, err := parseCourierFilters(req); err == nil {
		t.Fatalf("expected error for min_rating > 5")
	}
}

func TestParseCourierFilters_InvalidOrderBy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/couriers?order_by=price", nil)
	if _, _, _, err := parseCourierFilters(req); err == nil {
		t.Fatalf("expected error for invalid order_by")
	}
}
