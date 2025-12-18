package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"delivery-system/internal/apperror"
	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"
)

type stubPromoService struct {
	promo *models.PromoCode
	err   error
	list  []*models.PromoCode
}

func (s *stubPromoService) CreatePromoCode(ctx context.Context, req *models.CreatePromoCodeRequest) (*models.PromoCode, error) {
	return s.promo, s.err
}
func (s *stubPromoService) GetPromoCode(ctx context.Context, code string) (*models.PromoCode, error) {
	return s.promo, s.err
}
func (s *stubPromoService) UpdatePromoCode(ctx context.Context, code string, req *models.UpdatePromoCodeRequest) (*models.PromoCode, error) {
	return s.promo, s.err
}
func (s *stubPromoService) DeletePromoCode(ctx context.Context, code string) error {
	return s.err
}
func (s *stubPromoService) ListPromoCodes(ctx context.Context, limit, offset int) ([]*models.PromoCode, error) {
	return s.list, s.err
}

func TestPromoHandler_CreateAndGet(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	p := &models.PromoCode{
		Code:         "TEST",
		DiscountType: models.DiscountTypeFixed,
		Amount:       10,
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	handler := NewPromoHandler(&stubPromoService{promo: p}, log)

	body := bytes.NewBufferString(`{"code":"TEST","discount_type":"fixed","amount":10,"active":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes", body)
	rr := httptest.NewRecorder()
	handler.CreatePromoCode(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/api/promo-codes/TEST", nil)
	rrGet := httptest.NewRecorder()
	handler.GetPromoCode(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrGet.Code)
	}
}

func TestPromoHandler_CreatePromoCode_InvalidBody(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes", bytes.NewBufferString("bad json"))
	rr := httptest.NewRecorder()
	handler.CreatePromoCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPromoHandler_CreatePromoCode_EmptyCode(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes", bytes.NewBufferString(`{"code":"","discount_type":"fixed","amount":10}`))
	rr := httptest.NewRecorder()
	handler.CreatePromoCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPromoHandler_CreatePromoCode_CodeTooLong(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{}, log)

	longCode := make([]byte, 65)
	for i := range longCode {
		longCode[i] = 'A'
	}
	body := bytes.NewBufferString(`{"code":"` + string(longCode) + `","discount_type":"fixed","amount":10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes", body)
	rr := httptest.NewRecorder()
	handler.CreatePromoCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPromoHandler_CreatePromoCode_ServiceValidationError(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	service := &stubPromoService{err: apperror.Validation("amount must be non-negative for fixed discount", nil)}
	handler := NewPromoHandler(service, log)

	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes", bytes.NewBufferString(`{"code":"X","discount_type":"fixed","amount":-1}`))
	rr := httptest.NewRecorder()
	handler.CreatePromoCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for validation error, got %d", rr.Code)
	}
}

func TestPromoHandler_GetPromoCode_InvalidPath(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{}, log)

	req := httptest.NewRequest(http.MethodGet, "/api/invalid-prefix/X", nil)
	rr := httptest.NewRecorder()
	handler.GetPromoCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPromoHandler_List(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{list: []*models.PromoCode{}}, log)

	req := httptest.NewRequest(http.MethodGet, "/api/promo-codes", nil)
	rr := httptest.NewRecorder()
	handler.ListPromoCodes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestPromoHandler_List_MethodNotAllowed(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes", nil)
	rr := httptest.NewRecorder()
	handler.ListPromoCodes(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestPromoHandler_UpdateAndDelete(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	updated := &models.PromoCode{Code: "TEST", DiscountType: models.DiscountTypePercent, Amount: 20}
	service := &stubPromoService{promo: updated}
	handler := NewPromoHandler(service, log)

	req := httptest.NewRequest(http.MethodPut, "/api/promo-codes/TEST", bytes.NewBufferString(`{"discount_type":"percent","amount":20,"active":true}`))
	rr := httptest.NewRecorder()
	handler.UpdatePromoCode(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	reqDel := httptest.NewRequest(http.MethodDelete, "/api/promo-codes/TEST", nil)
	rrDel := httptest.NewRecorder()
	handler.DeletePromoCode(rrDel, reqDel)
	if rrDel.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrDel.Code)
	}
}

func TestPromoHandler_MethodNotAllowed(t *testing.T) {
	handler := NewPromoHandler(&stubPromoService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}))

	req := httptest.NewRequest(http.MethodPut, "/api/promo-codes", nil)
	rr := httptest.NewRecorder()
	handler.CreatePromoCode(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestPromoHandler_ServiceErrors(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	service := &stubPromoService{err: errors.New("fail")}
	handler := NewPromoHandler(service, log)

	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes", bytes.NewBufferString(`{"code":"X","discount_type":"fixed","amount":10}`))
	rr := httptest.NewRecorder()
	handler.CreatePromoCode(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/promo-codes/X", nil)
	rr = httptest.NewRecorder()
	handler.GetPromoCode(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/promo-codes/X", nil)
	rr = httptest.NewRecorder()
	handler.DeletePromoCode(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/promo-codes", nil)
	rr = httptest.NewRecorder()
	handler.ListPromoCodes(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestPromoHandler_NotFound(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	service := &stubPromoService{err: apperror.NotFound("promo code not found", nil)}
	handler := NewPromoHandler(service, log)
	req := httptest.NewRequest(http.MethodGet, "/api/promo-codes/ABSENT", nil)
	rr := httptest.NewRecorder()
	handler.GetPromoCode(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/promo-codes/ABSENT", bytes.NewBufferString(`{"discount_type":"percent","amount":10}`))
	rr = httptest.NewRecorder()
	handler.UpdatePromoCode(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/promo-codes/ABSENT", nil)
	rr = httptest.NewRecorder()
	handler.DeletePromoCode(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestPromoHandler_UpdatePromoCode_InvalidPayload(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/promo-codes/TEST", bytes.NewBufferString(`{"discount_type":"percent","amount":150}`))
	rr := httptest.NewRecorder()
	handler.UpdatePromoCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for validation error, got %d", rr.Code)
	}
}

func TestPromoHandler_UpdatePromoCode_InvalidServiceError(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewPromoHandler(&stubPromoService{err: apperror.Validation("invalid range", nil)}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/promo-codes/TEST", bytes.NewBufferString(`{"discount_type":"percent","amount":10}`))
	rr := httptest.NewRecorder()
	handler.UpdatePromoCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid data, got %d", rr.Code)
	}
}

func TestPromoHandler_UpdatePromoCode_MethodNotAllowed(t *testing.T) {
	handler := NewPromoHandler(&stubPromoService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}))
	req := httptest.NewRequest(http.MethodGet, "/api/promo-codes/TEST", nil)
	rr := httptest.NewRecorder()
	handler.UpdatePromoCode(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestPromoHandler_Delete_MethodNotAllowed(t *testing.T) {
	handler := NewPromoHandler(&stubPromoService{}, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}))
	req := httptest.NewRequest(http.MethodPost, "/api/promo-codes/TEST", nil)
	rr := httptest.NewRecorder()
	handler.DeletePromoCode(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
