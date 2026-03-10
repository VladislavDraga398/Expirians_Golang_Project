package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/IBM/sarama"
	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/messaging/kafka"
)

const (
	defaultReplayLimit = 100
	defaultIdleTimeout = 2 * time.Second
)

type config struct {
	brokers     []string
	sourceTopic string
	targetTopic string
	limit       int
	execute     bool
	fromNewest  bool
	idleTimeout time.Duration
}

type replayMessage struct {
	topic string
	key   string
	value []byte
}

type consumerDLQPayload struct {
	OriginalTopic string `json:"original_topic"`
	OriginalKey   string `json:"original_key"`
	OriginalValue string `json:"original_value"`
}

type outboxEnvelope struct {
	ID            string          `json:"id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
}

type outboxDLQPayload struct {
	OutboxID      string          `json:"outbox_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
}

type replayEnvelope struct {
	ID            string          `json:"id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	PublishedAt   time.Time       `json:"published_at"`
}

type offsetClient interface {
	GetOffset(topic string, partition int32, time int64) (int64, error)
	Partitions(topic string) ([]int32, error)
	Close() error
}

type partitionConsumer interface {
	Messages() <-chan *sarama.ConsumerMessage
	Errors() <-chan *sarama.ConsumerError
	Close() error
}

type partitionConsumerSource interface {
	ConsumePartition(topic string, partition int32, offset int64) (partitionConsumer, error)
	Close() error
}

type replayProducer interface {
	SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error)
	Close() error
}

type saramaConsumerAdapter struct {
	consumer sarama.Consumer
}

func (a saramaConsumerAdapter) ConsumePartition(topic string, partition int32, offset int64) (partitionConsumer, error) {
	pc, err := a.consumer.ConsumePartition(topic, partition, offset)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

func (a saramaConsumerAdapter) Close() error {
	if a.consumer == nil {
		return nil
	}
	return a.consumer.Close()
}

var newReplayDependencies = func(cfg config) (offsetClient, partitionConsumerSource, replayProducer, error) {
	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Return.Errors = true

	client, err := sarama.NewClient(cfg.brokers, consumerConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create kafka client: %w", err)
	}

	rawConsumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		_ = client.Close()
		return nil, nil, nil, fmt.Errorf("create kafka consumer: %w", err)
	}
	consumer := saramaConsumerAdapter{consumer: rawConsumer}

	if !cfg.execute {
		return client, consumer, nil, nil
	}

	producerConfig := sarama.NewConfig()
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 5
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.Compression = sarama.CompressionSnappy
	producerConfig.Producer.Idempotent = true
	producerConfig.Net.MaxOpenRequests = 1

	producer, err := sarama.NewSyncProducer(cfg.brokers, producerConfig)
	if err != nil {
		_ = consumer.Close()
		_ = client.Close()
		return nil, nil, nil, fmt.Errorf("create kafka producer: %w", err)
	}

	return client, consumer, producer, nil
}

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetLevel(log.InfoLevel)

	cfg, err := readConfig()
	if err != nil {
		fail("%v", err)
	}

	if err := run(context.Background(), cfg); err != nil {
		fail("dlq replay failed: %v", err)
	}
}

func readConfig() (config, error) {
	var (
		brokersRaw string
		cfg        config
	)

	flag.StringVar(&brokersRaw, "brokers", "", "Kafka brokers as comma-separated list (fallback: KAFKA_BROKERS)")
	flag.StringVar(&cfg.sourceTopic, "source-topic", kafka.TopicDeadLetterQueue, "DLQ source topic")
	flag.StringVar(&cfg.targetTopic, "target-topic", kafka.TopicOrderEvents, "target topic for replay")
	flag.IntVar(&cfg.limit, "limit", defaultReplayLimit, "max number of messages to scan/replay")
	flag.BoolVar(&cfg.execute, "execute", false, "execute replay; default is dry-run")
	flag.BoolVar(&cfg.fromNewest, "from-newest", false, "scan latest messages first (bounded by limit)")
	flag.DurationVar(&cfg.idleTimeout, "idle-timeout", defaultIdleTimeout, "idle timeout per partition")
	flag.Parse()

	if strings.TrimSpace(brokersRaw) == "" {
		brokersRaw = os.Getenv("KAFKA_BROKERS")
	}

	cfg.brokers = parseBrokers(brokersRaw)
	if len(cfg.brokers) == 0 {
		return config{}, fmt.Errorf("kafka brokers are required (-brokers or KAFKA_BROKERS)")
	}
	if strings.TrimSpace(cfg.sourceTopic) == "" {
		return config{}, fmt.Errorf("source-topic is required")
	}
	if strings.TrimSpace(cfg.targetTopic) == "" {
		return config{}, fmt.Errorf("target-topic is required")
	}
	if cfg.limit <= 0 {
		return config{}, fmt.Errorf("limit must be > 0")
	}
	if cfg.idleTimeout <= 0 {
		return config{}, fmt.Errorf("idle-timeout must be > 0")
	}

	return cfg, nil
}

func parseBrokers(raw string) []string {
	chunks := strings.Split(raw, ",")
	brokers := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		broker := strings.TrimSpace(chunk)
		if broker == "" {
			continue
		}
		brokers = append(brokers, broker)
	}
	return brokers
}

func run(ctx context.Context, cfg config) error {
	log.WithFields(log.Fields{
		"source_topic": cfg.sourceTopic,
		"target_topic": cfg.targetTopic,
		"limit":        cfg.limit,
		"execute":      cfg.execute,
		"from_newest":  cfg.fromNewest,
	}).Info("starting dlq replay")

	client, consumer, producer, err := newReplayDependencies(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if producer != nil {
			_ = producer.Close()
		}
		if consumer != nil {
			_ = consumer.Close()
		}
		if client != nil {
			_ = client.Close()
		}
	}()

	return runReplay(ctx, cfg, client, consumer, producer)
}

func runReplay(ctx context.Context, cfg config, client offsetClient, consumer partitionConsumerSource, producer replayProducer) error {
	if client == nil || consumer == nil {
		return fmt.Errorf("kafka client and consumer are required")
	}
	if cfg.execute && producer == nil {
		return fmt.Errorf("producer is required in execute mode")
	}

	partitions, err := client.Partitions(cfg.sourceTopic)
	if err != nil {
		return fmt.Errorf("get partitions for topic %s: %w", cfg.sourceTopic, err)
	}
	if len(partitions) == 0 {
		log.WithField("topic", cfg.sourceTopic).Warn("source topic has no partitions")
		return nil
	}
	sort.Slice(partitions, func(i, j int) bool { return partitions[i] < partitions[j] })

	var (
		processed int
		replayed  int
		skipped   int
	)

	for _, partition := range partitions {
		if processed >= cfg.limit {
			break
		}

		remaining := cfg.limit - processed
		stats, err := processPartition(ctx, consumer, client, producer, cfg, partition, remaining)
		if err != nil {
			return err
		}

		processed += stats.processed
		replayed += stats.replayed
		skipped += stats.skipped
	}

	mode := "dry-run"
	if cfg.execute {
		mode = "execute"
	}

	log.WithFields(log.Fields{
		"mode":      mode,
		"processed": processed,
		"replayed":  replayed,
		"skipped":   skipped,
	}).Info("dlq replay finished")

	return nil
}

type partitionStats struct {
	processed int
	replayed  int
	skipped   int
}

func processPartition(
	ctx context.Context,
	consumer partitionConsumerSource,
	client offsetClient,
	producer replayProducer,
	cfg config,
	partition int32,
	limit int,
) (partitionStats, error) {
	var stats partitionStats
	if limit <= 0 {
		return stats, nil
	}

	oldest, err := client.GetOffset(cfg.sourceTopic, partition, sarama.OffsetOldest)
	if err != nil {
		return stats, fmt.Errorf("get oldest offset for partition %d: %w", partition, err)
	}
	newest, err := client.GetOffset(cfg.sourceTopic, partition, sarama.OffsetNewest)
	if err != nil {
		return stats, fmt.Errorf("get newest offset for partition %d: %w", partition, err)
	}
	if newest <= oldest {
		return stats, nil
	}

	startOffset := oldest
	if cfg.fromNewest {
		startOffset = newest - int64(limit)
		if startOffset < oldest {
			startOffset = oldest
		}
	}

	partitionConsumer, err := consumer.ConsumePartition(cfg.sourceTopic, partition, startOffset)
	if err != nil {
		return stats, fmt.Errorf("consume partition %d: %w", partition, err)
	}
	defer func() { _ = partitionConsumer.Close() }()

	endOffset := newest
	idleTimer := time.NewTimer(cfg.idleTimeout)
	defer idleTimer.Stop()

	for stats.processed < limit {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		case err := <-partitionConsumer.Errors():
			if err != nil {
				return stats, fmt.Errorf("partition %d consumer error: %w", partition, err)
			}
		case msg, ok := <-partitionConsumer.Messages():
			if !ok || msg == nil {
				return stats, nil
			}

			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(cfg.idleTimeout)

			if msg.Offset >= endOffset {
				return stats, nil
			}

			replayMsg, ok, err := extractReplayMessage(msg, cfg.targetTopic)
			if err != nil {
				stats.processed++
				stats.skipped++
				log.WithError(err).WithFields(log.Fields{
					"partition": msg.Partition,
					"offset":    msg.Offset,
				}).Warn("skip unsupported dlq message")
				continue
			}
			if !ok {
				stats.processed++
				stats.skipped++
				continue
			}

			if cfg.execute {
				if err := publishReplay(producer, replayMsg); err != nil {
					return stats, fmt.Errorf("publish replay message: %w", err)
				}
				stats.replayed++
			} else {
				log.WithFields(log.Fields{
					"partition":    msg.Partition,
					"offset":       msg.Offset,
					"target_topic": replayMsg.topic,
					"key":          replayMsg.key,
				}).Info("dlq replay candidate")
				stats.replayed++
			}

			stats.processed++

			if msg.Offset+1 >= endOffset {
				return stats, nil
			}
		case <-idleTimer.C:
			return stats, nil
		}
	}

	return stats, nil
}

func publishReplay(producer replayProducer, msg replayMessage) error {
	if producer == nil {
		return fmt.Errorf("producer is nil")
	}

	producerMessage := &sarama.ProducerMessage{
		Topic:     msg.topic,
		Key:       sarama.StringEncoder(msg.key),
		Value:     sarama.ByteEncoder(msg.value),
		Timestamp: time.Now().UTC(),
	}

	_, _, err := producer.SendMessage(producerMessage)
	return err
}

func extractReplayMessage(msg *sarama.ConsumerMessage, defaultTopic string) (replayMessage, bool, error) {
	var consumerPayload consumerDLQPayload
	if err := json.Unmarshal(msg.Value, &consumerPayload); err == nil && consumerPayload.OriginalValue != "" {
		targetTopic := strings.TrimSpace(consumerPayload.OriginalTopic)
		if targetTopic == "" {
			targetTopic = defaultTopic
		}
		return replayMessage{
			topic: targetTopic,
			key:   consumerPayload.OriginalKey,
			value: []byte(consumerPayload.OriginalValue),
		}, true, nil
	}

	var envelope outboxEnvelope
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		return replayMessage{}, false, nil
	}
	if len(envelope.Payload) == 0 {
		return replayMessage{}, false, nil
	}

	var dlqPayload outboxDLQPayload
	if err := json.Unmarshal(envelope.Payload, &dlqPayload); err != nil {
		return replayMessage{}, false, fmt.Errorf("decode outbox dlq payload: %w", err)
	}
	if len(dlqPayload.Payload) == 0 {
		return replayMessage{}, false, fmt.Errorf("outbox dlq payload does not contain original event payload")
	}

	replay := replayEnvelope{
		ID:            firstNonEmpty(dlqPayload.OutboxID, envelope.ID),
		AggregateType: firstNonEmpty(dlqPayload.AggregateType, envelope.AggregateType),
		AggregateID:   firstNonEmpty(dlqPayload.AggregateID, envelope.AggregateID),
		EventType:     firstNonEmpty(dlqPayload.EventType, envelope.EventType),
		Payload:       dlqPayload.Payload,
		PublishedAt:   time.Now().UTC(),
	}
	encoded, err := json.Marshal(replay)
	if err != nil {
		return replayMessage{}, false, fmt.Errorf("encode replay envelope: %w", err)
	}

	key := replay.AggregateID
	if key == "" {
		key = replay.ID
	}

	return replayMessage{
		topic: defaultTopic,
		key:   key,
		value: encoded,
	}, true, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
