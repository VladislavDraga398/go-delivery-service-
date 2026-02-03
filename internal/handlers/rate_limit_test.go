package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
)

type stubLimiter struct {
	allowSeq []bool
	idx      int
	limit    int64
	enabled  bool
	err      error
}

func (s *stubLimiter) Allow(_ context.Context, _ string) (bool, int64, time.Time, error) {
	if s.err != nil {
		return false, 0, time.Time{}, s.err
	}
	if s.idx >= len(s.allowSeq) {
		return false, 0, time.Now(), nil
	}
	val := s.allowSeq[s.idx]
	s.idx++
	return val, s.limit - int64(s.idx), time.Now().Add(time.Minute), nil
}

func (s *stubLimiter) Enabled() bool {
	if s.enabled {
		return true
	}
	return s.enabled || len(s.allowSeq) > 0
}
func (s *stubLimiter) Limit() int64 { return s.limit }
func (s *stubLimiter) Usage(_ context.Context, _ string) (int64, int64, *time.Time, error) {
	now := time.Now().Add(time.Minute)
	return 0, s.limit, &now, nil
}

func TestRateLimitMiddleware_BlocksAfterLimit(t *testing.T) {
	limiter := &stubLimiter{allowSeq: []bool{true, false}, limit: 1}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})

	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RateLimitMiddleware(limiter, log, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.RemoteAddr = "1.2.3.4:1234"

	rr1 := httptest.NewRecorder()
	wrapped(rr1, req)
	if rr1.Code != http.StatusOK || calls != 1 {
		t.Fatalf("first request expected 200, calls=1; got %d, calls=%d", rr1.Code, calls)
	}

	rr2 := httptest.NewRecorder()
	wrapped(rr2, req)
	if rr2.Code != http.StatusTooManyRequests || calls != 1 {
		t.Fatalf("second request expected 429, calls still 1; got %d, calls=%d", rr2.Code, calls)
	}
}

func TestRateLimitMiddleware_DisabledSkips(t *testing.T) {
	limiter := &stubLimiter{enabled: false}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RateLimitMiddleware(limiter, log, handler)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	wrapped(rr, req)

	if calls != 1 || rr.Code != http.StatusOK {
		t.Fatalf("expected middleware to skip limiter, code=%d calls=%d", rr.Code, calls)
	}
}

func TestRateLimitMiddleware_Error(t *testing.T) {
	limiter := &stubLimiter{allowSeq: []bool{true}, limit: 1, enabled: true, err: errors.New("fail")}
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	RateLimitMiddleware(limiter, log, handler)(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on limiter error, got %d", rr.Code)
	}
}

func TestRateLimitStatus_Disabled(t *testing.T) {
	handler := NewRateLimitHandler(nil, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.RateLimitConfig{Enabled: false})
	req := httptest.NewRequest(http.MethodGet, "/api/rate-limit/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() == "" {
		t.Fatalf("expected body, got empty")
	}
}

type errorStatusLimiter struct {
	MiddlewareLimiter
}

func (e *errorStatusLimiter) Usage(ctx context.Context, key string) (int64, int64, *time.Time, error) {
	return 0, 0, nil, errors.New("usage error")
}

func TestRateLimitStatus_Error(t *testing.T) {
	limiter := &stubLimiter{allowSeq: []bool{true}, limit: 5, enabled: true}
	statusLimiter := &errorStatusLimiter{MiddlewareLimiter: limiter}
	handler := NewRateLimitHandler(statusLimiter, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.RateLimitConfig{Enabled: true})

	req := httptest.NewRequest(http.MethodGet, "/api/rate-limit/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestRateLimitStatus_MethodNotAllowed(t *testing.T) {
	handler := NewRateLimitHandler(nil, logger.New(&config.LoggerConfig{Level: "error", Format: "json"}), &config.RateLimitConfig{Enabled: true})
	req := httptest.NewRequest(http.MethodPost, "/api/rate-limit/status", nil)
	rr := httptest.NewRecorder()
	handler.Status(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
