package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	log "github.com/sirupsen/logrus"
)

type mockConsumerGroup struct {
	consumeFn func(context.Context, []string, sarama.ConsumerGroupHandler) error
	errorsCh  chan error
	closeFn   func() error
}

func (m *mockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	if m.consumeFn != nil {
		return m.consumeFn(ctx, topics, handler)
	}
	return nil
}

func (m *mockConsumerGroup) Errors() <-chan error {
	return m.errorsCh
}

func (m *mockConsumerGroup) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	if m.errorsCh != nil {
		close(m.errorsCh)
	}
	return nil
}

func (m *mockConsumerGroup) Pause(map[string][]int32)  {}
func (m *mockConsumerGroup) Resume(map[string][]int32) {}
func (m *mockConsumerGroup) PauseAll()                 {}
func (m *mockConsumerGroup) ResumeAll()                {}

type mockSession struct {
	ctx    context.Context
	marked []*sarama.ConsumerMessage
}

func (m *mockSession) Claims() map[string][]int32               { return nil }
func (m *mockSession) MemberID() string                         { return "member" }
func (m *mockSession) GenerationID() int32                      { return 1 }
func (m *mockSession) MarkOffset(string, int32, int64, string)  {}
func (m *mockSession) Commit()                                  {}
func (m *mockSession) ResetOffset(string, int32, int64, string) {}
func (m *mockSession) Context() context.Context                 { return m.ctx }
func (m *mockSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	m.marked = append(m.marked, msg)
}

type mockClaim struct {
	topic     string
	partition int32
	messages  chan *sarama.ConsumerMessage
}

func (m *mockClaim) Topic() string                            { return m.topic }
func (m *mockClaim) Partition() int32                         { return m.partition }
func (m *mockClaim) InitialOffset() int64                     { return 0 }
func (m *mockClaim) HighWaterMarkOffset() int64               { return 0 }
func (m *mockClaim) Messages() <-chan *sarama.ConsumerMessage { return m.messages }

func TestNewConsumerErrors(t *testing.T) {
	if _, err := NewConsumer([]string{"invalid-broker:9092"}, "group", []string{"topic"}, func(context.Context, *sarama.ConsumerMessage) error { return nil }); err == nil {
		t.Fatal("expected new consumer error")
	}
	if _, err := NewConsumerWithDLQ([]string{"invalid-broker:9092"}, "group", []string{"topic"}, func(context.Context, *sarama.ConsumerMessage) error { return nil }, nil, 3); err == nil {
		t.Fatal("expected new consumer with dlq error")
	}
}

func TestConsumerStartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	consumeCalls := 0
	errorsCh := make(chan error, 1)
	group := &mockConsumerGroup{
		errorsCh: errorsCh,
		consumeFn: func(_ context.Context, _ []string, _ sarama.ConsumerGroupHandler) error {
			consumeCalls++
			cancel()
			return nil
		},
		closeFn: func() error {
			close(errorsCh)
			return nil
		},
	}

	consumer := &Consumer{
		consumer:   group,
		topics:     []string{"topic-a"},
		handler:    func(context.Context, *sarama.ConsumerMessage) error { return nil },
		logger:     log.WithField("test", "consumer"),
		maxRetries: 2,
	}

	errorsCh <- errors.New("background error")
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := consumer.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if consumeCalls == 0 {
		t.Fatal("expected consume call")
	}
}

func TestConsumerStopError(t *testing.T) {
	errorsCh := make(chan error)
	group := &mockConsumerGroup{errorsCh: errorsCh, closeFn: func() error {
		close(errorsCh)
		return errors.New("close failed")
	}}
	consumer := &Consumer{consumer: group, logger: log.WithField("test", "stop")}
	if err := consumer.Stop(); err == nil {
		t.Fatal("expected stop error")
	}
}

func TestConsumerSetupCleanup(t *testing.T) {
	consumer := &Consumer{}
	if err := consumer.Setup(nil); err != nil {
		t.Fatalf("setup should return nil: %v", err)
	}
	if err := consumer.Cleanup(nil); err != nil {
		t.Fatalf("cleanup should return nil: %v", err)
	}
}

func TestConsumeClaim(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer := &Consumer{
		handler: func(context.Context, *sarama.ConsumerMessage) error { return nil },
		logger:  log.WithField("test", "claim"),
	}

	session := &mockSession{ctx: ctx}
	claim := &mockClaim{topic: "topic", partition: 0, messages: make(chan *sarama.ConsumerMessage, 2)}
	claim.messages <- &sarama.ConsumerMessage{Topic: "topic", Partition: 0, Offset: 1, Key: []byte("k"), Value: []byte("v")}
	close(claim.messages)

	if err := consumer.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("ConsumeClaim failed: %v", err)
	}
	if len(session.marked) != 1 {
		t.Fatalf("expected one marked message, got %d", len(session.marked))
	}
}

func TestConsumeClaimFailedHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer := &Consumer{
		handler:    func(context.Context, *sarama.ConsumerMessage) error { return errors.New("failed") },
		logger:     log.WithField("test", "claim-fail"),
		maxRetries: 1,
	}

	session := &mockSession{ctx: ctx}
	claim := &mockClaim{topic: "topic", partition: 0, messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- &sarama.ConsumerMessage{Topic: "topic", Partition: 0, Offset: 1, Key: []byte("k"), Value: []byte("v")}
	close(claim.messages)

	if err := consumer.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("ConsumeClaim failed: %v", err)
	}
	if len(session.marked) != 0 {
		t.Fatalf("failed message should not be marked, got %d", len(session.marked))
	}
}

func TestHandleMessageWithRetry(t *testing.T) {
	msg := &sarama.ConsumerMessage{Topic: "topic", Key: []byte("key"), Value: []byte(`{"a":1}`)}

	t.Run("success", func(t *testing.T) {
		consumer := &Consumer{
			handler:    func(context.Context, *sarama.ConsumerMessage) error { return nil },
			logger:     log.WithField("test", "retry-success"),
			maxRetries: 2,
		}
		if err := consumer.handleMessageWithRetry(context.Background(), msg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("retry below limit", func(t *testing.T) {
		retryingMessage := &sarama.ConsumerMessage{
			Topic:   "topic",
			Key:     []byte("key"),
			Value:   []byte("{}"),
			Headers: []*sarama.RecordHeader{{Key: []byte(HeaderRetryCount), Value: []byte("1")}},
		}
		attempts := 0
		consumer := &Consumer{
			handler: func(context.Context, *sarama.ConsumerMessage) error {
				attempts++
				return errors.New("temporary")
			},
			logger:     log.WithField("test", "retry"),
			maxRetries: 3,
			retryDelay: 0,
		}
		if err := consumer.handleMessageWithRetry(context.Background(), retryingMessage); err == nil {
			t.Fatal("expected retry error")
		}
		if attempts != 2 {
			t.Fatalf("expected 2 in-process attempts, got %d", attempts)
		}
	})

	t.Run("max retries without dlq", func(t *testing.T) {
		retryingMessage := &sarama.ConsumerMessage{
			Topic:   "topic",
			Key:     []byte("key"),
			Value:   []byte("{}"),
			Headers: []*sarama.RecordHeader{{Key: []byte(HeaderRetryCount), Value: []byte("3")}},
		}
		consumer := &Consumer{
			handler:    func(context.Context, *sarama.ConsumerMessage) error { return errors.New("permanent") },
			logger:     log.WithField("test", "max-no-dlq"),
			maxRetries: 3,
		}
		if err := consumer.handleMessageWithRetry(context.Background(), retryingMessage); err == nil {
			t.Fatal("expected error when dlq is absent")
		}
	})

	t.Run("max retries with dlq success", func(t *testing.T) {
		mockProducer := mocks.NewSyncProducer(t, nil)
		mockProducer.ExpectSendMessageAndSucceed()
		retryingMessage := &sarama.ConsumerMessage{
			Topic:   "topic",
			Key:     []byte("key"),
			Value:   []byte("{}"),
			Headers: []*sarama.RecordHeader{{Key: []byte(HeaderRetryCount), Value: []byte("3")}},
		}
		consumer := &Consumer{
			handler:     func(context.Context, *sarama.ConsumerMessage) error { return errors.New("permanent") },
			dlqProducer: &Producer{producer: mockProducer, logger: log.WithField("test", "dlq")},
			logger:      log.WithField("test", "max-dlq"),
			maxRetries:  3,
		}
		if err := consumer.handleMessageWithRetry(context.Background(), retryingMessage); err != nil {
			t.Fatalf("unexpected error after dlq publish: %v", err)
		}
		if err := mockProducer.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("max retries with dlq failure", func(t *testing.T) {
		mockProducer := mocks.NewSyncProducer(t, nil)
		mockProducer.ExpectSendMessageAndFail(sarama.ErrOutOfBrokers)
		retryingMessage := &sarama.ConsumerMessage{
			Topic:   "topic",
			Key:     []byte("key"),
			Value:   []byte("{}"),
			Headers: []*sarama.RecordHeader{{Key: []byte(HeaderRetryCount), Value: []byte("3")}},
		}
		consumer := &Consumer{
			handler:     func(context.Context, *sarama.ConsumerMessage) error { return errors.New("permanent") },
			dlqProducer: &Producer{producer: mockProducer, logger: log.WithField("test", "dlq")},
			logger:      log.WithField("test", "max-dlq-fail"),
			maxRetries:  3,
		}
		if err := consumer.handleMessageWithRetry(context.Background(), retryingMessage); err == nil {
			t.Fatal("expected dlq failure")
		}
		if err := mockProducer.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGetRetryCountAndParsers(t *testing.T) {
	consumer := &Consumer{}

	msg := &sarama.ConsumerMessage{Headers: []*sarama.RecordHeader{{Key: []byte(HeaderRetryCount), Value: []byte("5")}}}
	if got := consumer.getRetryCount(msg); got != 5 {
		t.Fatalf("unexpected retry count: %d", got)
	}

	msgInvalid := &sarama.ConsumerMessage{Headers: []*sarama.RecordHeader{{Key: []byte(HeaderRetryCount), Value: []byte("bad")}}}
	if got := consumer.getRetryCount(msgInvalid); got != 0 {
		t.Fatalf("invalid retry count should fallback to 0, got %d", got)
	}

	sagaMsg := &sarama.ConsumerMessage{Value: []byte(`{"event_type":"saga.started","order_id":"o-1"}`)}
	if _, err := ParseSagaEvent(sagaMsg); err != nil {
		t.Fatalf("ParseSagaEvent failed: %v", err)
	}
	if _, err := ParseSagaEvent(&sarama.ConsumerMessage{Value: []byte("{")}); err == nil {
		t.Fatal("expected ParseSagaEvent error")
	}

	orderMsg := &sarama.ConsumerMessage{Value: []byte(`{"event_type":"order.created","order_id":"o-1","customer_id":"c-1","status":"pending"}`)}
	if _, err := ParseOrderEvent(orderMsg); err != nil {
		t.Fatalf("ParseOrderEvent failed: %v", err)
	}
	if _, err := ParseOrderEvent(&sarama.ConsumerMessage{Value: []byte("{")}); err == nil {
		t.Fatal("expected ParseOrderEvent error")
	}
}

func TestSendToDLQ(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	consumer := &Consumer{
		dlqProducer: &Producer{producer: mockProducer, logger: log.WithField("test", "send-dlq")},
		logger:      log.WithField("test", "consumer-send-dlq"),
	}

	msg := &sarama.ConsumerMessage{Topic: "orders", Partition: 1, Offset: 42, Key: []byte("k"), Value: []byte("v")}
	if err := consumer.sendToDLQ(msg, errors.New("boom")); err != nil {
		t.Fatalf("sendToDLQ failed: %v", err)
	}

	if err := mockProducer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConsumeClaimStopsOnContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	consumer := &Consumer{
		handler:    func(context.Context, *sarama.ConsumerMessage) error { return nil },
		logger:     log.WithField("test", "claim-stop"),
		maxRetries: 1,
	}
	session := &mockSession{ctx: ctx}
	claim := &mockClaim{topic: "topic", partition: 0, messages: make(chan *sarama.ConsumerMessage)}

	done := make(chan struct{})
	go func() {
		_ = consumer.ConsumeClaim(session, claim)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ConsumeClaim did not stop after context cancellation")
	}
}
