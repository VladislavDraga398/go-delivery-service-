package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"
	"delivery-system/internal/models"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

func TestConsumer_ProcessMessage_WithHandler(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	c := &Consumer{
		log:      log,
		handlers: make(map[models.EventType]EventHandler),
	}

	called := false
	c.RegisterHandler(models.EventTypeOrderCreated, func(ctx context.Context, event *models.Event) error {
		called = true
		return nil
	})

	ev := models.Event{ID: uuid.New(), Type: models.EventTypeOrderCreated}
	data, _ := json.Marshal(ev)
	msg := &sarama.ConsumerMessage{Value: data, Topic: "orders"}

	if err := c.processMessage(msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatalf("handler not called")
	}
	if c.HandlerCount() != 1 {
		t.Fatalf("handler count expected 1")
	}
}

func TestConsumer_ProcessMessage_NoHandler(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	c := &Consumer{
		log:      log,
		handlers: make(map[models.EventType]EventHandler),
		ctx:      context.Background(),
	}

	ev := models.Event{ID: uuid.New(), Type: models.EventTypeCourierAssigned}
	data, _ := json.Marshal(ev)
	msg := &sarama.ConsumerMessage{Value: data, Topic: "orders"}

	if err := c.processMessage(msg); err != nil {
		t.Fatalf("expected no error for missing handler, got %v", err)
	}
}

func TestConsumer_ProcessMessage_HandlerError(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	c := &Consumer{
		log:      log,
		handlers: make(map[models.EventType]EventHandler),
		ctx:      context.Background(),
	}

	expectedErr := fmt.Errorf("fail")
	c.RegisterHandler(models.EventTypeOrderCreated, func(ctx context.Context, event *models.Event) error {
		return expectedErr
	})

	ev := models.Event{ID: uuid.New(), Type: models.EventTypeOrderCreated}
	data, _ := json.Marshal(ev)
	msg := &sarama.ConsumerMessage{Value: data, Topic: "orders"}

	if err := c.processMessage(msg); err == nil {
		t.Fatalf("expected handler error")
	}
}

func TestConsumer_ProcessMessage_InvalidJSON(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	c := &Consumer{
		log:      log,
		handlers: make(map[models.EventType]EventHandler),
		ctx:      context.Background(),
	}

	msg := &sarama.ConsumerMessage{Value: []byte("not json"), Topic: "orders"}

	if err := c.processMessage(msg); err == nil {
		t.Fatalf("expected unmarshal error")
	}
}

type mockConsumerGroup struct {
	consumeCount int
}

func (m *mockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	m.consumeCount++
	_ = handler.Setup(nil)
	return ctx.Err()
}
func (m *mockConsumerGroup) Errors() <-chan error      { ch := make(chan error); close(ch); return ch }
func (m *mockConsumerGroup) Close() error              { return nil }
func (m *mockConsumerGroup) Pause(map[string][]int32)  {}
func (m *mockConsumerGroup) Resume(map[string][]int32) {}
func (m *mockConsumerGroup) PauseAll()                 {}
func (m *mockConsumerGroup) ResumeAll()                {}

type mockSession struct {
	ctx context.Context
}

func (m *mockSession) Claims() map[string][]int32                                               { return nil }
func (m *mockSession) MemberID() string                                                         { return "" }
func (m *mockSession) GenerationID() int32                                                      { return 0 }
func (m *mockSession) MarkOffset(topic string, partition int32, offset int64, metadata string)  {}
func (m *mockSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {}
func (m *mockSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string)                 {}
func (m *mockSession) Commit()                                                                  {}
func (m *mockSession) Context() context.Context                                                 { return m.ctx }

type mockClaim struct {
	msgs chan *sarama.ConsumerMessage
}

func (m *mockClaim) Topic() string              { return "orders" }
func (m *mockClaim) Partition() int32           { return 0 }
func (m *mockClaim) InitialOffset() int64       { return 0 }
func (m *mockClaim) HighWaterMarkOffset() int64 { return 0 }
func (m *mockClaim) Messages() <-chan *sarama.ConsumerMessage {
	return m.msgs
}

func TestConsumer_StartStop(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	mockGroup := &mockConsumerGroup{}
	ctx, cancel := context.WithCancel(context.Background())

	c := &Consumer{
		consumer: mockGroup,
		log:      log,
		handlers: map[models.EventType]EventHandler{},
		topics:   []string{"orders"},
		ctx:      ctx,
		cancel:   cancel,
	}

	if err := c.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := c.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if mockGroup.consumeCount == 0 {
		t.Fatalf("expected Consume called")
	}
}

func TestNewTestConsumer(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	mockGroup := &mockConsumerGroup{}
	c := NewTestConsumer(mockGroup, log)
	if c.consumer != mockGroup {
		t.Fatalf("consumer group not set")
	}
	if err := c.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := c.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if mockGroup.consumeCount == 0 {
		t.Fatalf("expected Consume called at least once")
	}
}

func TestConsumer_Handler(t *testing.T) {
	c := &Consumer{handlers: map[models.EventType]EventHandler{}}
	h := func(ctx context.Context, event *models.Event) error { return nil }
	c.RegisterHandler("custom", h)
	if c.Handler("custom") == nil {
		t.Fatalf("expected handler returned")
	}
}

func TestConsumer_ConsumeClaim(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	c := &Consumer{
		log:      log,
		handlers: map[models.EventType]EventHandler{},
		ctx:      context.Background(),
	}
	c.RegisterHandler(models.EventTypeOrderCreated, func(ctx context.Context, event *models.Event) error { return nil })

	msgs := make(chan *sarama.ConsumerMessage, 1)
	ev := models.Event{ID: uuid.New(), Type: models.EventTypeOrderCreated}
	data, _ := json.Marshal(ev)
	msgs <- &sarama.ConsumerMessage{Value: data, Topic: "orders"}
	close(msgs)

	session := &mockSession{ctx: context.Background()}
	claim := &mockClaim{msgs: msgs}

	if err := c.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("consume claim failed: %v", err)
	}
}

func TestNewConsumer_Error(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	cfg := &config.KafkaConfig{Brokers: []string{"localhost:0"}, GroupID: "g", Topics: config.Topics{}}
	if _, err := NewConsumer(cfg, log); err == nil {
		t.Fatalf("expected error creating consumer")
	}
}

func TestConsumer_Cleanup(t *testing.T) {
	c := &Consumer{}
	if err := c.Cleanup(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
