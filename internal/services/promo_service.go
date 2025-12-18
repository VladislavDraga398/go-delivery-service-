package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"delivery-system/internal/apperror"
	"delivery-system/internal/database"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/lib/pq"
)

// PromoService управляет промокодами и расчётом скидок.
type PromoService struct {
	db  *database.DB
	log *logger.Logger
}

// NewPromoService создаёт сервис промокодов.
func NewPromoService(db *database.DB, log *logger.Logger) *PromoService {
	return &PromoService{
		db:  db,
		log: log,
	}
}

// CreatePromoCode создаёт новый промокод.
func (s *PromoService) CreatePromoCode(ctx context.Context, req *models.CreatePromoCodeRequest) (*models.PromoCode, error) {
	if err := validatePromoCodePayload(req.DiscountType, req.Amount); err != nil {
		return nil, apperror.Validation(err.Error(), err)
	}

	promo := &models.PromoCode{
		Code:         req.Code,
		DiscountType: req.DiscountType,
		Amount:       req.Amount,
		MaxUses:      req.MaxUses,
		ExpiresAt:    req.ExpiresAt,
		Active:       req.Active,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	query := `
		INSERT INTO promo_codes (code, discount_type, amount, max_uses, used_count, expires_at, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 0, $5, $6, $7, $8)
	`

	_, err := s.db.ExecContext(ctx, query, promo.Code, promo.DiscountType, promo.Amount, promo.MaxUses, promo.ExpiresAt, promo.Active, promo.CreatedAt, promo.UpdatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, apperror.Conflict("promo code already exists", err)
		}
		return nil, fmt.Errorf("failed to create promo code: %w", err)
	}

	s.log.WithField("promo_code", promo.Code).Info("Promo code created")
	return promo, nil
}

// UpdatePromoCode обновляет параметры промокода.
func (s *PromoService) UpdatePromoCode(ctx context.Context, code string, req *models.UpdatePromoCodeRequest) (*models.PromoCode, error) {
	if err := validatePromoCodePayload(req.DiscountType, req.Amount); err != nil {
		return nil, apperror.Validation(err.Error(), err)
	}

	query := `
		UPDATE promo_codes
		SET discount_type = $1, amount = $2, max_uses = $3, expires_at = $4, active = $5, updated_at = $6
		WHERE code = $7
	`

	result, err := s.db.ExecContext(ctx, query, req.DiscountType, req.Amount, req.MaxUses, req.ExpiresAt, req.Active, time.Now(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to update promo code: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return nil, apperror.NotFound("promo code not found", nil)
	}

	return s.GetPromoCode(ctx, code)
}

// DeletePromoCode удаляет промокод.
func (s *PromoService) DeletePromoCode(ctx context.Context, code string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM promo_codes WHERE code = $1", code)
	if err != nil {
		return fmt.Errorf("failed to delete promo code: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return apperror.NotFound("promo code not found", nil)
	}
	return nil
}

// GetPromoCode возвращает промокод по коду.
func (s *PromoService) GetPromoCode(ctx context.Context, code string) (*models.PromoCode, error) {
	query := `
		SELECT code, discount_type, amount, max_uses, used_count, expires_at, active, created_at, updated_at
		FROM promo_codes
		WHERE code = $1
	`

	promo := &models.PromoCode{}
	if err := s.db.QueryRowContext(ctx, query, code).Scan(
		&promo.Code, &promo.DiscountType, &promo.Amount, &promo.MaxUses, &promo.UsedCount,
		&promo.ExpiresAt, &promo.Active, &promo.CreatedAt, &promo.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperror.NotFound("promo code not found", err)
		}
		return nil, fmt.Errorf("failed to get promo code: %w", err)
	}
	return promo, nil
}

// ListPromoCodes возвращает список промокодов.
func (s *PromoService) ListPromoCodes(ctx context.Context, limit, offset int) ([]*models.PromoCode, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT code, discount_type, amount, max_uses, used_count, expires_at, active, created_at, updated_at
		FROM promo_codes
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list promo codes: %w", err)
	}
	defer rows.Close()

	var promos []*models.PromoCode
	for rows.Next() {
		p := &models.PromoCode{}
		if err := rows.Scan(&p.Code, &p.DiscountType, &p.Amount, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan promo code: %w", err)
		}
		promos = append(promos, p)
	}

	return promos, nil
}

// ApplyPromoWithTx рассчитывает скидку и увеличивает счётчик использования в рамках транзакции.
func (s *PromoService) ApplyPromoWithTx(ctx context.Context, tx *sql.Tx, code string, itemsTotal, deliveryCost float64) (float64, error) {
	query := `
		SELECT discount_type, amount, max_uses, used_count, expires_at, active
		FROM promo_codes
		WHERE code = $1
		FOR UPDATE
	`

	var (
		discountType models.DiscountType
		amount       float64
		maxUses      int
		usedCount    int
		expiresAt    *time.Time
		active       bool
	)

	if err := tx.QueryRowContext(ctx, query, code).Scan(&discountType, &amount, &maxUses, &usedCount, &expiresAt, &active); err != nil {
		if err == sql.ErrNoRows {
			return 0, apperror.NotFound("promo code not found", err)
		}
		return 0, fmt.Errorf("failed to get promo code: %w", err)
	}

	if !active {
		return 0, apperror.Conflict("promo code is inactive", nil)
	}

	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return 0, apperror.Conflict("promo code expired", nil)
	}

	if maxUses > 0 && usedCount >= maxUses {
		return 0, apperror.Conflict("promo code usage limit reached", nil)
	}

	totalBase := itemsTotal + deliveryCost
	discount := calculateDiscount(discountType, amount, totalBase, deliveryCost)

	updateQuery := `
		UPDATE promo_codes
		SET used_count = used_count + 1, updated_at = $1
		WHERE code = $2
	`
	if _, err := tx.ExecContext(ctx, updateQuery, time.Now(), code); err != nil {
		return 0, fmt.Errorf("failed to update promo usage: %w", err)
	}

	return discount, nil
}

func calculateDiscount(discountType models.DiscountType, amount, baseTotal, deliveryCost float64) float64 {
	switch discountType {
	case models.DiscountTypeFixed:
		if amount < 0 {
			return 0
		}
		if amount > baseTotal {
			return baseTotal
		}
		return round2(amount)
	case models.DiscountTypePercent:
		if amount <= 0 {
			return 0
		}
		if amount > 100 {
			amount = 100
		}
		return round2(baseTotal * amount / 100.0)
	case models.DiscountTypeFreeDelivery:
		if deliveryCost < 0 {
			return 0
		}
		return round2(deliveryCost)
	default:
		return 0
	}
}

func validatePromoCodePayload(discountType models.DiscountType, amount float64) error {
	switch discountType {
	case models.DiscountTypeFixed:
		if amount < 0 {
			return fmt.Errorf("amount must be non-negative for fixed discount")
		}
	case models.DiscountTypePercent:
		if amount <= 0 || amount > 100 {
			return fmt.Errorf("percent amount must be between 0 and 100")
		}
	case models.DiscountTypeFreeDelivery:
		// amount is ignored
	default:
		return fmt.Errorf("invalid discount_type")
	}
	return nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
