package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"delivery-system/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPromoService_ApplyPercent(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewPromoService(db, log)

	code := "SALE10"

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT discount_type, amount, max_uses, used_count, expires_at, active FROM promo_codes").
		WithArgs(code).
		WillReturnRows(sqlmock.NewRows([]string{"discount_type", "amount", "max_uses", "used_count", "expires_at", "active"}).
			AddRow(models.DiscountTypePercent, 10.0, 5, 1, nil, true))

	mock.ExpectExec("UPDATE promo_codes").
		WithArgs(sqlmock.AnyArg(), code).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}

	discount, err := service.ApplyPromoWithTx(context.Background(), tx, code, 200, 50)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	if discount != 25.0 {
		t.Fatalf("expected discount 25.0, got %.2f", discount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoService_ApplyFreeDelivery(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewPromoService(db, log)

	code := "FREEDEL"

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT discount_type, amount, max_uses, used_count, expires_at, active FROM promo_codes").
		WithArgs(code).
		WillReturnRows(sqlmock.NewRows([]string{"discount_type", "amount", "max_uses", "used_count", "expires_at", "active"}).
			AddRow(models.DiscountTypeFreeDelivery, 0.0, 0, 0, nil, true))

	mock.ExpectExec("UPDATE promo_codes").
		WithArgs(sqlmock.AnyArg(), code).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, _ := db.Begin()
	discount, err := service.ApplyPromoWithTx(context.Background(), tx, code, 120, 80)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	_ = tx.Commit()

	if discount != 80 {
		t.Fatalf("expected discount 80, got %.2f", discount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoService_ApplyPromo_Expired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewPromoService(db, log)

	code := "OLD"
	expired := time.Now().Add(-time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT discount_type, amount, max_uses, used_count, expires_at, active FROM promo_codes").
		WithArgs(code).
		WillReturnRows(sqlmock.NewRows([]string{"discount_type", "amount", "max_uses", "used_count", "expires_at", "active"}).
			AddRow(models.DiscountTypeFixed, 50.0, 0, 0, expired, true))
	// Expect rollback due to error
	mock.ExpectRollback()

	tx, _ := db.Begin()
	if _, err := service.ApplyPromoWithTx(context.Background(), tx, code, 100, 20); err == nil {
		t.Fatalf("expected error for expired promo")
	}
	_ = tx.Rollback()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoService_ApplyPromo_LimitReached(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	log := newTestLogger()
	service := NewPromoService(db, log)

	code := "USED"

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT discount_type, amount, max_uses, used_count, expires_at, active FROM promo_codes").
		WithArgs(code).
		WillReturnRows(sqlmock.NewRows([]string{"discount_type", "amount", "max_uses", "used_count", "expires_at", "active"}).
			AddRow(models.DiscountTypeFixed, 50.0, 1, 1, nil, true))
	mock.ExpectRollback()

	tx, _ := db.Begin()
	if _, err := service.ApplyPromoWithTx(context.Background(), tx, code, 100, 20); err == nil {
		t.Fatalf("expected error for usage limit")
	}
	_ = tx.Rollback()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoService_CreateUpdateDeleteAndList(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	log := newTestLogger()
	service := NewPromoService(db, log)

	mock.ExpectExec("INSERT INTO promo_codes").WillReturnResult(sqlmock.NewResult(1, 1))
	promo, err := service.CreatePromoCode(context.Background(), &models.CreatePromoCodeRequest{
		Code:         "NEW",
		DiscountType: models.DiscountTypePercent,
		Amount:       10,
		MaxUses:      5,
		ExpiresAt:    nil,
		Active:       true,
	})
	if err != nil || promo.Code != "NEW" {
		t.Fatalf("create failed: %v", err)
	}

	mock.ExpectExec("UPDATE promo_codes").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expAt := time.Now()
	mock.ExpectQuery("SELECT code, discount_type").
		WithArgs("NEW").
		WillReturnRows(sqlmock.NewRows([]string{"code", "discount_type", "amount", "max_uses", "used_count", "expires_at", "active", "created_at", "updated_at"}).
			AddRow("NEW", models.DiscountTypePercent, 15.0, 10, 0, &expAt, true, time.Now(), time.Now()))

	updated, err := service.UpdatePromoCode(context.Background(), "NEW", &models.UpdatePromoCodeRequest{
		DiscountType: models.DiscountTypePercent,
		Amount:       15,
		MaxUses:      10,
		ExpiresAt:    &expAt,
		Active:       true,
	})
	if err != nil || updated.Amount != 15 {
		t.Fatalf("update failed: %v", err)
	}

	mock.ExpectExec("DELETE FROM promo_codes").
		WithArgs("NEW").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := service.DeletePromoCode(context.Background(), "NEW"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	mock.ExpectQuery("SELECT code, discount_type").
		WillReturnRows(sqlmock.NewRows([]string{"code", "discount_type", "amount", "max_uses", "used_count", "expires_at", "active", "created_at", "updated_at"}).
			AddRow("A", models.DiscountTypeFixed, 5.0, 0, 0, time.Now(), true, time.Now(), time.Now()).
			AddRow("B", models.DiscountTypePercent, 10.0, 0, 0, time.Now(), true, time.Now(), time.Now()))
	list, err := service.ListPromoCodes(context.Background(), 0, 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("list failed: %v len=%d", err, len(list))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoService_CreatePromoCode_InvalidPayload(t *testing.T) {
	db, _ := newMockDB(t)
	defer db.Close()

	service := NewPromoService(db, newTestLogger())
	if _, err := service.CreatePromoCode(context.Background(), &models.CreatePromoCodeRequest{
		Code:         "BAD",
		DiscountType: models.DiscountTypePercent,
		Amount:       150,
	}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestPromoService_UpdatePromoCode_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	service := NewPromoService(db, newTestLogger())

	mock.ExpectExec("UPDATE promo_codes").
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := service.UpdatePromoCode(context.Background(), "MISS", &models.UpdatePromoCodeRequest{
		DiscountType: models.DiscountTypeFixed,
		Amount:       10,
		Active:       true,
	})
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestPromoService_UpdatePromoCode_RowsAffectedError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	service := NewPromoService(db, newTestLogger())

	mock.ExpectExec("UPDATE promo_codes").
		WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected error")))

	if _, err := service.UpdatePromoCode(context.Background(), "X", &models.UpdatePromoCodeRequest{
		DiscountType: models.DiscountTypeFixed,
		Amount:       10,
		Active:       true,
	}); err == nil {
		t.Fatalf("expected rows affected error")
	}
}

func TestPromoService_DeletePromoCode_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	service := NewPromoService(db, newTestLogger())

	mock.ExpectExec("DELETE FROM promo_codes").
		WithArgs("MISS").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := service.DeletePromoCode(context.Background(), "MISS"); err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestPromoService_DeletePromoCode_RowsAffectedError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	service := NewPromoService(db, newTestLogger())

	mock.ExpectExec("DELETE FROM promo_codes").
		WithArgs("X").
		WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected error")))

	if err := service.DeletePromoCode(context.Background(), "X"); err == nil {
		t.Fatalf("expected rows affected error")
	}
}

func TestPromoService_GetPromoCode_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	log := newTestLogger()
	service := NewPromoService(db, log)

	mock.ExpectQuery("SELECT code, discount_type").
		WithArgs("MISS").
		WillReturnError(sql.ErrNoRows)

	if _, err := service.GetPromoCode(context.Background(), "MISS"); err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestValidatePromoCodePayload(t *testing.T) {
	if err := validatePromoCodePayload(models.DiscountTypeFixed, -1); err == nil {
		t.Fatalf("expected error for negative amount")
	}
	if err := validatePromoCodePayload("unknown", 10); err == nil {
		t.Fatalf("expected error for invalid type")
	}
	if err := validatePromoCodePayload(models.DiscountTypePercent, 150); err == nil {
		t.Fatalf("expected error for >100 percent")
	}
	if err := validatePromoCodePayload(models.DiscountTypePercent, 50); err != nil {
		t.Fatalf("expected valid percent, got %v", err)
	}
}

func TestCalculateDiscount(t *testing.T) {
	if v := calculateDiscount(models.DiscountTypeFixed, 10, 100, 20); v != 10 {
		t.Fatalf("expected fixed 10, got %v", v)
	}
	if v := calculateDiscount(models.DiscountTypePercent, 10, 200, 0); v != 20 {
		t.Fatalf("expected percent 20, got %v", v)
	}
	if v := calculateDiscount(models.DiscountTypeFreeDelivery, 0, 100, 30); v != 30 {
		t.Fatalf("expected free delivery 30, got %v", v)
	}
}

func TestCalculateDiscount_EdgeCases(t *testing.T) {
	if v := calculateDiscount(models.DiscountTypeFixed, -5, 100, 10); v != 0 {
		t.Fatalf("expected zero for negative fixed discount, got %v", v)
	}
	if v := calculateDiscount(models.DiscountTypePercent, 150, 100, 0); v != 100 {
		t.Fatalf("expected capped percent discount, got %v", v)
	}
	if v := calculateDiscount(models.DiscountType("unknown"), 10, 100, 10); v != 0 {
		t.Fatalf("expected zero for unknown type, got %v", v)
	}
}
