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

type stubOrderService struct {
	order        *models.Order
	orders       []*models.Order
	review       *models.Review
	err          error
	statusCalled bool
}

func (s *stubOrderService) CreateOrder(ctx context.Context, req *models.CreateOrderRequest) (*models.Order, error) {
	return s.order, s.err
}
func (s *stubOrderService) GetOrder(ctx context.Context, orderID uuid.UUID) (*models.Order, error) {
	return s.order, s.err
}
func (s *stubOrderService) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, req *models.UpdateOrderStatusRequest) error {
	s.statusCalled = true
	return s.err
}
func (s *stubOrderService) GetOrders(ctx context.Context, status *models.OrderStatus, courierID *uuid.UUID, limit, offset int) ([]*models.Order, error) {
	return s.orders, s.err
}
func (s *stubOrderService) CreateReview(ctx context.Context, orderID uuid.UUID, req *models.CreateReviewRequest) (*models.Review, error) {
	return s.review, s.err
}
func (s *stubOrderService) GetCourierReviews(ctx context.Context, courierID uuid.UUID, limit, offset int) ([]*models.Review, error) {
	return []*models.Review{}, s.err
}

type stubAssignmentService struct {
	courier *models.Courier
	err     error
	called  bool
}

func (s *stubAssignmentService) AutoAssignCourier(ctx context.Context, orderID uuid.UUID, deliveryLat, deliveryLon float64) (*models.Courier, error) {
	s.called = true
	return s.courier, s.err
}

type stubGeocodingService struct{}

func (s *stubGeocodingService) Geocode(ctx context.Context, address string) (float64, float64, error) {
	return 55.0, 37.0, nil
}

type stubProducer struct {
	created bool
	status  bool
}

func (p *stubProducer) PublishOrderCreated(order *models.Order) error {
	p.created = true
	return nil
}
func (p *stubProducer) PublishOrderStatusChanged(orderID uuid.UUID, oldStatus, newStatus models.OrderStatus, courierID *uuid.UUID) error {
	p.status = true
	return nil
}
func (p *stubProducer) PublishCourierStatusChanged(courierID uuid.UUID, oldStatus, newStatus models.CourierStatus) error {
	return nil
}
func (p *stubProducer) PublishLocationUpdated(courierID uuid.UUID, lat, lon float64) error {
	return nil
}
func (p *stubProducer) PublishCourierAssigned(orderID, courierID uuid.UUID) error {
	return nil
}

type stubRedis struct{}

func (s *stubRedis) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}
func (s *stubRedis) Get(ctx context.Context, key string, dest interface{}) error { return nil }
func (s *stubRedis) Delete(ctx context.Context, key string) error                { return nil }
func (s *stubRedis) DeleteByPrefix(ctx context.Context, prefix string) error     { return nil }

var _ RedisClient = (*stubRedis)(nil)

type stubRedisMissOrder struct{}

func (s *stubRedisMissOrder) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}
func (s *stubRedisMissOrder) Get(ctx context.Context, key string, dest interface{}) error {
	return fmt.Errorf("miss")
}
func (s *stubRedisMissOrder) Delete(ctx context.Context, key string) error            { return nil }
func (s *stubRedisMissOrder) DeleteByPrefix(ctx context.Context, prefix string) error { return nil }

type recordingGeocoder struct{ calls int }

func (r *recordingGeocoder) Geocode(ctx context.Context, address string) (float64, float64, error) {
	r.calls++
	return 55.0, 37.0, nil
}

func newTestOrderHandler(order *models.Order) *OrderHandler {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	return NewOrderHandler(
		&stubOrderService{order: order, orders: []*models.Order{order}},
		&stubAssignmentService{courier: &models.Courier{ID: uuid.New(), Name: "c"}},
		&stubGeocodingService{},
		&stubProducer{},
		&stubRedis{},
		log,
	)
}

func TestOrderHandler_CreateOrder_Success(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID, CustomerName: "Test", PickupLat: floatPtr(1), PickupLon: floatPtr(1), DeliveryLat: floatPtr(2), DeliveryLon: floatPtr(2), Items: []models.OrderItem{}}
	h := newTestOrderHandler(order)

	payload := `{"customer_name":"Test","customer_phone":"+7999","delivery_address":"addr","pickup_address":"pick","pickup_lat":1,"pickup_lon":1,"delivery_lat":2,"delivery_lon":2,"items":[{"name":"Item","quantity":1,"price":10}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(payload))
	rr := httptest.NewRecorder()

	h.CreateOrder(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestOrderHandler_CreateOrder_Invalid(t *testing.T) {
	h := newTestOrderHandler(&models.Order{})

	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()
	h.CreateOrder(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(`{"customer_name":"","items":[]}`))
	rr = httptest.NewRecorder()
	h.CreateOrder(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for validation, got %d", rr.Code)
	}
}

func TestOrderHandler_CreateOrder_MethodNotAllowed(t *testing.T) {
	h := newTestOrderHandler(&models.Order{})
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	rr := httptest.NewRecorder()
	h.CreateOrder(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestOrderHandler_CreateOrder_ServiceError(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	h := NewOrderHandler(&stubOrderService{err: fmt.Errorf("fail")}, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedisMissOrder{}, log)
	body := `{"customer_name":"Test","customer_phone":"+7999","delivery_address":"addr","pickup_address":"p","pickup_lat":1,"pickup_lon":1,"delivery_lat":2,"delivery_lon":2,"items":[{"name":"x","quantity":1,"price":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.CreateOrder(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestOrderHandler_GetOrder(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID}
	h := newTestOrderHandler(order)

	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+orderID.String(), nil)
	rr := httptest.NewRecorder()
	h.GetOrder(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestOrderHandler_GetOrder_InvalidID(t *testing.T) {
	h := newTestOrderHandler(&models.Order{})
	req := httptest.NewRequest(http.MethodGet, "/api/orders/not-uuid", nil)
	rr := httptest.NewRecorder()
	h.GetOrder(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", rr.Code)
	}
}

func TestOrderHandler_GetOrder_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	h := NewOrderHandler(&stubOrderService{err: fmt.Errorf("fail")}, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedisMissOrder{}, log)
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	h.GetOrder(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestOrderHandler_CreateReview(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	orderService := &stubOrderService{order: order, review: &models.Review{ID: uuid.New(), Rating: 5}}
	h := NewOrderHandler(orderService, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	body := bytes.NewBufferString(`{"rating":5,"comment":"ok"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+orderID.String()+"/review", body)
	rr := httptest.NewRecorder()

	h.CreateReview(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestOrderHandler_CreateReview_MethodNotAllowed(t *testing.T) {
	h := newTestOrderHandler(&models.Order{ID: uuid.New()})
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+uuid.New().String()+"/review", nil)
	rr := httptest.NewRecorder()
	h.CreateReview(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestOrderHandler_CreateReview_Error(t *testing.T) {
	orderID := uuid.New()
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	orderService := &stubOrderService{order: &models.Order{ID: orderID}, err: fmt.Errorf("fail")}
	h := NewOrderHandler(orderService, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedisMissOrder{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+orderID.String()+"/review", bytes.NewBufferString(`{"rating":5}`))
	rr := httptest.NewRecorder()
	h.CreateReview(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestOrderHandler_GetOrders(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID}
	h := newTestOrderHandler(order)

	req := httptest.NewRequest(http.MethodGet, "/api/orders?limit=1&offset=0", nil)
	rr := httptest.NewRecorder()
	h.GetOrders(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestOrderHandler_GetOrders_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	h := NewOrderHandler(&stubOrderService{err: fmt.Errorf("fail")}, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedisMissOrder{}, log)
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	rr := httptest.NewRecorder()
	h.GetOrders(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestOrderHandler_UpdateStatus_InvalidMethod(t *testing.T) {
	h := newTestOrderHandler(&models.Order{ID: uuid.New()})
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+uuid.New().String()+"/status", nil)
	rr := httptest.NewRecorder()
	h.UpdateOrderStatus(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestOrderHandler_UpdateStatus(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID}
	stubSvc := &stubOrderService{order: order}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	h := NewOrderHandler(stubSvc, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	body := bytes.NewBufferString(`{"status":"delivered"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/orders/"+orderID.String()+"/status", body)
	rr := httptest.NewRecorder()
	h.UpdateOrderStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !stubSvc.statusCalled {
		t.Fatalf("expected status update call")
	}
}

func TestOrderHandler_UpdateStatus_BadBody(t *testing.T) {
	h := newTestOrderHandler(&models.Order{ID: uuid.New()})
	req := httptest.NewRequest(http.MethodPut, "/api/orders/"+uuid.New().String()+"/status", bytes.NewBufferString("bad"))
	rr := httptest.NewRecorder()
	h.UpdateOrderStatus(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestOrderHandler_UpdateStatus_NotFound(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	svc := &stubOrderService{err: apperror.NotFound("order not found", nil)}
	h := NewOrderHandler(svc, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/orders/"+uuid.New().String()+"/status", bytes.NewBufferString(`{"status":"delivered"}`))
	rr := httptest.NewRecorder()
	h.UpdateOrderStatus(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestOrderHandler_UpdateStatus_ServiceError(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	svc := &stubOrderService{err: fmt.Errorf("fail")}
	h := NewOrderHandler(svc, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPut, "/api/orders/"+uuid.New().String()+"/status", bytes.NewBufferString(`{"status":"delivered"}`))
	rr := httptest.NewRecorder()
	h.UpdateOrderStatus(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestOrderHandler_AutoAssign_InvalidID(t *testing.T) {
	h := newTestOrderHandler(&models.Order{})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/not-uuid/auto-assign", nil)
	rr := httptest.NewRecorder()
	h.AutoAssignCourier(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestOrderHandler_AutoAssign_MethodNotAllowed(t *testing.T) {
	h := newTestOrderHandler(&models.Order{})
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+uuid.New().String()+"/auto-assign", nil)
	rr := httptest.NewRecorder()
	h.AutoAssignCourier(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestOrderHandler_AutoAssign(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID, DeliveryLat: floatPtr(2), DeliveryLon: floatPtr(2)}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	svc := &stubOrderService{order: order}
	assign := &stubAssignmentService{courier: &models.Courier{ID: uuid.New()}}
	h := NewOrderHandler(svc, assign, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+orderID.String()+"/auto-assign", bytes.NewBufferString(`{"delivery_lat":2,"delivery_lon":2}`))
	rr := httptest.NewRecorder()
	h.AutoAssignCourier(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestOrderHandler_AutoAssign_Error(t *testing.T) {
	orderID := uuid.New()
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	svc := &stubOrderService{order: &models.Order{ID: orderID, DeliveryLat: floatPtr(1), DeliveryLon: floatPtr(2)}}
	assign := &stubAssignmentService{err: fmt.Errorf("assign fail")}
	h := NewOrderHandler(svc, assign, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+orderID.String()+"/auto-assign", bytes.NewBufferString(`{"delivery_lat":1,"delivery_lon":2}`))
	rr := httptest.NewRecorder()
	h.AutoAssignCourier(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestOrderHandler_AutoAssign_NotFound(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	svc := &stubOrderService{err: apperror.NotFound("order not found", nil)}
	h := NewOrderHandler(svc, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+uuid.New().String()+"/auto-assign", nil)
	rr := httptest.NewRecorder()
	h.AutoAssignCourier(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestOrderHandler_AutoAssign_PartialCoordinates(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	h := NewOrderHandler(&stubOrderService{order: order}, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	body := bytes.NewBufferString(`{"delivery_lat":10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+orderID.String()+"/auto-assign", body)
	rr := httptest.NewRecorder()
	h.AutoAssignCourier(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestOrderHandler_AutoAssign_MissingOrderCoords(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	h := NewOrderHandler(&stubOrderService{order: order}, &stubAssignmentService{}, &stubGeocodingService{}, &stubProducer{}, &stubRedis{}, log)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+orderID.String()+"/auto-assign", nil)
	rr := httptest.NewRecorder()
	h.AutoAssignCourier(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestOrderHandler_CreateOrder_AutoAssignWithGeocode(t *testing.T) {
	orderID := uuid.New()
	order := &models.Order{ID: orderID}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})

	geo := &recordingGeocoder{}
	assign := &stubAssignmentService{courier: &models.Courier{ID: uuid.New(), Name: "assigned"}}
	h := NewOrderHandler(&stubOrderService{order: order, orders: []*models.Order{order}}, assign, geo, &stubProducer{}, &stubRedis{}, log)

	body := `{"customer_name":"Geo","customer_phone":"+7999","delivery_address":"delivery addr","pickup_address":"pickup addr","items":[{"name":"Item","quantity":1,"price":10}],"auto_assign":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.CreateOrder(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	if geo.calls < 2 {
		t.Fatalf("expected geocoder called for both addresses, got %d", geo.calls)
	}
	if !assign.called {
		t.Fatalf("expected auto-assign to be triggered")
	}
}

func floatPtr(v float64) *float64 { return &v }
