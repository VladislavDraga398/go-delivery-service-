package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Константы
const (
	defaultCacheTTL = 15 * time.Minute
)

// ErrorResponse представляет структуру ответа с ошибкой
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeJSONResponse отправляет JSON ответ
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// writeErrorResponse отправляет ответ с ошибкой
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	}
	writeJSONResponse(w, statusCode, response)
}

// extractUUIDFromPath извлекает UUID из пути URL
func extractUUIDFromPath(path, prefix string) (uuid.UUID, error) {
	if !strings.HasPrefix(path, prefix) {
		return uuid.Nil, fmt.Errorf("invalid path format")
	}

	// Убираем префикс и получаем ID
	idStr := strings.TrimPrefix(path, prefix)

	// Убираем возможный суффикс (например, /status)
	parts := strings.Split(idStr, "/")
	if len(parts) == 0 {
		return uuid.Nil, fmt.Errorf("missing ID in path")
	}

	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID format: %w", err)
	}

	return id, nil
}
