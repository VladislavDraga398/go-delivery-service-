package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/redis"
)

type fakeRateRedis struct {
	data   map[string]int64
	expire map[string]time.Time
}

func newFakeRateRedis() *fakeRateRedis {
	return &fakeRateRedis{
		data:   make(map[string]int64),
		expire: make(map[string]time.Time),
	}
}

func (f *fakeRateRedis) Incr(ctx context.Context, key string) (int64, error) {
	f.cleanup()
	val := f.data[key] + 1
	f.data[key] = val
	return val, nil
}

func (f *fakeRateRedis) Expire(ctx context.Context, key string, ttl time.Duration) error {
	f.expire[key] = time.Now().Add(ttl)
	return nil
}

func (f *fakeRateRedis) TTL(ctx context.Context, key string) (time.Duration, error) {
	f.cleanup()
	if exp, ok := f.expire[key]; ok {
		return time.Until(exp), nil
	}
	return 0, nil
}

func (f *fakeRateRedis) GetInt(ctx context.Context, key string) (int64, error) {
	f.cleanup()
	val, ok := f.data[key]
	if !ok {
		return 0, nil
	}
	return val, nil
}

func (f *fakeRateRedis) cleanup() {
	now := time.Now()
	for k, exp := range f.expire {
		if now.After(exp) {
			delete(f.expire, k)
			delete(f.data, k)
		}
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	fakeRedis := newFakeRateRedis()
	limiter := &RateLimiter{
		redis:   fakeRedis,
		enabled: true,
		limit:   2,
		window:  time.Second,
		prefix:  "test",
	}

	ctx := context.Background()
	allowed, remaining, _, err := limiter.Allow(ctx, "ip1")
	if err != nil || !allowed || remaining != 1 {
		t.Fatalf("first request should be allowed, remaining=1, got allowed=%v remaining=%d err=%v", allowed, remaining, err)
	}

	allowed, remaining, _, err = limiter.Allow(ctx, "ip1")
	if err != nil || !allowed || remaining != 0 {
		t.Fatalf("second request should be allowed, remaining=0, got allowed=%v remaining=%d err=%v", allowed, remaining, err)
	}

	allowed, remaining, _, err = limiter.Allow(ctx, "ip1")
	if err != nil || allowed || remaining != 0 {
		t.Fatalf("third request should be blocked, got allowed=%v remaining=%d err=%v", allowed, remaining, err)
	}
}

func TestRateLimiter_NewDisabled(t *testing.T) {
	if limiter := NewRateLimiter(nil, nil, nil); limiter.Enabled() {
		t.Fatalf("expected limiter disabled without cfg/redis")
	}
	cfg := &config.RateLimitConfig{Enabled: false}
	if limiter := NewRateLimiter(nil, nil, cfg); limiter.Enabled() {
		t.Fatalf("expected limiter disabled when cfg disabled")
	}
}

type stubRateRedis struct{}

func (s *stubRateRedis) Incr(ctx context.Context, key string) (int64, error) { return 1, nil }
func (s *stubRateRedis) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}
func (s *stubRateRedis) TTL(ctx context.Context, key string) (time.Duration, error) {
	return time.Second, nil
}
func (s *stubRateRedis) GetInt(ctx context.Context, key string) (int64, error) { return 0, nil }

func TestRateLimiter_NewEnabled(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 10, WindowSeconds: 60, KeyPrefix: "p"}
	limiter := NewRateLimiter(&redis.Client{}, nil, cfg)
	limiter.redis = &stubRateRedis{}
	if !limiter.Enabled() || limiter.Limit() != 10 {
		t.Fatalf("expected enabled limiter with limit 10")
	}
}

func TestRateLimiter_UsageAndLimit(t *testing.T) {
	fakeRedis := newFakeRateRedis()
	limiter := &RateLimiter{redis: fakeRedis, enabled: true, limit: 3, window: time.Minute, prefix: "rl"}
	_, _, _, _ = limiter.Allow(context.Background(), "ip1")
	_, _, _, _ = limiter.Allow(context.Background(), "ip1")

	used, remaining, resetAt, err := limiter.Usage(context.Background(), "ip1")
	if err != nil || used != 2 || remaining != 1 || resetAt == nil {
		t.Fatalf("unexpected usage: used=%d remaining=%d reset=%v err=%v", used, remaining, resetAt, err)
	}

	if limiter.Limit() != 3 || !limiter.Enabled() {
		t.Fatalf("limit/enabled accessors mismatch")
	}
}

func TestExtractClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Real-IP", "10.0.0.1")
	if ip := ExtractClientIP(r); ip != "10.0.0.1" {
		t.Fatalf("expected real ip, got %s", ip)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.2, 10.0.0.3")
	if ip := ExtractClientIP(r); ip != "10.0.0.2" {
		t.Fatalf("expected first forwarded ip, got %s", ip)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.0.1:1234"
	if ip := ExtractClientIP(r); ip != "192.168.0.1" {
		t.Fatalf("expected remote addr ip, got %s", ip)
	}
}
