package kafka

import (
	"encoding/json"
	"fmt"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

// Producer представляет Kafka producer
type Producer struct {
	producer sarama.SyncProducer
	log      *logger.Logger
	topics   *config.Topics
}

// NewProducer создает новый Kafka producer
func NewProducer(cfg *config.KafkaConfig, log *logger.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll       // Ждем подтверждения от всех реплик
	config.Producer.Retry.Max = 3                          // Максимум 3 попытки
	config.Producer.Return.Successes = true                // Возвращаем успешные результаты
	config.Producer.Compression = sarama.CompressionSnappy // Сжатие данных

	producer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	log.Info("Kafka producer created successfully")

	return &Producer{
		producer: producer,
		log:      log,
		topics:   &cfg.Topics,
	}, nil
}

// Close закрывает producer
func (p *Producer) Close() error {
	return p.producer.Close()
}

// PublishOrderCreated публикует событие создания заказа
func (p *Producer) PublishOrderCreated(order *models.Order) error {
	event := models.Event{
		ID:        uuid.New(),
		Type:      models.EventTypeOrderCreated,
		Timestamp: time.Now(),
		Data: models.OrderCreatedEvent{
			OrderID:         order.ID,
			CustomerName:    order.CustomerName,
			CustomerPhone:   order.CustomerPhone,
			DeliveryAddress: order.DeliveryAddress,
			TotalAmount:     order.TotalAmount,
		},
	}

	return p.publishEvent(p.topics.Orders, event)
}

// PublishOrderStatusChanged публикует событие изменения статуса заказа
func (p *Producer) PublishOrderStatusChanged(orderID uuid.UUID, oldStatus, newStatus models.OrderStatus, courierID *uuid.UUID) error {
	event := models.Event{
		ID:        uuid.New(),
		Type:      models.EventTypeOrderStatusChanged,
		Timestamp: time.Now(),
		Data: models.OrderStatusChangedEvent{
			OrderID:   orderID,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			CourierID: courierID,
			Timestamp: time.Now(),
		},
	}

	return p.publishEvent(p.topics.Orders, event)
}

// PublishCourierAssigned публикует событие назначения курьера
func (p *Producer) PublishCourierAssigned(orderID, courierID uuid.UUID) error {
	event := models.Event{
		ID:        uuid.New(),
		Type:      models.EventTypeCourierAssigned,
		Timestamp: time.Now(),
		Data: models.CourierAssignedEvent{
			OrderID:   orderID,
			CourierID: courierID,
			Timestamp: time.Now(),
		},
	}

	return p.publishEvent(p.topics.Couriers, event)
}

// PublishCourierStatusChanged публикует событие изменения статуса курьера
func (p *Producer) PublishCourierStatusChanged(courierID uuid.UUID, oldStatus, newStatus models.CourierStatus) error {
	event := models.Event{
		ID:        uuid.New(),
		Type:      models.EventTypeCourierStatusChanged,
		Timestamp: time.Now(),
		Data: models.CourierStatusChangedEvent{
			CourierID: courierID,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			Timestamp: time.Now(),
		},
	}

	return p.publishEvent(p.topics.Couriers, event)
}

// PublishLocationUpdated публикует событие обновления местоположения
func (p *Producer) PublishLocationUpdated(courierID uuid.UUID, lat, lon float64) error {
	event := models.Event{
		ID:        uuid.New(),
		Type:      models.EventTypeLocationUpdated,
		Timestamp: time.Now(),
		Data: models.LocationUpdatedEvent{
			CourierID: courierID,
			Lat:       lat,
			Lon:       lon,
			Timestamp: time.Now(),
		},
	}

	return p.publishEvent(p.topics.Locations, event)
}

// publishEvent публикует событие в указанный топик
func (p *Producer) publishEvent(topic string, event models.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(event.ID.String()),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("event_type"),
				Value: []byte(event.Type),
			},
			{
				Key:   []byte("timestamp"),
				Value: []byte(event.Timestamp.Format(time.RFC3339)),
			},
		},
	}

	partition, offset, err := p.producer.SendMessage(message)
	if err != nil {
		return fmt.Errorf("failed to send message to topic %s: %w", topic, err)
	}

	p.log.WithField("topic", topic).
		WithField("partition", partition).
		WithField("offset", offset).
		WithField("event_type", event.Type).
		WithField("event_id", event.ID).
		Debug("Event published successfully")

	return nil
}
