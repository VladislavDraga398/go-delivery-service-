package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"delivery-system/internal/apperror"
	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/google/uuid"
)

type stubCourierService struct {
	courier *models.Courier
	list    []*models.Courier
	err     error
	getErr  error
	updErr  error
}

func (s *stubCourierService) CreateCourier(ctx context.Context, req *models.CreateCourierRequest) (*models.Courier, error) {
	return s.courier, s.err
}
func (s *stubCourierService) GetCourier(ctx context.Context, courierID uuid.UUID) (*models.Courier, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.courier, s.err
}
func (s *stubCourierService) UpdateCourierStatus(ctx context.Context, courierID uuid.UUID, req *models.UpdateCourierStatusRequest) error {
	if s.updErr != nil {
		return s.updErr
	}
	return s.err
}
func (s *stubCourierService) GetCouriers(ctx context.Context, status *models.CourierStatus, minRating *float64, limit, offset int, orderBy string) ([]*models.Courier, error) {
	return s.list, s.err
}
func (s *stubCourierService) GetAvailableCouriers(ctx context.Context) ([]*models.Courier, error) {
	return s.list, s.err
}
func (s *stubCourierService) AssignOrderToCourier(ctx context.Context, orderID, courierID uuid.UUID) error {
	return s.err
}

type stubOrderSvc struct {
	order   *models.Order
	reviews []*models.Review
	err     error
}

func (s *stubOrderSvc) CreateOrder(ctx context.Context, req *models.CreateOrderRequest) (*models.Order, error) {
	return s.order, s.err
}
func (s *stubOrderSvc) GetOrder(ctx context.Context, orderID uuid.UUID) (*models.Order, error) {
	return s.order, s.err
}
func (s *stubOrderSvc) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, req *models.UpdateOrderStatusRequest) error {
	return s.err
}
func (s *stubOrderSvc) GetOrders(ctx context.Context, status *models.OrderStatus, courierID *uuid.UUID, limit, offset int) ([]*models.Order, error) {
	return []*models.Order{s.order}, s.err
}
func (s *stubOrderSvc) CreateReview(ctx context.Context, orderID uuid.UUID, req *models.CreateReviewRequest) (*models.Review, error) {
	return nil, s.err
}
func (s *stubOrderSvc) GetCourierReviews(ctx context.Context, courierID uuid.UUID, limit, offset int) ([]*models.Review, error) {
	if s.reviews != nil {
		return s.reviews, s.err
	}
	return []*models.Review{}, s.err
}

type stubProducerCourier struct{}

func (s *stubProducerCourier) PublishOrderCreated(order *models.Order) error { return nil }
func (s *stubProducerCourier) PublishOrderStatusChanged(orderID uuid.UUID, oldStatus, newStatus models.OrderStatus, courierID *uuid.UUID) error {
	return nil
}
func (s *stubProducerCourier) PublishCourierStatusChanged(courierID uuid.UUID, oldStatus, newStatus models.CourierStatus) error {
	return nil
}
func (s *stubProducerCourier) PublishLocationUpdated(courierID uuid.UUID, lat, lon float64) error {
	return nil
}
func (s *stubProducerCourier) PublishCourierAssigned(orderID, courierID uuid.UUID) error { return nil }

type recordingProducerCourier struct {
	statusChangedCalls int
	locationCalls      int

	statusChangedErr error
	locationErr      error
}

func (p *recordingProducerCourier) PublishOrderCreated(order *models.Order) error { return nil }
func (p *recordingProducerCourier) PublishOrderStatusChanged(orderID uuid.UUID, oldStatus, newStatus models.OrderStatus, courierID *uuid.UUID) error {
	return nil
}
func (p *recordingProducerCourier) PublishCourierStatusChanged(courierID uuid.UUID, oldStatus, newStatus models.CourierStatus) error {
	p.statusChangedCalls++
	return p.statusChangedErr
}
func (p *recordingProducerCourier) PublishLocationUpdated(courierID uuid.UUID, lat, lon float64) error {
	p.locationCalls++
	return p.locationErr
}
func (p *recordingProducerCourier) PublishCourierAssigned(orderID, courierID uuid.UUID) error {
	return nil
}

type stubRedisMiss struct{}

func (s *stubRedisMiss) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}
func (s *stubRedisMiss) Get(ctx context.Context, key string, dest interface{}) error {
	return fmt.Errorf("miss")
}
func (s *stubRedisMiss) Delete(ctx context.Context, key string) error            { return nil }
func (s *stubRedisMiss) DeleteByPrefix(ctx context.Context, prefix string) error { return nil }

func newCourierHandler() *CourierHandler {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	courier := &models.Courier{ID: uuid.New(), Name: "C"}
	order := &models.Order{ID: uuid.New()}
	return NewCourierHandler(&stubCourierService{courier: courier, list: []*models.Courier{courier}}, &stubOrderSvc{order: order, reviews: []*models.Review{{ID: uuid.New(), Rating: 5}}}, &stubProducerCourier{}, &stubRedis{}, log)
}

func TestCourierHandler_CreateCourier(t *testing.T) {
	h := newCourierHandler()
	body := bytes.NewBufferString(`{"name":"Test","phone":"+7999"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/couriers", body)
	rr := httptest.NewRecorder()
	h.CreateCourier(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestCourierHandler_CreateCourier_Validation(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/couriers", bytes.NewBufferString(`{"name":""}`))
	rr := httptest.NewRecorder()
	h.CreateCourier(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCourierHandler_GetCourier(t *testing.T) {
	h := newCourierHandler()
	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/couriers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	h.GetCourier(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCourierHandler_GetCourier_InvalidID(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/couriers/not-uuid", nil)
	rr := httptest.NewRecorder()
	h.GetCourier(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCourierHandler_ErrorPaths(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	errSvc := &stubCourierService{err: fmt.Errorf("fail")}
	handler := NewCourierHandler(errSvc, &stubOrderSvc{}, &stubProducerCourier{}, &stubRedisMiss{}, log)

	req := httptest.NewRequest(http.MethodGet, "/api/couriers/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	handler.GetCourier(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/couriers", nil)
	rr = httptest.NewRecorder()
	handler.GetCouriers(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestCourierHandler_UpdateStatus(t *testing.T) {
	h := newCourierHandler()
	id := uuid.New()
	body := bytes.NewBufferString(`{"status":"available"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/couriers/"+id.String()+"/status", body)
	rr := httptest.NewRecorder()
	h.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCourierHandler_UpdateStatus_InvalidID(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodPut, "/api/couriers/not-uuid/status", bytes.NewBufferString(`{"status":"available"}`))
	rr := httptest.NewRecorder()
	h.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCourierHandler_UpdateStatus_WithLocation(t *testing.T) {
	h := newCourierHandler()
	id := uuid.New()
	body := bytes.NewBufferString(`{"status":"available","current_lat":55,"current_lon":37}`)
	req := httptest.NewRequest(http.MethodPut, "/api/couriers/"+id.String()+"/status", body)
	rr := httptest.NewRecorder()
	h.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCourierHandler_UpdateStatus_ProducerErrorsAreIgnored(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	id := uuid.New()

	courierSvc := &stubCourierService{
		courier: &models.Courier{ID: id, Status: models.CourierStatusBusy},
	}
	producer := &recordingProducerCourier{
		statusChangedErr: fmt.Errorf("status event failed"),
		locationErr:      fmt.Errorf("location event failed"),
	}

	handler := NewCourierHandler(courierSvc, &stubOrderSvc{}, producer, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/couriers/"+id.String()+"/status", bytes.NewBufferString(`{"status":"available","current_lat":55,"current_lon":37}`))
	rr := httptest.NewRecorder()
	handler.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if producer.statusChangedCalls != 1 {
		t.Fatalf("expected status changed published once, got %d", producer.statusChangedCalls)
	}
	if producer.locationCalls != 1 {
		t.Fatalf("expected location updated published once, got %d", producer.locationCalls)
	}
}

func TestCourierHandler_UpdateStatus_GetCourierNotFound(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := NewCourierHandler(&stubCourierService{getErr: apperror.NotFound("courier not found", nil)}, &stubOrderSvc{}, &stubProducerCourier{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/couriers/"+uuid.New().String()+"/status", bytes.NewBufferString(`{"status":"available"}`))
	rr := httptest.NewRecorder()
	handler.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestCourierHandler_UpdateStatus_UpdateNotFound(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	id := uuid.New()
	handler := NewCourierHandler(&stubCourierService{
		courier: &models.Courier{ID: id, Status: models.CourierStatusAvailable},
		updErr:  apperror.NotFound("courier not found", nil),
	}, &stubOrderSvc{}, &stubProducerCourier{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/couriers/"+id.String()+"/status", bytes.NewBufferString(`{"status":"available"}`))
	rr := httptest.NewRecorder()
	handler.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestCourierHandler_AssignOrder(t *testing.T) {
	h := newCourierHandler()
	id := uuid.New()
	body := bytes.NewBufferString(`{"order_id":"` + uuid.New().String() + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/couriers/"+id.String()+"/assign", body)
	rr := httptest.NewRecorder()
	h.AssignOrderToCourier(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCourierHandler_ListAvailable(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/couriers/available", nil)
	rr := httptest.NewRecorder()
	h.GetAvailableCouriers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCourierHandler_ListAvailable_InvalidMethod(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/couriers/available", nil)
	rr := httptest.NewRecorder()
	h.GetAvailableCouriers(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestCourierHandler_CreateCourier_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	errSvc := &stubCourierService{err: fmt.Errorf("fail")}
	handler := NewCourierHandler(errSvc, &stubOrderSvc{}, &stubProducerCourier{}, &stubRedisMiss{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/couriers", bytes.NewBufferString(`{"name":"Err","phone":"+7999"}`))
	rr := httptest.NewRecorder()
	handler.CreateCourier(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestCourierHandler_UpdateStatus_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	errSvc := &stubCourierService{err: fmt.Errorf("fail")}
	handler := NewCourierHandler(errSvc, &stubOrderSvc{}, &stubProducerCourier{}, &stubRedisMiss{}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/couriers/"+uuid.New().String()+"/status", bytes.NewBufferString(`{"status":"available"}`))
	rr := httptest.NewRecorder()
	handler.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestCourierHandler_GetCouriers(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/couriers?status=available&min_rating=4.5&limit=10&offset=0&order_by=rating", nil)
	rr := httptest.NewRecorder()
	h.GetCouriers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCourierHandler_GetCourierReviews(t *testing.T) {
	h := newCourierHandler()
	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/couriers/"+id.String()+"/reviews?limit=5&offset=0", nil)
	rr := httptest.NewRecorder()
	h.GetCourierReviews(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCourierHandler_UpdateStatus_BadBody(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodPut, "/api/couriers/"+uuid.New().String()+"/status", bytes.NewBufferString("bad"))
	rr := httptest.NewRecorder()
	h.UpdateCourierStatus(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCourierHandler_AssignOrder_BadBody(t *testing.T) {
	h := newCourierHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/couriers/"+uuid.New().String()+"/assign", bytes.NewBufferString("bad"))
	rr := httptest.NewRecorder()
	h.AssignOrderToCourier(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCourierHandler_AssignOrder_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	errSvc := &stubCourierService{err: fmt.Errorf("fail")}
	handler := NewCourierHandler(errSvc, &stubOrderSvc{}, &stubProducerCourier{}, &stubRedisMiss{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/couriers/"+uuid.New().String()+"/assign", bytes.NewBufferString(`{"order_id":"`+uuid.New().String()+`"}`))
	rr := httptest.NewRecorder()
	handler.AssignOrderToCourier(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestCourierHandler_ListAvailable_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	errSvc := &stubCourierService{err: fmt.Errorf("fail")}
	handler := NewCourierHandler(errSvc, &stubOrderSvc{}, &stubProducerCourier{}, &stubRedisMiss{}, log)

	req := httptest.NewRequest(http.MethodGet, "/api/couriers/available", nil)
	rr := httptest.NewRecorder()
	handler.GetAvailableCouriers(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestValidateCreateCourierRequest(t *testing.T) {
	h := &CourierHandler{}
	if err := h.validateCreateCourierRequest(&models.CreateCourierRequest{}); err == nil {
		t.Fatalf("expected validation error for empty payload")
	}
	if err := h.validateCreateCourierRequest(&models.CreateCourierRequest{Name: "Name", Phone: "+7999"}); err != nil {
		t.Fatalf("expected valid payload, got %v", err)
	}
}
