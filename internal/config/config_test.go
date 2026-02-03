package config

import (
	"os"
	"testing"
)

func TestGetEnvHelpers(t *testing.T) {
	os.Setenv("TEST_STR", "value")
	os.Setenv("TEST_INT", "123")
	os.Setenv("TEST_FLOAT", "3.14")
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_FALSE", "false")

	if v := getEnv("TEST_STR", ""); v != "value" {
		t.Fatalf("expected value, got %s", v)
	}
	if v := getEnvAsInt("TEST_INT", 0); v != 123 {
		t.Fatalf("expected 123, got %d", v)
	}
	if v := getEnvAsFloat("TEST_FLOAT", 0); v != 3.14 {
		t.Fatalf("expected 3.14, got %f", v)
	}
	if !getEnvAsBool("TEST_BOOL_TRUE", false) {
		t.Fatalf("expected true")
	}
	if getEnvAsBool("TEST_BOOL_FALSE", true) {
		t.Fatalf("expected false")
	}
}

func TestLoadDefaults(t *testing.T) {
	// ensure no interfering env vars
	_ = os.Unsetenv("SERVER_PORT")
	cfg := Load()
	if cfg.Server.Port == "" {
		t.Fatalf("expected default server port set")
	}
	if cfg.Analytics.CacheTTLMinutes == 0 {
		t.Fatalf("expected analytics defaults set")
	}
}
