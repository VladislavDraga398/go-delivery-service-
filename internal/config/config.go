package config

import (
	"os"
	"strconv"
	"strings"
)

// Config представляет конфигурацию приложения
type Config struct {
	Server    ServerConfig    `json:"server"`
	Database  DatabaseConfig  `json:"database"`
	Redis     RedisConfig     `json:"redis"`
	Kafka     KafkaConfig     `json:"kafka"`
	Logger    LoggerConfig    `json:"logger"`
	Geocoding GeocodingConfig `json:"geocoding"`
	Pricing   PricingConfig   `json:"pricing"`
	Analytics AnalyticsConfig `json:"analytics"`
	RateLimit RateLimitConfig `json:"rate_limit"`
}

// ServerConfig представляет конфигурацию HTTP сервера
type ServerConfig struct {
	Port         string `json:"port"`
	Host         string `json:"host"`
	ReadTimeout  int    `json:"read_timeout"`
	WriteTimeout int    `json:"write_timeout"`
}

// DatabaseConfig представляет конфигурацию базы данных
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"db_name"`
	SSLMode  string `json:"ssl_mode"`
}

// RedisConfig представляет конфигурацию Redis
type RedisConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// KafkaConfig представляет конфигурацию Kafka
type KafkaConfig struct {
	Brokers []string `json:"brokers"`
	GroupID string   `json:"group_id"`
	Topics  Topics   `json:"topics"`
}

// Topics представляет список топиков Kafka
type Topics struct {
	Orders    string `json:"orders"`
	Couriers  string `json:"couriers"`
	Locations string `json:"locations"`
}

// LoggerConfig представляет конфигурацию логгера
type LoggerConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	File   string `json:"file"`
}

// GeocodingConfig описывает настройки геокодера
type GeocodingConfig struct {
	Provider       string `json:"provider"`        // offline | yandex
	YandexAPIKey   string `json:"yandex_api_key"`  // Ключ для Yandex геокодера
	YandexBaseURL  string `json:"yandex_base_url"` // https://geocode-maps.yandex.ru/1.x
	TimeoutSeconds int    `json:"timeout_seconds"` // таймаут http-запроса
}

// PricingConfig хранит тарифы для доставки
type PricingConfig struct {
	BaseFare float64 `json:"base_fare"`
	PerKm    float64 `json:"per_km"`
	MinFare  float64 `json:"min_fare"`
}

// AnalyticsConfig хранит настройки аналитики
type AnalyticsConfig struct {
	CacheTTLMinutes       int    `json:"cache_ttl_minutes"`
	MaxRangeDays          int    `json:"max_range_days"`
	DefaultGroupBy        string `json:"default_group_by"`
	DefaultTopLimit       int    `json:"default_top_limit"`
	DefaultCourierLimit   int    `json:"default_courier_limit"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
}

// RateLimitConfig описывает настройки rate limiting
type RateLimitConfig struct {
	Enabled       bool   `json:"enabled"`
	Requests      int    `json:"requests"`
	WindowSeconds int    `json:"window_seconds"`
	KeyPrefix     string `json:"key_prefix"`
}

// Load загружает конфигурацию из переменных окружения
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:  getEnvAsInt("SERVER_READ_TIMEOUT", 10),
			WriteTimeout: getEnvAsInt("SERVER_WRITE_TIMEOUT", 10),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "delivery_user"),
			Password: getEnv("DB_PASSWORD", "delivery_pass"),
			DBName:   getEnv("DB_NAME", "delivery_system"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Kafka: KafkaConfig{
			Brokers: strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			GroupID: getEnv("KAFKA_GROUP_ID", "delivery-service"),
			Topics: Topics{
				Orders:    getEnv("KAFKA_TOPIC_ORDERS", "orders"),
				Couriers:  getEnv("KAFKA_TOPIC_COURIERS", "couriers"),
				Locations: getEnv("KAFKA_TOPIC_LOCATIONS", "locations"),
			},
		},
		Logger: LoggerConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
			File:   getEnv("LOG_FILE", ""),
		},
		Geocoding: GeocodingConfig{
			Provider:       getEnv("GEOCODER_PROVIDER", "offline"),
			YandexAPIKey:   getEnv("YANDEX_GEOCODER_API_KEY", ""),
			YandexBaseURL:  getEnv("YANDEX_GEOCODER_BASE_URL", "https://geocode-maps.yandex.ru/1.x"),
			TimeoutSeconds: getEnvAsInt("GEOCODER_TIMEOUT_SECONDS", 5),
		},
		Pricing: PricingConfig{
			BaseFare: getEnvAsFloat("PRICING_BASE_FARE", 100.0),
			PerKm:    getEnvAsFloat("PRICING_PER_KM", 20.0),
			MinFare:  getEnvAsFloat("PRICING_MIN_FARE", 150.0),
		},
		Analytics: AnalyticsConfig{
			CacheTTLMinutes:       getEnvAsInt("ANALYTICS_CACHE_TTL_MINUTES", 10),
			MaxRangeDays:          getEnvAsInt("ANALYTICS_MAX_RANGE_DAYS", 365),
			DefaultGroupBy:        getEnv("ANALYTICS_DEFAULT_GROUP_BY", "none"),
			DefaultTopLimit:       getEnvAsInt("ANALYTICS_DEFAULT_TOP_LIMIT", 5),
			DefaultCourierLimit:   getEnvAsInt("ANALYTICS_DEFAULT_COURIER_LIMIT", 50),
			RequestTimeoutSeconds: getEnvAsInt("ANALYTICS_REQUEST_TIMEOUT_SECONDS", 5),
		},
		RateLimit: RateLimitConfig{
			Enabled:       getEnvAsBool("RATE_LIMIT_ENABLED", false),
			Requests:      getEnvAsInt("RATE_LIMIT_REQUESTS", 100),
			WindowSeconds: getEnvAsInt("RATE_LIMIT_WINDOW_SECONDS", 60),
			KeyPrefix:     getEnv("RATE_LIMIT_KEY_PREFIX", "ratelimit"),
		},
	}
}

// getEnv получает значение переменной окружения с значением по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt получает значение переменной окружения как int с значением по умолчанию
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// getEnvAsFloat получает значение переменной окружения как float64 с значением по умолчанию
func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	return defaultValue
}

// getEnvAsBool получает значение переменной окружения как bool с значением по умолчанию
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := strings.ToLower(getEnv(key, ""))
	if valueStr == "true" || valueStr == "1" || valueStr == "yes" {
		return true
	}
	if valueStr == "false" || valueStr == "0" || valueStr == "no" {
		return false
	}
	return defaultValue
}
