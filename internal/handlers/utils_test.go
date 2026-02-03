package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractUUIDFromPath(t *testing.T) {
	id := "123e4567-e89b-12d3-a456-426614174000"
	parsed, err := extractUUIDFromPath("/api/orders/"+id, "/api/orders/")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.String() != id {
		t.Fatalf("unexpected id: %s", parsed)
	}

	if _, err := extractUUIDFromPath("/wrong/path", "/api/orders/"); err == nil {
		t.Fatalf("expected error for invalid path")
	}
}

func TestWriteJSONResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONResponse(rr, http.StatusOK, map[string]string{"ok": "true"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content-type: %s", ct)
	}
	if body := rr.Body.String(); body == "" {
		t.Fatalf("empty body")
	}
}
