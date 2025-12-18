package redis

import (
	"context"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"

	miniredis "github.com/alicebob/miniredis/v2"
	redislib "github.com/go-redis/redis/v8"
)

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis, context.Context) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redislib.NewClient(&redislib.Options{Addr: mr.Addr()})
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	return &Client{client: rdb, log: log}, mr, context.Background()
}

func TestConnectSuccess(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	cfg := &config.RedisConfig{Host: "127.0.0.1", Port: mr.Port(), DB: 0}

	client, err := Connect(cfg, log)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestConnectFailure(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	cfg := &config.RedisConfig{Host: "127.0.0.1", Port: "0", DB: 0}
	if _, err := Connect(cfg, log); err == nil {
		t.Fatalf("expected connect error")
	}
}

func TestCloseNil(t *testing.T) {
	var client *Client
	if err := client.Close(); err != nil {
		t.Fatalf("expected nil error on nil client close, got %v", err)
	}
}

func TestGenerateKey(t *testing.T) {
	key := GenerateKey("prefix", "123")
	if key != "prefix:123" {
		t.Fatalf("unexpected key: %s", key)
	}
}

func TestSetGetExistsDelete(t *testing.T) {
	client, _, ctx := newTestClient(t)

	type payload struct {
		Value string
	}

	val := payload{Value: "data"}
	if err := client.Set(ctx, "key1", val, time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	var got payload
	if err := client.Get(ctx, "key1", &got); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Value != val.Value {
		t.Fatalf("unexpected value: %+v", got)
	}

	exists, err := client.Exists(ctx, "key1")
	if err != nil || !exists {
		t.Fatalf("exists expected true, got %v err=%v", exists, err)
	}

	if err := client.Delete(ctx, "key1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	exists, _ = client.Exists(ctx, "key1")
	if exists {
		t.Fatalf("expected key removed")
	}
}

func TestGetMissingKey(t *testing.T) {
	client, _, ctx := newTestClient(t)
	var dest struct{}
	if err := client.Get(ctx, "absent", &dest); err == nil {
		t.Fatalf("expected error for missing key")
	}
}

func TestSetMultipleGetMultiple(t *testing.T) {
	client, _, ctx := newTestClient(t)

	values := map[string]interface{}{
		"one":   map[string]string{"v": "1"},
		"two":   map[string]int{"n": 2},
		"three": "plain",
	}

	if err := client.SetMultiple(ctx, values, time.Minute); err != nil {
		t.Fatalf("set multiple failed: %v", err)
	}

	result, err := client.GetMultiple(ctx, []string{"one", "two", "absent"})
	if err != nil {
		t.Fatalf("get multiple failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if _, ok := result["one"]; !ok {
		t.Fatalf("expected key 'one'")
	}
}

func TestDeleteByPrefix(t *testing.T) {
	client, mr, ctx := newTestClient(t)

	_ = mr.Set("stats:1", "a")
	_ = mr.Set("stats:2", "b")
	_ = mr.Set("other:3", "c")

	if err := client.DeleteByPrefix(ctx, "stats"); err != nil {
		t.Fatalf("delete by prefix failed: %v", err)
	}

	if mr.Exists("stats:1") || mr.Exists("stats:2") {
		t.Fatalf("expected stats keys removed")
	}
	if !mr.Exists("other:3") {
		t.Fatalf("expected other key kept")
	}
}

func TestGetIntAndTTL(t *testing.T) {
	client, mr, ctx := newTestClient(t)

	client.client.Set(ctx, "counter", 5, 2*time.Second)

	val, err := client.GetInt(ctx, "counter")
	if err != nil {
		t.Fatalf("get int failed: %v", err)
	}
	if val != 5 {
		t.Fatalf("unexpected int value: %d", val)
	}

	ttl, err := client.TTL(ctx, "counter")
	if err != nil {
		t.Fatalf("ttl failed: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("expected positive ttl, got %v", ttl)
	}

	mr.FastForward(3 * time.Second)
	if _, err := client.GetInt(ctx, "counter"); err == nil {
		t.Fatalf("expected error for expired key")
	}
}

func TestIncrAndExpire(t *testing.T) {
	client, mr, ctx := newTestClient(t)
	val, err := client.Incr(ctx, "hits")
	if err != nil || val != 1 {
		t.Fatalf("expected incr to 1, got %d err=%v", val, err)
	}
	if err := client.Expire(ctx, "hits", time.Second); err != nil {
		t.Fatalf("expire failed: %v", err)
	}
	mr.FastForward(2 * time.Second)
	if _, err := client.GetInt(ctx, "hits"); err == nil {
		t.Fatalf("expected key expired")
	}
}

func TestHealth(t *testing.T) {
	client, _, ctx := newTestClient(t)
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health failed: %v", err)
	}
}
