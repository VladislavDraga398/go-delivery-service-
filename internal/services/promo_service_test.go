package services

import (
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

	discount, err := service.ApplyPromoWithTx(tx, code, 200, 50)
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
	discount, err := service.ApplyPromoWithTx(tx, code, 120, 80)
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
	if _, err := service.ApplyPromoWithTx(tx, code, 100, 20); err == nil {
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
	if _, err := service.ApplyPromoWithTx(tx, code, 100, 20); err == nil {
		t.Fatalf("expected error for usage limit")
	}
	_ = tx.Rollback()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
