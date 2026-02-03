package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/redis"
)

// RateLimiter реализует простое ограничение по количеству запросов в фиксированном окне на ключ (IP).
type RateLimiter struct {
	redis   rateRedis
	log     *logger.Logger
	enabled bool
	limit   int64
	window  time.Duration
	prefix  string
}

type rateRedis interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	GetInt(ctx context.Context, key string) (int64, error)
}

// NewRateLimiter создаёт rate limiter.
func NewRateLimiter(redisClient *redis.Client, log *logger.Logger, cfg *config.RateLimitConfig) *RateLimiter {
	if redisClient == nil || cfg == nil || !cfg.Enabled || cfg.Requests <= 0 || cfg.WindowSeconds <= 0 {
		return &RateLimiter{enabled: false}
	}

	prefix := cfg.KeyPrefix
	if prefix == "" {
		prefix = "ratelimit"
	}

	return &RateLimiter{
		redis:   redisClient,
		log:     log,
		enabled: true,
		limit:   int64(cfg.Requests),
		window:  time.Duration(cfg.WindowSeconds) * time.Second,
		prefix:  prefix,
	}
}

// Allow возвращает признак разрешения, оставшийся лимит и время сброса окна.
func (r *RateLimiter) Allow(ctx context.Context, key string) (allowed bool, remaining int64, resetAt time.Time, err error) {
	if !r.enabled {
		return true, r.limit, time.Now().Add(r.window), nil
	}

	now := time.Now()
	redisKey := r.makeKey(key)

	count, err := r.redis.Incr(ctx, redisKey)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("rate limiter incr failed: %w", err)
	}

	if count == 1 {
		if err := r.redis.Expire(ctx, redisKey, r.window); err != nil {
			r.log.WithError(err).WithField("key", redisKey).Warn("failed to set rate limit ttl")
		}
	}

	ttl, ttlErr := r.redis.TTL(ctx, redisKey)
	if ttlErr != nil {
		r.log.WithError(ttlErr).WithField("key", redisKey).Warn("failed to get rate limit ttl")
		ttl = r.window
	}

	remaining = r.limit - count
	if remaining < 0 {
		remaining = 0
	}
	resetAt = now.Add(ttl)

	return count <= r.limit, remaining, resetAt, nil
}

// Usage возвращает текущее значение окна и время сброса.
func (r *RateLimiter) Usage(ctx context.Context, key string) (used int64, remaining int64, resetAt *time.Time, err error) {
	if !r.enabled {
		return 0, r.limit, nil, nil
	}

	redisKey := r.makeKey(key)
	count, err := r.redis.GetInt(ctx, redisKey)
	if err != nil {
		// если ключа нет — считаем нулём
		return 0, r.limit, nil, nil
	}

	ttl, ttlErr := r.redis.TTL(ctx, redisKey)
	if ttlErr != nil {
		r.log.WithError(ttlErr).WithField("key", redisKey).Warn("failed to get rate limit ttl")
	} else {
		tmp := time.Now().Add(ttl)
		resetAt = &tmp
	}

	remaining = r.limit - count
	if remaining < 0 {
		remaining = 0
	}

	return count, remaining, resetAt, nil
}

func (r *RateLimiter) makeKey(key string) string {
	safeKey := strings.ReplaceAll(key, ":", "_")
	return fmt.Sprintf("%s:%s", r.prefix, safeKey)
}

// Limit возвращает лимит для текущего окна.
func (r *RateLimiter) Limit() int64 {
	return r.limit
}

// Enabled сообщает, включён ли rate limiting.
func (r *RateLimiter) Enabled() bool {
	return r.enabled
}

// ExtractClientIP получает IP из заголовков/RemoteAddr.
func ExtractClientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); ip != "" {
		parts := strings.Split(ip, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
