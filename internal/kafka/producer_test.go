package kafka

import (
	"testing"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/google/uuid"
)

func TestPublishEvent(t *testing.T) {
	cfg := sarama.NewConfig()
	mp := mocks.NewSyncProducer(t, cfg)
	mp.ExpectSendMessageAndSucceed()

	event := models.Event{ID: uuid.New(), Type: models.EventTypeOrderCreated}
	p := &Producer{
		producer: mp,
		log:      logger.New(&config.LoggerConfig{Level: "error", Format: "json"}),
		topics:   &config.Topics{Orders: "orders"},
	}
	if err := p.publishEvent("orders", event); err != nil {
		t.Fatalf("expected publish success, got %v", err)
	}

	if err := mp.Close(); err != nil {
		t.Fatalf("failed to close mock producer: %v", err)
	}
}

func TestProducer_WrapperMethods(t *testing.T) {
	cfg := sarama.NewConfig()
	mp := mocks.NewSyncProducer(t, cfg)
	for i := 0; i < 5; i++ {
		mp.ExpectSendMessageAndSucceed()
	}

	p := &Producer{
		producer: mp,
		log:      logger.New(&config.LoggerConfig{Level: "error", Format: "json"}),
		topics:   &config.Topics{Orders: "orders", Couriers: "couriers", Locations: "locations"},
	}

	orderID := uuid.New()
	courierID := uuid.New()
	order := &models.Order{ID: orderID, CustomerName: "n", CustomerPhone: "p", DeliveryAddress: "addr", TotalAmount: 10}

	if err := p.PublishOrderCreated(order); err != nil {
		t.Fatalf("PublishOrderCreated failed: %v", err)
	}
	if err := p.PublishOrderStatusChanged(orderID, models.OrderStatusCreated, models.OrderStatusDelivered, &courierID); err != nil {
		t.Fatalf("PublishOrderStatusChanged failed: %v", err)
	}
	if err := p.PublishCourierAssigned(orderID, courierID); err != nil {
		t.Fatalf("PublishCourierAssigned failed: %v", err)
	}
	if err := p.PublishCourierStatusChanged(courierID, models.CourierStatusAvailable, models.CourierStatusBusy); err != nil {
		t.Fatalf("PublishCourierStatusChanged failed: %v", err)
	}
	if err := p.PublishLocationUpdated(courierID, 1, 2); err != nil {
		t.Fatalf("PublishLocationUpdated failed: %v", err)
	}
}

func TestProducer_PublishEvent_Failure(t *testing.T) {
	cfg := sarama.NewConfig()
	mp := mocks.NewSyncProducer(t, cfg)
	mp.ExpectSendMessageAndFail(sarama.ErrOutOfBrokers)

	p := &Producer{
		producer: mp,
		log:      logger.New(&config.LoggerConfig{Level: "error", Format: "json"}),
		topics:   &config.Topics{Orders: "orders"},
	}

	ev := models.Event{ID: uuid.New(), Type: models.EventTypeOrderCreated}
	err := p.publishEvent("orders", ev)
	if err == nil {
		t.Fatalf("expected error on send failure")
	}
	_ = p.Close()
}

func TestNewProducer_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	cfg := &config.KafkaConfig{Brokers: []string{"localhost:0"}}
	if _, err := NewProducer(cfg, log); err == nil {
		t.Fatalf("expected error creating producer")
	}
}

func TestProducer_CloseNil(t *testing.T) {
	var p *Producer
	if err := p.Close(); err != nil {
		t.Fatalf("expected nil error on nil producer")
	}
	p = &Producer{}
	if err := p.Close(); err != nil {
		t.Fatalf("expected nil error on empty producer, got %v", err)
	}
}
