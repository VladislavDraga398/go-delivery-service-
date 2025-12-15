package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/redis"

	"github.com/alicebob/miniredis/v2"
)

func newTestRedis(t *testing.T) *redis.Client {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	parts := strings.Split(mr.Addr(), ":")
	cfg := &config.RedisConfig{
		Host: parts[0],
		Port: parts[1],
		DB:   0,
	}

	log := logger.New(&config.LoggerConfig{Level: "debug", Format: "json"})
	rdb, err := redis.Connect(cfg, log)
	if err != nil {
		t.Fatalf("failed to connect redis: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	return rdb
}

func TestGeocodingService_Geocode_CachesResult(t *testing.T) {
	rdb := newTestRedis(t)

	log := logger.New(&config.LoggerConfig{Level: "debug", Format: "json"})
	service := NewGeocodingService(rdb, log, &config.GeocodingConfig{
		Provider: "offline",
	})

	ctx := context.Background()
	addr := "Moscow, Red Square"

	lat1, lon1, err := service.Geocode(ctx, addr)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	lat2, lon2, err := service.Geocode(ctx, addr)
	if err != nil {
		t.Fatalf("expected success on second call, got err: %v", err)
	}

	if lat1 != lat2 || lon1 != lon2 {
		t.Fatalf("expected cached coords to match: (%.5f, %.5f) vs (%.5f, %.5f)", lat1, lon1, lat2, lon2)
	}

	if lat1 < -90 || lat1 > 90 || lon1 < -180 || lon1 > 180 {
		t.Fatalf("coordinates out of bounds: lat=%.2f lon=%.2f", lat1, lon1)
	}
}

func TestGeocodingService_YandexProvider(t *testing.T) {
	// Mock Yandex API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"response":{"GeoObjectCollection":{"featureMember":[{"GeoObject":{"Point":{"pos":"37.6200 55.7500"}}}]}}}`)
	}))
	defer ts.Close()

	rdb := newTestRedis(t)
	log := logger.New(&config.LoggerConfig{Level: "debug", Format: "json"})

	service := NewGeocodingService(rdb, log, &config.GeocodingConfig{
		Provider:       "yandex",
		YandexAPIKey:   "test-key",
		YandexBaseURL:  ts.URL,
		TimeoutSeconds: 5,
	})

	ctx := context.Background()
	lat, lon, err := service.Geocode(ctx, "Some address")
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	if lat != 55.75 || lon != 37.62 {
		t.Fatalf("unexpected coords: lat=%.2f lon=%.2f", lat, lon)
	}
}
