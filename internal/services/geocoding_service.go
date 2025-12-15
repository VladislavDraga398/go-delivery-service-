package services

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/redis"
)

const geocodeCacheTTL = 24 * time.Hour

// Coordinates представляют координаты точки.
type Coordinates struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// GeocodingService эмулирует геокодер с кешированием в Redis.
// В продакшене сюда можно подключить внешний API (Yandex/Google) и reuse тот же интерфейс.
type GeocodingService struct {
	redis  *redis.Client
	log    *logger.Logger
	client *http.Client
	cfg    *config.GeocodingConfig
}

// NewGeocodingService создает сервис геокодирования.
func NewGeocodingService(redis *redis.Client, log *logger.Logger, cfg *config.GeocodingConfig) *GeocodingService {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &GeocodingService{
		redis:  redis,
		log:    log,
		client: &http.Client{Timeout: timeout},
		cfg:    cfg,
	}
}

// Geocode возвращает координаты по адресу, используя кеш Redis.
// Сейчас используется детерминированное хеш-преобразование адреса (без внешних API).
func (s *GeocodingService) Geocode(ctx context.Context, address string) (float64, float64, error) {
	if address == "" {
		return 0, 0, fmt.Errorf("address is empty")
	}

	key := redis.GenerateKey(redis.KeyPrefixGeocode, hashKey(address))

	// Пробуем из кеша
	var cached Coordinates
	if err := s.redis.Get(ctx, key, &cached); err == nil {
		return cached.Lat, cached.Lon, nil
	}

	// Выбираем провайдера
	var (
		lat float64
		lon float64
		err error
	)

	if strings.EqualFold(s.cfg.Provider, "yandex") && s.cfg.YandexAPIKey != "" {
		lat, lon, err = s.yandexGeocode(ctx, address)
		if err != nil {
			s.log.WithError(err).WithField("address", address).Warn("Yandex geocode failed, fallback to offline")
			lat, lon = hashToCoordinates(address)
		}
	} else {
		lat, lon = hashToCoordinates(address)
	}

	coords := Coordinates{Lat: lat, Lon: lon}

	// Пишем в кеш (best effort)
	if err := s.redis.Set(ctx, key, coords, geocodeCacheTTL); err != nil {
		s.log.WithError(err).WithField("address", address).Warn("Failed to cache geocode result")
	}

	return lat, lon, nil
}

// yandexGeocode вызывает API Яндекс Геокодера и возвращает координаты (lat, lon).
func (s *GeocodingService) yandexGeocode(ctx context.Context, address string) (float64, float64, error) {
	params := url.Values{}
	params.Set("apikey", s.cfg.YandexAPIKey)
	params.Set("format", "json")
	params.Set("geocode", address)

	endpoint := s.cfg.YandexBaseURL
	if endpoint == "" {
		endpoint = "https://geocode-maps.yandex.ru/1.x"
	}

	reqURL := endpoint + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to call yandex geocode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, 0, fmt.Errorf("yandex geocode returned status %d: %s", resp.StatusCode, string(body))
	}

	var data yandexResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, fmt.Errorf("failed to decode yandex geocode response: %w", err)
	}

	pos := data.FirstPos()
	if pos == "" {
		return 0, 0, fmt.Errorf("yandex geocode returned empty position")
	}

	// pos формат: "37.6173 55.7558" (lon lat)
	var lon, lat float64
	_, err = fmt.Sscanf(pos, "%f %f", &lon, &lat)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse position: %w", err)
	}

	return lat, lon, nil
}

// Структуры для парсинга Yandex ответа
type yandexResponse struct {
	Response struct {
		GeoObjectCollection struct {
			FeatureMember []struct {
				GeoObject struct {
					Point struct {
						Pos string `json:"pos"`
					} `json:"Point"`
				} `json:"GeoObject"`
			} `json:"featureMember"`
		} `json:"GeoObjectCollection"`
	} `json:"response"`
}

func (r *yandexResponse) FirstPos() string {
	if len(r.Response.GeoObjectCollection.FeatureMember) == 0 {
		return ""
	}
	return r.Response.GeoObjectCollection.FeatureMember[0].GeoObject.Point.Pos
}

// hashToCoordinates генерирует координаты из адреса.
func hashToCoordinates(address string) (float64, float64) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(address))
	val := h.Sum64()

	// lat: -90..90, lon: -180..180
	lat := -90 + float64(val%18000)/100.0          // шаг 0.01 градуса
	lon := -180 + float64((val/18000)%36000)/100.0 // шаг 0.01 градуса

	return lat, lon
}

// hashKey делает короткий ключ для адреса.
func hashKey(address string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(address))
	return fmt.Sprintf("%x", h.Sum64())
}
