package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IBM/sarama"
)

type stubDB struct{ err error }

func (s *stubDB) Health() error { return s.err }

type stubRedisHealth struct{ err error }

func (s *stubRedisHealth) Health(ctx context.Context) error { return s.err }

func TestHealthHandler_ReadinessOK(t *testing.T) {
	h := NewHealthHandler(&stubDB{}, &stubRedisHealth{}, []string{}, func([]string) error { return nil })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/readiness", nil)

	h.Readiness(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHealthHandler_Readiness_DBError(t *testing.T) {
	h := NewHealthHandler(&stubDB{err: errors.New("db down")}, &stubRedisHealth{}, []string{}, func([]string) error { return nil })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/readiness", nil)

	h.Readiness(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHealthHandler_Liveness(t *testing.T) {
	h := NewHealthHandler(&stubDB{}, &stubRedisHealth{}, []string{}, func([]string) error { return nil })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/liveness", nil)

	h.Liveness(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHealthHandler_Liveness_MethodNotAllowed(t *testing.T) {
	h := NewHealthHandler(&stubDB{}, &stubRedisHealth{}, []string{}, func([]string) error { return nil })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health/liveness", nil)
	h.Liveness(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHealthHandler_Health_Unhealthy(t *testing.T) {
	h := NewHealthHandler(&stubDB{}, &stubRedisHealth{err: errors.New("redis down")}, []string{}, func([]string) error { return nil })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	h.Health(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHealthHandler_KafkaError(t *testing.T) {
	h := NewHealthHandler(&stubDB{}, &stubRedisHealth{}, []string{"kafka:9092"}, func([]string) error { return errors.New("kafka down") })

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/readiness", nil)
	h.Readiness(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	h := NewHealthHandler(&stubDB{}, &stubRedisHealth{}, []string{}, func([]string) error { return nil })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	h.Health(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestCheckKafkaHealth_NoBrokers(t *testing.T) {
	if err := checkKafkaHealth([]string{}); err == nil {
		t.Fatalf("expected error for empty brokers")
	}
}

func TestCheckKafkaHealth_Wrapper(t *testing.T) {
	if err := CheckKafkaHealth([]string{}); err == nil {
		t.Fatalf("expected error for wrapper with empty brokers")
	}
}

func TestCheckKafkaHealth_WithMockBroker(t *testing.T) {
	broker := sarama.NewMockBroker(t, 1)
	defer broker.Close()

	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker.Addr(), broker.BrokerID()).
			SetController(broker.BrokerID()).
			SetLeader("health", 0, broker.BrokerID()),
	})

	if err := checkKafkaHealth([]string{broker.Addr()}); err != nil {
		t.Fatalf("expected kafka health ok, got %v", err)
	}
}
