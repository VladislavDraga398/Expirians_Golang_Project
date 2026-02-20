package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

func TestParseBrokers(t *testing.T) {
	brokers := parseBrokers(" broker-1:9092, ,broker-2:9092 ")
	if len(brokers) != 2 {
		t.Fatalf("unexpected brokers count: got=%d want=2", len(brokers))
	}
	if brokers[0] != "broker-1:9092" || brokers[1] != "broker-2:9092" {
		t.Fatalf("unexpected brokers: %+v", brokers)
	}
}

func TestExtractReplayMessage_ConsumerDLQPayload(t *testing.T) {
	payload := map[string]any{
		"original_topic": "oms.order.events",
		"original_key":   "order-1",
		"original_value": `{"id":"evt-1"}`,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	message := &sarama.ConsumerMessage{Value: raw}
	got, ok, err := extractReplayMessage(message, "fallback-topic")
	if err != nil {
		t.Fatalf("extractReplayMessage failed: %v", err)
	}
	if !ok {
		t.Fatal("expected replay candidate")
	}
	if got.topic != "oms.order.events" {
		t.Fatalf("unexpected topic: %s", got.topic)
	}
	if got.key != "order-1" {
		t.Fatalf("unexpected key: %s", got.key)
	}
	if string(got.value) != `{"id":"evt-1"}` {
		t.Fatalf("unexpected replay value: %s", string(got.value))
	}
}

func TestExtractReplayMessage_OutboxDLQPayload(t *testing.T) {
	envelope := map[string]any{
		"id":             "outbox-1",
		"aggregate_type": "order",
		"aggregate_id":   "order-1",
		"event_type":     "order.confirmed",
		"payload": map[string]any{
			"outbox_id":      "outbox-1",
			"aggregate_type": "order",
			"aggregate_id":   "order-1",
			"event_type":     "order.confirmed",
			"payload": map[string]any{
				"status": "confirmed",
			},
			"publish_error": "timeout",
		},
	}

	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope failed: %v", err)
	}

	message := &sarama.ConsumerMessage{Value: raw}
	got, ok, err := extractReplayMessage(message, "oms.order.events")
	if err != nil {
		t.Fatalf("extractReplayMessage failed: %v", err)
	}
	if !ok {
		t.Fatal("expected replay candidate")
	}
	if got.topic != "oms.order.events" {
		t.Fatalf("unexpected topic: %s", got.topic)
	}
	if got.key != "order-1" {
		t.Fatalf("unexpected key: %s", got.key)
	}
	if !json.Valid(got.value) {
		t.Fatalf("replay payload must be valid JSON: %s", string(got.value))
	}
}

func TestExtractReplayMessage_OutboxInvalidNestedPayload(t *testing.T) {
	envelope := map[string]any{
		"id":             "outbox-1",
		"aggregate_type": "order",
		"aggregate_id":   "order-1",
		"event_type":     "order.confirmed",
		"payload": map[string]any{
			"outbox_id":      "outbox-1",
			"aggregate_type": "order",
			"aggregate_id":   "order-1",
			"event_type":     "order.confirmed",
			// nested payload intentionally omitted to trigger validation error branch
		},
	}

	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope failed: %v", err)
	}

	_, ok, err := extractReplayMessage(&sarama.ConsumerMessage{Value: raw}, "oms.order.events")
	if err == nil {
		t.Fatal("expected error for missing nested payload")
	}
	if ok {
		t.Fatal("expected no replay candidate")
	}
}

func TestExtractReplayMessage_UnknownPayload(t *testing.T) {
	message := &sarama.ConsumerMessage{Value: []byte(`{"foo":"bar"}`)}

	_, ok, err := extractReplayMessage(message, "oms.order.events")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected message to be skipped")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x", "y"); got != "x" {
		t.Fatalf("unexpected first non-empty value: %q", got)
	}
	if got := firstNonEmpty("", " "); got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestReadConfig_FromFlags(t *testing.T) {
	withFlagArgs(t, []string{
		"-brokers=broker-1:9092,broker-2:9092",
		"-source-topic=oms.dlq",
		"-target-topic=oms.order.events",
		"-limit=10",
		"-execute=true",
		"-from-newest=true",
		"-idle-timeout=3s",
	}, func() {
		cfg, err := readConfig()
		if err != nil {
			t.Fatalf("readConfig failed: %v", err)
		}
		if len(cfg.brokers) != 2 {
			t.Fatalf("unexpected brokers count: %d", len(cfg.brokers))
		}
		if cfg.limit != 10 {
			t.Fatalf("unexpected limit: %d", cfg.limit)
		}
		if !cfg.execute {
			t.Fatal("expected execute=true")
		}
		if !cfg.fromNewest {
			t.Fatal("expected fromNewest=true")
		}
		if cfg.idleTimeout.Seconds() != 3 {
			t.Fatalf("unexpected idle-timeout: %s", cfg.idleTimeout)
		}
	})
}

func TestReadConfig_ValidationErrors(t *testing.T) {
	withFlagArgs(t, []string{"-brokers=", "-source-topic=oms.dlq", "-target-topic=oms.order.events"}, func() {
		_, err := readConfig()
		if err == nil || !strings.Contains(err.Error(), "kafka brokers are required") {
			t.Fatalf("expected brokers validation error, got: %v", err)
		}
	})

	withFlagArgs(t, []string{"-brokers=broker:9092", "-source-topic=", "-target-topic=oms.order.events"}, func() {
		_, err := readConfig()
		if err == nil || !strings.Contains(err.Error(), "source-topic is required") {
			t.Fatalf("expected source-topic validation error, got: %v", err)
		}
	})

	withFlagArgs(t, []string{"-brokers=broker:9092", "-source-topic=oms.dlq", "-target-topic=", "-limit=1"}, func() {
		_, err := readConfig()
		if err == nil || !strings.Contains(err.Error(), "target-topic is required") {
			t.Fatalf("expected target-topic validation error, got: %v", err)
		}
	})

	withFlagArgs(t, []string{"-brokers=broker:9092", "-source-topic=oms.dlq", "-target-topic=oms.order.events", "-limit=0"}, func() {
		_, err := readConfig()
		if err == nil || !strings.Contains(err.Error(), "limit must be > 0") {
			t.Fatalf("expected limit validation error, got: %v", err)
		}
	})

	withFlagArgs(t, []string{"-brokers=broker:9092", "-source-topic=oms.dlq", "-target-topic=oms.order.events", "-idle-timeout=0s"}, func() {
		_, err := readConfig()
		if err == nil || !strings.Contains(err.Error(), "idle-timeout must be > 0") {
			t.Fatalf("expected idle-timeout validation error, got: %v", err)
		}
	})
}

func TestPublishReplay(t *testing.T) {
	if err := publishReplay(nil, replayMessage{}); err == nil {
		t.Fatal("expected error for nil producer")
	}

	producer := &stubReplayProducer{}
	err := publishReplay(producer, replayMessage{topic: "topic", key: "key", value: []byte(`{"x":1}`)})
	if err != nil {
		t.Fatalf("publishReplay failed: %v", err)
	}
	if producer.calls != 1 {
		t.Fatalf("unexpected producer calls: %d", producer.calls)
	}
	if producer.lastMsg == nil || producer.lastMsg.Topic != "topic" {
		t.Fatalf("unexpected last message: %+v", producer.lastMsg)
	}

	producer.sendErr = errors.New("send failed")
	err = publishReplay(producer, replayMessage{topic: "topic", key: "key", value: []byte(`{"x":1}`)})
	if err == nil {
		t.Fatal("expected publishReplay error")
	}
}

func TestProcessPartition_DryRun(t *testing.T) {
	client := &stubOffsetClient{
		partitions: []int32{0},
		offsets: map[int32]offsetRange{
			0: {oldest: 0, newest: 2},
		},
	}
	consumer := &stubPartitionConsumerSource{
		consumers: map[int32]partitionConsumer{
			0: closedPartitionConsumer([]*sarama.ConsumerMessage{{
				Partition: 0,
				Offset:    0,
				Value:     []byte(`{"original_topic":"oms.order.events","original_key":"order-1","original_value":"{\"id\":\"evt-1\"}"}`),
			}}),
		},
	}

	cfg := config{
		sourceTopic: "oms.dlq",
		targetTopic: "oms.order.events",
		idleTimeout: 20 * time.Millisecond,
	}

	stats, err := processPartition(context.Background(), consumer, client, nil, cfg, 0, 10)
	if err != nil {
		t.Fatalf("processPartition failed: %v", err)
	}
	if stats.processed != 1 || stats.replayed != 1 || stats.skipped != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(consumer.calls) != 1 || consumer.calls[0].offset != 0 {
		t.Fatalf("unexpected consume calls: %+v", consumer.calls)
	}
}

func TestProcessPartition_Execute(t *testing.T) {
	client := &stubOffsetClient{
		offsets: map[int32]offsetRange{0: {oldest: 0, newest: 2}},
	}
	consumer := &stubPartitionConsumerSource{
		consumers: map[int32]partitionConsumer{
			0: closedPartitionConsumer([]*sarama.ConsumerMessage{{
				Partition: 0,
				Offset:    0,
				Value:     []byte(`{"original_topic":"oms.order.events","original_key":"order-1","original_value":"{\"id\":\"evt-1\"}"}`),
			}}),
		},
	}
	producer := &stubReplayProducer{}

	cfg := config{sourceTopic: "oms.dlq", targetTopic: "oms.order.events", execute: true, idleTimeout: 20 * time.Millisecond}

	stats, err := processPartition(context.Background(), consumer, client, producer, cfg, 0, 10)
	if err != nil {
		t.Fatalf("processPartition failed: %v", err)
	}
	if stats.replayed != 1 {
		t.Fatalf("expected replayed=1, got %+v", stats)
	}
	if producer.calls != 1 {
		t.Fatalf("expected one producer call, got %d", producer.calls)
	}
}

func TestProcessPartition_ErrorBranches(t *testing.T) {
	cfg := config{sourceTopic: "oms.dlq", targetTopic: "oms.order.events", execute: true, idleTimeout: 20 * time.Millisecond}

	clientOffsetErr := &stubOffsetClient{offsetErr: map[int32]error{0: errors.New("offset")}}
	if _, err := processPartition(context.Background(), &stubPartitionConsumerSource{}, clientOffsetErr, &stubReplayProducer{}, cfg, 0, 1); err == nil {
		t.Fatal("expected offset error")
	}

	client := &stubOffsetClient{offsets: map[int32]offsetRange{0: {oldest: 0, newest: 2}}}
	consumerErr := &stubPartitionConsumerSource{consumeErr: errors.New("consume")}
	if _, err := processPartition(context.Background(), consumerErr, client, &stubReplayProducer{}, cfg, 0, 1); err == nil {
		t.Fatal("expected consume error")
	}

	pcWithErr := &stubPartitionConsumer{
		messages: make(chan *sarama.ConsumerMessage),
		errors:   make(chan *sarama.ConsumerError, 1),
	}
	pcWithErr.errors <- &sarama.ConsumerError{Err: errors.New("consumer boom")}
	close(pcWithErr.errors)
	consumer := &stubPartitionConsumerSource{consumers: map[int32]partitionConsumer{0: pcWithErr}}
	if _, err := processPartition(context.Background(), consumer, client, &stubReplayProducer{}, cfg, 0, 1); err == nil {
		t.Fatal("expected consumer error branch")
	}
	close(pcWithErr.messages)

	pcBadPayload := closedPartitionConsumer([]*sarama.ConsumerMessage{{
		Partition: 0,
		Offset:    0,
		Value:     []byte(`{"id":"x","payload":"not-an-object"}`),
	}})
	consumer = &stubPartitionConsumerSource{consumers: map[int32]partitionConsumer{0: pcBadPayload}}
	stats, err := processPartition(context.Background(), consumer, client, &stubReplayProducer{}, cfg, 0, 1)
	if err != nil {
		t.Fatalf("unexpected bad-payload error: %v", err)
	}
	if stats.skipped != 1 {
		t.Fatalf("expected skipped=1, got %+v", stats)
	}

	pcOK := closedPartitionConsumer([]*sarama.ConsumerMessage{{
		Partition: 0,
		Offset:    0,
		Value:     []byte(`{"original_topic":"oms.order.events","original_key":"order-1","original_value":"{\"id\":\"evt-1\"}"}`),
	}})
	consumer = &stubPartitionConsumerSource{consumers: map[int32]partitionConsumer{0: pcOK}}
	producer := &stubReplayProducer{sendErr: errors.New("send fail")}
	if _, err := processPartition(context.Background(), consumer, client, producer, cfg, 0, 1); err == nil {
		t.Fatal("expected producer send error")
	}
}

func TestProcessPartition_IdleTimeoutAndContext(t *testing.T) {
	client := &stubOffsetClient{offsets: map[int32]offsetRange{0: {oldest: 0, newest: 2}}}

	idleConsumer := &stubPartitionConsumer{
		messages: make(chan *sarama.ConsumerMessage),
		errors:   make(chan *sarama.ConsumerError),
	}
	consumer := &stubPartitionConsumerSource{consumers: map[int32]partitionConsumer{0: idleConsumer}}
	cfg := config{sourceTopic: "oms.dlq", targetTopic: "oms.order.events", idleTimeout: 10 * time.Millisecond}

	stats, err := processPartition(context.Background(), consumer, client, nil, cfg, 0, 1)
	if err != nil {
		t.Fatalf("unexpected idle-timeout error: %v", err)
	}
	if stats.processed != 0 {
		t.Fatalf("expected processed=0, got %+v", stats)
	}
	close(idleConsumer.messages)
	close(idleConsumer.errors)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	canceledPC := &stubPartitionConsumer{
		messages: make(chan *sarama.ConsumerMessage),
		errors:   make(chan *sarama.ConsumerError),
	}
	canceledConsumer := &stubPartitionConsumerSource{consumers: map[int32]partitionConsumer{0: canceledPC}}
	if _, err := processPartition(ctx, canceledConsumer, client, nil, cfg, 0, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	close(canceledPC.messages)
	close(canceledPC.errors)
}

func TestRunReplay(t *testing.T) {
	cfg := config{sourceTopic: "oms.dlq", targetTopic: "oms.order.events", limit: 1, idleTimeout: 20 * time.Millisecond}

	if err := runReplay(context.Background(), cfg, nil, nil, nil); err == nil {
		t.Fatal("expected missing deps error")
	}

	client := &stubOffsetClient{
		partitions: []int32{2, 0},
		offsets: map[int32]offsetRange{
			0: {oldest: 0, newest: 2},
			2: {oldest: 0, newest: 2},
		},
	}
	consumer := &stubPartitionConsumerSource{
		consumers: map[int32]partitionConsumer{
			0: closedPartitionConsumer([]*sarama.ConsumerMessage{{
				Partition: 0,
				Offset:    0,
				Value:     []byte(`{"original_topic":"oms.order.events","original_key":"order-1","original_value":"{\"id\":\"evt-1\"}"}`),
			}}),
			2: closedPartitionConsumer([]*sarama.ConsumerMessage{{
				Partition: 2,
				Offset:    0,
				Value:     []byte(`{"original_topic":"oms.order.events","original_key":"order-2","original_value":"{\"id\":\"evt-2\"}"}`),
			}}),
		},
	}

	if err := runReplay(context.Background(), cfg, client, consumer, nil); err != nil {
		t.Fatalf("runReplay failed: %v", err)
	}
	if len(consumer.calls) != 1 {
		t.Fatalf("expected one partition due limit=1, got calls=%d", len(consumer.calls))
	}
	if consumer.calls[0].partition != 0 {
		t.Fatalf("expected first sorted partition=0, got %d", consumer.calls[0].partition)
	}

	executeCfg := cfg
	executeCfg.execute = true
	if err := runReplay(context.Background(), executeCfg, client, consumer, nil); err == nil {
		t.Fatal("expected execute mode to require producer")
	}

	emptyClient := &stubOffsetClient{partitions: nil}
	if err := runReplay(context.Background(), cfg, emptyClient, consumer, nil); err != nil {
		t.Fatalf("expected nil error for empty partitions, got %v", err)
	}
}

func TestRun_UsesDependencies(t *testing.T) {
	oldDeps := newReplayDependencies
	defer func() { newReplayDependencies = oldDeps }()

	cfg := config{sourceTopic: "oms.dlq", targetTopic: "oms.order.events", limit: 1, idleTimeout: 20 * time.Millisecond}

	newReplayDependencies = func(config) (offsetClient, partitionConsumerSource, replayProducer, error) {
		return nil, nil, nil, errors.New("deps failed")
	}
	if err := run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "deps failed") {
		t.Fatalf("expected deps error, got %v", err)
	}

	client := &stubOffsetClient{
		partitions: []int32{0},
		offsets: map[int32]offsetRange{
			0: {oldest: 0, newest: 2},
		},
	}
	consumer := &stubPartitionConsumerSource{
		consumers: map[int32]partitionConsumer{
			0: closedPartitionConsumer([]*sarama.ConsumerMessage{{
				Partition: 0,
				Offset:    0,
				Value:     []byte(`{"original_topic":"oms.order.events","original_key":"order-1","original_value":"{\"id\":\"evt-1\"}"}`),
			}}),
		},
	}
	producer := &stubReplayProducer{}

	newReplayDependencies = func(config) (offsetClient, partitionConsumerSource, replayProducer, error) {
		return client, consumer, producer, nil
	}
	if err := run(context.Background(), cfg); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !client.closed || !consumer.closed || !producer.closed {
		t.Fatalf("expected all deps to be closed: client=%v consumer=%v producer=%v", client.closed, consumer.closed, producer.closed)
	}
}

func TestMain_SuccessWithStubbedDeps(t *testing.T) {
	oldDeps := newReplayDependencies
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		newReplayDependencies = oldDeps
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	client := &stubOffsetClient{
		partitions: []int32{0},
		offsets: map[int32]offsetRange{
			0: {oldest: 0, newest: 2},
		},
	}
	consumer := &stubPartitionConsumerSource{
		consumers: map[int32]partitionConsumer{
			0: closedPartitionConsumer([]*sarama.ConsumerMessage{{
				Partition: 0,
				Offset:    0,
				Value:     []byte(`{"original_topic":"oms.order.events","original_key":"order-1","original_value":"{\"id\":\"evt-1\"}"}`),
			}}),
		},
	}
	newReplayDependencies = func(config) (offsetClient, partitionConsumerSource, replayProducer, error) {
		return client, consumer, nil, nil
	}

	os.Args = []string{"dlq-reprocess", "-brokers=broker:9092", "-source-topic=oms.dlq", "-target-topic=oms.order.events", "-limit=1", "-idle-timeout=50ms"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	main()
}

func TestFailExits(t *testing.T) {
	if os.Getenv("DLQ_TEST_FAIL_EXIT") == "1" {
		fail("boom")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFailExits")
	cmd.Env = append(os.Environ(), "DLQ_TEST_FAIL_EXIT=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with error")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got %v", err)
	}
}

func withFlagArgs(t *testing.T, args []string, fn func()) {
	t.Helper()

	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	os.Args = append([]string{"dlq-reprocess"}, args...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	fn()
}

type offsetRange struct {
	oldest int64
	newest int64
}

type stubOffsetClient struct {
	partitions    []int32
	partitionsErr error
	offsets       map[int32]offsetRange
	offsetErr     map[int32]error
	closed        bool
}

func (s *stubOffsetClient) GetOffset(_ string, partition int32, marker int64) (int64, error) {
	if err, ok := s.offsetErr[partition]; ok {
		return 0, err
	}

	r := s.offsets[partition]
	switch marker {
	case sarama.OffsetOldest:
		return r.oldest, nil
	case sarama.OffsetNewest:
		return r.newest, nil
	default:
		return 0, fmt.Errorf("unsupported marker %d", marker)
	}
}

func (s *stubOffsetClient) Partitions(string) ([]int32, error) {
	if s.partitionsErr != nil {
		return nil, s.partitionsErr
	}
	return append([]int32(nil), s.partitions...), nil
}

func (s *stubOffsetClient) Close() error {
	s.closed = true
	return nil
}

type consumeCall struct {
	partition int32
	offset    int64
}

type stubPartitionConsumerSource struct {
	consumers  map[int32]partitionConsumer
	consumeErr error
	calls      []consumeCall
	closed     bool
}

func (s *stubPartitionConsumerSource) ConsumePartition(_ string, partition int32, offset int64) (partitionConsumer, error) {
	s.calls = append(s.calls, consumeCall{partition: partition, offset: offset})
	if s.consumeErr != nil {
		return nil, s.consumeErr
	}
	pc, ok := s.consumers[partition]
	if !ok {
		return nil, fmt.Errorf("partition %d not configured", partition)
	}
	return pc, nil
}

func (s *stubPartitionConsumerSource) Close() error {
	s.closed = true
	return nil
}

type stubPartitionConsumer struct {
	messages chan *sarama.ConsumerMessage
	errors   chan *sarama.ConsumerError
	closed   bool
}

func (s *stubPartitionConsumer) Messages() <-chan *sarama.ConsumerMessage { return s.messages }
func (s *stubPartitionConsumer) Errors() <-chan *sarama.ConsumerError     { return s.errors }
func (s *stubPartitionConsumer) Close() error {
	s.closed = true
	return nil
}

func closedPartitionConsumer(messages []*sarama.ConsumerMessage) *stubPartitionConsumer {
	msgCh := make(chan *sarama.ConsumerMessage, len(messages))
	errCh := make(chan *sarama.ConsumerError)
	for _, msg := range messages {
		msgCh <- msg
	}
	close(msgCh)
	close(errCh)
	return &stubPartitionConsumer{messages: msgCh, errors: errCh}
}

type stubReplayProducer struct {
	sendErr error
	calls   int
	closed  bool
	lastMsg *sarama.ProducerMessage
}

func (s *stubReplayProducer) SendMessage(msg *sarama.ProducerMessage) (int32, int64, error) {
	s.calls++
	s.lastMsg = msg
	if s.sendErr != nil {
		return 0, 0, s.sendErr
	}
	return 0, int64(s.calls), nil
}

func (s *stubReplayProducer) Close() error {
	s.closed = true
	return nil
}
