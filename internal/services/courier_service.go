package services

import (
	"database/sql"
	"fmt"
	"time"

	"delivery-system/internal/database"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/google/uuid"
)

// CourierService представляет сервис для работы с курьерами
type CourierService struct {
	db  *database.DB
	log *logger.Logger
}

// NewCourierService создает новый экземпляр сервиса курьеров
func NewCourierService(db *database.DB, log *logger.Logger) *CourierService {
	return &CourierService{
		db:  db,
		log: log,
	}
}

// CreateCourier создает нового курьера
func (s *CourierService) CreateCourier(req *models.CreateCourierRequest) (*models.Courier, error) {
	courier := &models.Courier{
		ID:        uuid.New(),
		Name:      req.Name,
		Phone:     req.Phone,
		Status:    models.CourierStatusOffline,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `
		INSERT INTO couriers (id, name, phone, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.Exec(query, courier.ID, courier.Name, courier.Phone,
		courier.Status, courier.CreatedAt, courier.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create courier: %w", err)
	}

	s.log.WithFields(map[string]interface{}{
		"courier_id":   courier.ID,
		"courier_name": courier.Name,
		"phone":        courier.Phone,
	}).Info("Courier created successfully")

	return courier, nil
}

// GetCourier получает курьера по ID
func (s *CourierService) GetCourier(courierID uuid.UUID) (*models.Courier, error) {
	courier := &models.Courier{}

	query := `
		SELECT id, name, phone, status, current_lat, current_lon, 
		       created_at, updated_at, last_seen_at
		FROM couriers 
		WHERE id = $1
	`

	err := s.db.QueryRow(query, courierID).Scan(
		&courier.ID, &courier.Name, &courier.Phone, &courier.Status,
		&courier.CurrentLat, &courier.CurrentLon, &courier.CreatedAt,
		&courier.UpdatedAt, &courier.LastSeenAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("courier not found")
		}
		return nil, fmt.Errorf("failed to get courier: %w", err)
	}

	return courier, nil
}

// UpdateCourierStatus обновляет статус курьера
func (s *CourierService) UpdateCourierStatus(courierID uuid.UUID, req *models.UpdateCourierStatusRequest) error {
	query := `
		UPDATE couriers 
		SET status = $1, current_lat = $2, current_lon = $3, updated_at = $4, last_seen_at = $5
		WHERE id = $6
	`

	now := time.Now()
	result, err := s.db.Exec(query, req.Status, req.CurrentLat, req.CurrentLon, now, now, courierID)
	if err != nil {
		return fmt.Errorf("failed to update courier status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("courier not found")
	}

	s.log.WithFields(map[string]interface{}{
		"courier_id": courierID,
		"new_status": req.Status,
		"lat":        req.CurrentLat,
		"lon":        req.CurrentLon,
	}).Info("Courier status updated")

	return nil
}

// GetCouriers получает список курьеров с фильтрацией
func (s *CourierService) GetCouriers(status *models.CourierStatus, limit, offset int) ([]*models.Courier, error) {
	query := `
		SELECT id, name, phone, status, current_lat, current_lon, 
		       created_at, updated_at, last_seen_at
		FROM couriers 
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
		argIndex++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get couriers: %w", err)
	}
	defer rows.Close()

	var couriers []*models.Courier
	for rows.Next() {
		courier := &models.Courier{}
		if err := rows.Scan(&courier.ID, &courier.Name, &courier.Phone, &courier.Status,
			&courier.CurrentLat, &courier.CurrentLon, &courier.CreatedAt,
			&courier.UpdatedAt, &courier.LastSeenAt); err != nil {
			return nil, fmt.Errorf("failed to scan courier: %w", err)
		}
		couriers = append(couriers, courier)
	}

	return couriers, nil
}

// GetAvailableCouriers получает список доступных курьеров
func (s *CourierService) GetAvailableCouriers() ([]*models.Courier, error) {
	status := models.CourierStatusAvailable
	return s.GetCouriers(&status, 0, 0)
}

// AssignOrderToCourier назначает заказ курьеру
func (s *CourierService) AssignOrderToCourier(orderID, courierID uuid.UUID) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Проверяем, что курьер доступен
	var courierStatus string
	courierQuery := "SELECT status FROM couriers WHERE id = $1"
	err = tx.QueryRow(courierQuery, courierID).Scan(&courierStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("courier not found")
		}
		return fmt.Errorf("failed to check courier status: %w", err)
	}

	if courierStatus != string(models.CourierStatusAvailable) {
		return fmt.Errorf("courier is not available")
	}

	// Назначаем заказ курьеру и меняем статус заказа
	orderQuery := `
		UPDATE orders 
		SET courier_id = $1, status = $2, updated_at = $3
		WHERE id = $4 AND status = $5
	`
	result, err := tx.Exec(orderQuery, courierID, models.OrderStatusAccepted, time.Now(), orderID, models.OrderStatusCreated)
	if err != nil {
		return fmt.Errorf("failed to assign order to courier: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("order not found or already assigned")
	}

	// Меняем статус курьера на "занят"
	courierUpdateQuery := `
		UPDATE couriers 
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	_, err = tx.Exec(courierUpdateQuery, models.CourierStatusBusy, time.Now(), courierID)
	if err != nil {
		return fmt.Errorf("failed to update courier status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.WithFields(map[string]interface{}{
		"order_id":   orderID,
		"courier_id": courierID,
	}).Info("Order assigned to courier successfully")

	return nil
}
