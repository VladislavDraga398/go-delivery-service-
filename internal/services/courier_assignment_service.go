package services

import (
	"fmt"
	"math"

	"delivery-system/internal/database"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/google/uuid"
)

// CourierAssignmentService представляет сервис автоназначения курьеров
type CourierAssignmentService struct {
	db             *database.DB
	courierService *CourierService
	orderService   *OrderService
	log            *logger.Logger
}

// NewCourierAssignmentService создает новый экземпляр сервиса автоназначения
func NewCourierAssignmentService(db *database.DB, courierService *CourierService, orderService *OrderService, log *logger.Logger) *CourierAssignmentService {
	return &CourierAssignmentService{
		db:             db,
		courierService: courierService,
		orderService:   orderService,
		log:            log,
	}
}

// CourierScore представляет оценку курьера для назначения
type CourierScore struct {
	CourierID     uuid.UUID
	CourierName   string
	DistanceScore float64 // 0-1, где 1 = лучший (ближайший)
	RatingScore   float64 // 0-1, где 1 = лучший (рейтинг 5)
	WorkloadScore float64 // 0-1, где 1 = лучший (нет активных заказов)
	TotalScore    float64 // взвешенная сумма
	Distance      float64 // расстояние в км
	Rating        float64
	ActiveOrders  int
}

// AssignmentWeights представляет веса для алгоритма назначения
type AssignmentWeights struct {
	Distance float64 // по умолчанию 0.40
	Rating   float64 // по умолчанию 0.30
	Workload float64 // по умолчанию 0.30
}

// DefaultWeights возвращает стандартные веса
func DefaultWeights() AssignmentWeights {
	return AssignmentWeights{
		Distance: 0.40,
		Rating:   0.30,
		Workload: 0.30,
	}
}

// AutoAssignCourier автоматически выбирает и назначает оптимального курьера на заказ
func (s *CourierAssignmentService) AutoAssignCourier(orderID uuid.UUID, deliveryLat, deliveryLon float64) (*models.Courier, error) {
	// Получаем заказ
	order, err := s.orderService.GetOrder(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if order.Status != models.OrderStatusCreated {
		return nil, fmt.Errorf("order is not in 'created' status")
	}

	if order.CourierID != nil {
		return nil, fmt.Errorf("order already has assigned courier")
	}

	// Получаем доступных курьеров
	availableCouriers, err := s.courierService.GetAvailableCouriers()
	if err != nil {
		return nil, fmt.Errorf("failed to get available couriers: %w", err)
	}

	if len(availableCouriers) == 0 {
		return nil, fmt.Errorf("no available couriers found")
	}

	// Фильтруем курьеров с координатами
	var couriersWithLocation []*models.Courier
	for _, c := range availableCouriers {
		if c.CurrentLat != nil && c.CurrentLon != nil {
			couriersWithLocation = append(couriersWithLocation, c)
		}
	}

	if len(couriersWithLocation) == 0 {
		return nil, fmt.Errorf("no couriers with known location available")
	}

	// Рассчитываем оценки для каждого курьера
	weights := DefaultWeights()
	scores := make([]CourierScore, 0, len(couriersWithLocation))

	for _, courier := range couriersWithLocation {
		score := s.calculateCourierScore(courier, deliveryLat, deliveryLon, weights)
		scores = append(scores, score)
	}

	// Находим курьера с максимальной оценкой
	bestScore := scores[0]
	for _, score := range scores[1:] {
		if score.TotalScore > bestScore.TotalScore {
			bestScore = score
		}
	}

	// Назначаем заказ лучшему курьеру
	err = s.courierService.AssignOrderToCourier(orderID, bestScore.CourierID)
	if err != nil {
		return nil, fmt.Errorf("failed to assign order to courier: %w", err)
	}

	// Логируем причину выбора
	s.log.WithFields(map[string]interface{}{
		"order_id":       orderID,
		"courier_id":     bestScore.CourierID,
		"courier_name":   bestScore.CourierName,
		"total_score":    bestScore.TotalScore,
		"distance_score": bestScore.DistanceScore,
		"rating_score":   bestScore.RatingScore,
		"workload_score": bestScore.WorkloadScore,
		"distance_km":    bestScore.Distance,
		"rating":         bestScore.Rating,
		"active_orders":  bestScore.ActiveOrders,
	}).Info("Courier auto-assigned based on scoring algorithm")

	// Возвращаем назначенного курьера
	return s.courierService.GetCourier(bestScore.CourierID)
}

// calculateCourierScore рассчитывает общую оценку курьера для назначения
func (s *CourierAssignmentService) calculateCourierScore(courier *models.Courier, targetLat, targetLon float64, weights AssignmentWeights) CourierScore {
	score := CourierScore{
		CourierID:   courier.ID,
		CourierName: courier.Name,
		Rating:      courier.Rating,
	}

	// Расчёт расстояния от курьера до точки доставки
	distance := calculateDistance(*courier.CurrentLat, *courier.CurrentLon, targetLat, targetLon)
	score.Distance = distance

	// Distance Score: чем ближе, тем лучше (используем обратную функцию)
	// Максимальное расстояние для нормализации: 50 км
	maxDistance := 50.0
	if distance > maxDistance {
		score.DistanceScore = 0.0
	} else {
		score.DistanceScore = 1.0 - (distance / maxDistance)
	}

	// Rating Score: нормализуем рейтинг от 0 до 5 в диапазон 0-1
	score.RatingScore = courier.Rating / 5.0

	// Workload Score: получаем количество активных заказов у курьера
	activeOrders := s.getActiveCourierOrders(courier.ID)
	score.ActiveOrders = activeOrders

	// Чем меньше заказов, тем лучше (используем обратную функцию)
	// Максимальное количество заказов для нормализации: 5
	maxOrders := 5.0
	if float64(activeOrders) >= maxOrders {
		score.WorkloadScore = 0.0
	} else {
		score.WorkloadScore = 1.0 - (float64(activeOrders) / maxOrders)
	}

	// Рассчитываем взвешенную сумму
	score.TotalScore = (score.DistanceScore * weights.Distance) +
		(score.RatingScore * weights.Rating) +
		(score.WorkloadScore * weights.Workload)

	return score
}

// getActiveCourierOrders возвращает количество активных заказов у курьера
func (s *CourierAssignmentService) getActiveCourierOrders(courierID uuid.UUID) int {
	query := `
		SELECT COUNT(*) 
		FROM orders 
		WHERE courier_id = $1 
		  AND status IN ('accepted', 'preparing', 'ready', 'in_delivery')
	`

	var count int
	err := s.db.QueryRow(query, courierID).Scan(&count)
	if err != nil {
		s.log.WithError(err).WithField("courier_id", courierID).Warn("Failed to get active orders count, assuming 0")
		return 0
	}

	return count
}

// calculateDistance вычисляет расстояние между двумя точками по формуле гаверсинуса (в км)
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	// Преобразуем градусы в радианы
	lat1Rad := lat1 * math.Pi / 180.0
	lon1Rad := lon1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	lon2Rad := lon2 * math.Pi / 180.0

	// Разница координат
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	// Формула гаверсинуса
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}
