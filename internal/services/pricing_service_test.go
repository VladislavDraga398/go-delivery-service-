package services

import "testing"

func TestCalculateCost_MinFare(t *testing.T) {
	svc := NewPricingService(100, 20, 150)
	if cost := svc.CalculateCost(1); cost != 150 {
		t.Fatalf("expected min fare 150, got %.2f", cost)
	}
}

func TestCalculateCost_NegativeDistance(t *testing.T) {
	svc := NewPricingService(50, 10, 0)
	if cost := svc.CalculateCost(-5); cost != 50 {
		t.Fatalf("expected base fare for negative distance, got %.2f", cost)
	}
}
