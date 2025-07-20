package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/IBM/sarama"
)

// EventHandler представляет обработчик событий
type EventHandler func(ctx context.Context, event *models.Event) error

// Consumer представляет Kafka consumer
type Consumer struct {
	consumer sarama.ConsumerGroup
	log      *logger.Logger
	handlers map[models.EventType]EventHandler
	topics   []string
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewConsumer создает новый Kafka consumer
func NewConsumer(cfg *config.KafkaConfig, log *logger.Logger) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Group.Session.Timeout = 10000000000   // 10 секунд
	config.Consumer.Group.Heartbeat.Interval = 3000000000 // 3 секунды

	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	topics := []string{cfg.Topics.Orders, cfg.Topics.Couriers, cfg.Topics.Locations}

	log.Info("Kafka consumer created successfully")

	return &Consumer{
		consumer: consumer,
		log:      log,
		handlers: make(map[models.EventType]EventHandler),
		topics:   topics,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// RegisterHandler регистрирует обработчик для определенного типа события
func (c *Consumer) RegisterHandler(eventType models.EventType, handler EventHandler) {
	c.handlers[eventType] = handler
	c.log.WithField("event_type", eventType).Info("Event handler registered")
}

// Start запускает consumer
func (c *Consumer) Start() error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				if err := c.consumer.Consume(c.ctx, c.topics, c); err != nil {
					c.log.WithError(err).Error("Error consuming messages")
				}
			}
		}
	}()

	c.log.Info("Kafka consumer started")
	return nil
}

// Stop останавливает consumer
func (c *Consumer) Stop() error {
	c.cancel()
	c.wg.Wait()
	return c.consumer.Close()
}

// Setup реализует интерфейс sarama.ConsumerGroupHandler
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup реализует интерфейс sarama.ConsumerGroupHandler
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim реализует интерфейс sarama.ConsumerGroupHandler
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			if err := c.processMessage(message); err != nil {
				c.log.WithError(err).
					WithField("topic", message.Topic).
					WithField("partition", message.Partition).
					WithField("offset", message.Offset).
					Error("Failed to process message")
			} else {
				session.MarkMessage(message, "")
			}

		case <-session.Context().Done():
			return nil
		}
	}
}

// processMessage обрабатывает полученное сообщение
func (c *Consumer) processMessage(message *sarama.ConsumerMessage) error {
	var event models.Event
	if err := json.Unmarshal(message.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	c.log.WithField("event_type", event.Type).
		WithField("event_id", event.ID).
		WithField("topic", message.Topic).
		Debug("Processing event")

	// Находим обработчик для данного типа события
	handler, exists := c.handlers[event.Type]
	if !exists {
		c.log.WithField("event_type", event.Type).Warn("No handler registered for event type")
		return nil // Не возвращаем ошибку, просто пропускаем событие
	}

	// Вызываем обработчик
	if err := handler(c.ctx, &event); err != nil {
		return fmt.Errorf("handler failed for event type %s: %w", event.Type, err)
	}

	c.log.WithField("event_type", event.Type).
		WithField("event_id", event.ID).
		Debug("Event processed successfully")

	return nil
}
