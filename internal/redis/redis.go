package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"

	"github.com/go-redis/redis/v8"
)

// Client представляет клиент Redis
type Client struct {
	client *redis.Client
	log    *logger.Logger
}

// Connect создает подключение к Redis
func Connect(cfg *config.RedisConfig, log *logger.Logger) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Проверка подключения
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Successfully connected to Redis")

	return &Client{
		client: rdb,
		log:    log,
	}, nil
}

// Close закрывает подключение к Redis
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Set устанавливает значение с TTL
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	err = c.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	c.log.WithField("key", key).Debug("Value set in Redis")
	return nil
}

// Get получает значение по ключу
func (c *Client) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key %s not found", key)
		}
		return fmt.Errorf("failed to get key %s: %w", key, err)
	}

	err = json.Unmarshal([]byte(val), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal value for key %s: %w", key, err)
	}

	c.log.WithField("key", key).Debug("Value retrieved from Redis")
	return nil
}

// Delete удаляет значение по ключу
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	c.log.WithField("key", key).Debug("Key deleted from Redis")
	return nil
}

// Exists проверяет существование ключа
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if key %s exists: %w", key, err)
	}

	return exists > 0, nil
}

// Incr увеличивает значение по ключу и возвращает новое значение
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to incr key %s: %w", key, err)
	}
	return val, nil
}

// Expire устанавливает TTL для ключа
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set ttl for key %s: %w", key, err)
	}
	return nil
}

// TTL возвращает оставшийся TTL для ключа
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get ttl for key %s: %w", key, err)
	}
	return ttl, nil
}

// GetInt получает значение и парсит в int64
func (c *Client) GetInt(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("key %s not found", key)
		}
		return 0, fmt.Errorf("failed to get int value for key %s: %w", key, err)
	}
	return val, nil
}

// SetMultiple устанавливает несколько значений за одну операцию
func (c *Client) SetMultiple(ctx context.Context, values map[string]interface{}, ttl time.Duration) error {
	pipe := c.client.Pipeline()

	for key, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
		}
		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute pipeline: %w", err)
	}

	c.log.WithField("count", len(values)).Debug("Multiple values set in Redis")
	return nil
}

// GetMultiple получает несколько значений за одну операцию
func (c *Client) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return make(map[string]string), nil
	}

	values, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple keys: %w", err)
	}

	result := make(map[string]string)
	for i, key := range keys {
		if values[i] != nil {
			result[key] = values[i].(string)
		}
	}

	c.log.WithField("count", len(result)).Debug("Multiple values retrieved from Redis")
	return result, nil
}

// Health проверяет состояние Redis
func (c *Client) Health(ctx context.Context) error {
	_, err := c.client.Ping(ctx).Result()
	return err
}

// DeleteByPrefix удаляет ключи по префиксу (использует SCAN).
func (c *Client) DeleteByPrefix(ctx context.Context, prefix string) error {
	iter := c.client.Scan(ctx, 0, prefix+"*", 0).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys by prefix %s: %w", prefix, err)
	}
	if len(keys) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete keys by prefix %s: %w", prefix, err)
	}

	c.log.WithFields(map[string]interface{}{
		"prefix": prefix,
		"count":  len(keys),
	}).Debug("Deleted Redis keys by prefix")

	return nil
}

// GenerateKey генерирует ключ для кеша
func GenerateKey(prefix, id string) string {
	return fmt.Sprintf("%s:%s", prefix, id)
}

// Константы для префиксов ключей
const (
	KeyPrefixOrder   = "order"
	KeyPrefixCourier = "courier"
	KeyPrefixStats   = "stats"
	KeyPrefixGeocode = "geocode"
)
