package services

import "math"

// PricingService рассчитывает стоимость доставки по расстоянию.
type PricingService struct {
	BaseFare float64
	PerKm    float64
	MinFare  float64
}

// NewPricingService создаёт сервис с тарифами.
func NewPricingService(baseFare, perKm, minFare float64) *PricingService {
	return &PricingService{
		BaseFare: baseFare,
		PerKm:    perKm,
		MinFare:  minFare,
	}
}

// CalculateCost считает цену с учётом базовой ставки, тарифа за км и минимальной цены.
func (s *PricingService) CalculateCost(distanceKm float64) float64 {
	if distanceKm < 0 {
		distanceKm = 0
	}

	cost := s.BaseFare + (distanceKm * s.PerKm)
	if cost < s.MinFare {
		cost = s.MinFare
	}

	// Округляем до 2 знаков
	return math.Round(cost*100) / 100
}
