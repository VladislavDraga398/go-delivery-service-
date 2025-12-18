package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/services"
)

// RateLimitHandler отвечает за статус лимита и middleware.
type RateLimitHandler struct {
	limiter RateLimitStatusProvider
	log     *logger.Logger
	cfg     *config.RateLimitConfig
}

// NewRateLimitHandler создает новый RateLimitHandler.
func NewRateLimitHandler(limiter RateLimitStatusProvider, log *logger.Logger, cfg *config.RateLimitConfig) *RateLimitHandler {
	return &RateLimitHandler{
		limiter: limiter,
		log:     log,
		cfg:     cfg,
	}
}

// Status возвращает текущие значения лимита для клиента.
func (h *RateLimitHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if h.limiter == nil || h.cfg == nil || !h.cfg.Enabled {
		writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
		return
	}

	key := services.ExtractClientIP(r)
	used, remaining, resetAt, err := h.limiter.Usage(r.Context(), key)
	if err != nil {
		h.log.WithError(err).Error("Failed to fetch rate limit usage")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to fetch rate limit usage")
		return
	}

	resp := map[string]interface{}{
		"enabled":        true,
		"limit":          h.cfg.Requests,
		"window_seconds": h.cfg.WindowSeconds,
		"used":           used,
		"remaining":      remaining,
		"key":            key,
	}
	if resetAt != nil {
		resp["reset_at"] = resetAt.Format(time.RFC3339)
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// MiddlewareLimiter описывает контракт для rate limiter.
type MiddlewareLimiter interface {
	Allow(ctx context.Context, key string) (bool, int64, time.Time, error)
	Enabled() bool
	Limit() int64
}

// RateLimitStatusProvider расширяет интерфейс для эндпоинта статуса.
type RateLimitStatusProvider interface {
	MiddlewareLimiter
	Usage(ctx context.Context, key string) (int64, int64, *time.Time, error)
}

// RateLimitMiddleware применяет rate limiting к хендлеру.
func RateLimitMiddleware(limiter MiddlewareLimiter, log *logger.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if limiter == nil || !limiter.Enabled() {
			next(w, r)
			return
		}

		key := services.ExtractClientIP(r)
		allowed, remaining, resetAt, err := limiter.Allow(r.Context(), key)
		if err != nil {
			log.WithError(err).Error("Rate limiter failed")
			writeErrorResponse(w, http.StatusInternalServerError, "Rate limiter error")
			return
		}

		// Заголовки совместимые с common rate limit policy
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limiter.Limit(), 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		if !resetAt.IsZero() {
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		}

		if !allowed {
			writeErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		next(w, r)
	}
}
