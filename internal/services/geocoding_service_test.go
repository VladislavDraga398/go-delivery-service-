package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/redis"

	"github.com/alicebob/miniredis/v2"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestRedis(t *testing.T) *redis.Client {
	mr, err := miniredis.Run()
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skip: cannot start miniredis in this environment: %v", err)
		}
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

func TestGeocodingService_YandexGeocode_StatusNotOK(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	service := NewGeocodingService(nil, log, &config.GeocodingConfig{
		Provider:       "yandex",
		YandexAPIKey:   "k",
		YandexBaseURL:  "http://example",
		TimeoutSeconds: 1,
	})
	service.client = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("oops")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	if _, _, err := service.yandexGeocode(context.Background(), "addr"); err == nil {
		t.Fatalf("expected error for non-200 response")
	}
}

func TestGeocodingService_YandexGeocode_DecodeError(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	service := NewGeocodingService(nil, log, &config.GeocodingConfig{
		Provider:       "yandex",
		YandexAPIKey:   "k",
		YandexBaseURL:  "http://example",
		TimeoutSeconds: 1,
	})
	service.client = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	if _, _, err := service.yandexGeocode(context.Background(), "addr"); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestGeocodingService_YandexGeocode_EmptyPos(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	service := NewGeocodingService(nil, log, &config.GeocodingConfig{
		Provider:       "yandex",
		YandexAPIKey:   "k",
		YandexBaseURL:  "http://example",
		TimeoutSeconds: 1,
	})
	service.client = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"response":{"GeoObjectCollection":{"featureMember":[]}}}`)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	if _, _, err := service.yandexGeocode(context.Background(), "addr"); err == nil {
		t.Fatalf("expected empty pos error")
	}
}

func TestGeocodingService_YandexGeocode_ParseError(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	service := NewGeocodingService(nil, log, &config.GeocodingConfig{
		Provider:       "yandex",
		YandexAPIKey:   "k",
		YandexBaseURL:  "http://example",
		TimeoutSeconds: 1,
	})
	service.client = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"response":{"GeoObjectCollection":{"featureMember":[{"GeoObject":{"Point":{"pos":"nope"}}}]}}}`)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	if _, _, err := service.yandexGeocode(context.Background(), "addr"); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestGeocodingService_YandexProvider(t *testing.T) {
	// Mock Yandex API
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skip: cannot start test HTTP server: %v", err)
		}
		t.Fatalf("failed to listen for test server: %v", err)
	}

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"response":{"GeoObjectCollection":{"featureMember":[{"GeoObject":{"Point":{"pos":"37.6200 55.7500"}}}]}}}`)
	}))
	ts.Listener = ln
	ts.Start()
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

func TestYandexResponse_FirstPos(t *testing.T) {
	var resp yandexResponse
	if resp.FirstPos() != "" {
		t.Fatalf("expected empty pos for empty response")
	}

	resp.Response.GeoObjectCollection.FeatureMember = append(resp.Response.GeoObjectCollection.FeatureMember, struct {
		GeoObject struct {
			Point struct {
				Pos string `json:"pos"`
			} `json:"Point"`
		} `json:"GeoObject"`
	}{
		GeoObject: struct {
			Point struct {
				Pos string `json:"pos"`
			} `json:"Point"`
		}{
			Point: struct {
				Pos string `json:"pos"`
			}{Pos: "37 55"},
		},
	})

	if got := resp.FirstPos(); got != "37 55" {
		t.Fatalf("unexpected first pos: %s", got)
	}
}
