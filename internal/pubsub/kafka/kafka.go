package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Config struct {
	BootstrapServers      string
	ClientID              string
	BatchNumMessages      int
	LingerMs              int
	CompressionType       string
	Acks                  string
	EnableIdempotence     bool
	Retries               int
	RetryBackoffMs        int
	MessageTimeoutMs      int
	SocketKeepaliveEnable bool
	QueueBufferingMaxMsgs int

	ProducerMaxRetries   int
	ProducerRetryBackoff time.Duration

	TopicUserActivity string
	TopicAppKey       string
}

type PubSub struct {
	producer *kafka.Producer
	cfg      Config
	log      *slog.Logger
}

func New(
	ctx context.Context,
	log *slog.Logger,
	cfg Config,
) (*PubSub, error) {
	const op = "pubsub.kafka.New"

	if cfg.ProducerMaxRetries < 1 {
		cfg.ProducerMaxRetries = 1
	}

	if cfg.ProducerRetryBackoff < 0 {
		cfg.ProducerRetryBackoff = 0
	}

	host, err := os.Hostname()
	if err != nil {
		host = "unknown_hostname"
	}

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": cfg.BootstrapServers,
		"client.id":         cfg.ClientID + host,

		// Batching
		"batch.num.messages": cfg.BatchNumMessages,
		"linger.ms":          cfg.LingerMs,
		"compression.type":   cfg.CompressionType,

		// Reliability
		"acks":                    cfg.Acks,
		"enable.idempotence":      cfg.EnableIdempotence,
		"retries":                 cfg.Retries,
		"retry.backoff.ms":        cfg.RetryBackoffMs,
		"message.timeout.ms":      cfg.MessageTimeoutMs,
		"socket.keepalive.enable": cfg.SocketKeepaliveEnable,

		// Memory
		"queue.buffering.max.messages": cfg.QueueBufferingMaxMsgs,
	})
	if err != nil {
		return nil, fmt.Errorf("%s new kafka producer failed: %w", op, err)
	}

	const pingTimeout = 1000 // ms
	_, err = p.GetMetadata(nil, true, pingTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s: could not ping kafka: %w", op, err)
	}

	go eventListener(ctx, log.With(slog.String("op", op)), p)

	return &PubSub{
		producer: p,
		cfg:      cfg,
		log:      log,
	}, nil
}

// eventListener provides delivery report handler for produced messages
func eventListener(ctx context.Context, log *slog.Logger, p *kafka.Producer) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-p.Events():
			if !ok {
				return
			}
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					log.Error(
						"Kafka: Delivery failed",
						slog.String("topic", *ev.TopicPartition.Topic),
						slog.Any("error", ev.TopicPartition.Error),
					)
				} else {
					log.Debug(
						"Kafka: Delivered message",
						slog.String("topic", *ev.TopicPartition.Topic),
						slog.Int("partition", int(ev.TopicPartition.Partition)),
					)
				}
			}
		}
	}
}

// Ping checks if the Kafka cluster is reachable by requesting metadata.
func (ps *PubSub) Ping(ctx context.Context) error {
	const pingTimeout = 1000 // ms

	_, err := ps.producer.GetMetadata(nil, true, pingTimeout)
	if err != nil {
		return fmt.Errorf("kafka ping failed: %w", err)
	}

	return nil
}

func (ps *PubSub) Stop() {
	// Wait up to 15 seconds for outstanding messages to be delivered
	ps.producer.Flush(15 * 1000)
	ps.producer.Close()
}

func (ps *PubSub) ProduceAppKeyEvent(key, data []byte) error {
	op := "auth.Kafka.ProduceAppKeyEvent"

	log := ps.log.With(slog.String("op", op))

	err := ps.withRetry(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &ps.cfg.TopicAppKey,
			Partition: kafka.PartitionAny,
		},
		Key:   key,
		Value: data,
	})
	if err != nil {
		log.Error("failed to enqueue kafka message", slog.Any("error", err))

		return fmt.Errorf("failed to enqueue kafka message: %w", err)
	}

	return nil
}

func (ps *PubSub) ProduceUserActivityEvent(key, data []byte) error {
	op := "auth.Kafka.ProduceUserActivityEvent"

	log := ps.log.With(slog.String("op", op))

	err := ps.withRetry(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &ps.cfg.TopicUserActivity,
			Partition: kafka.PartitionAny,
		},
		Key:   key,
		Value: data,
	})
	if err != nil {
		log.Error("failed to enqueue kafka message", slog.Any("error", err))

		return fmt.Errorf("failed to enqueue kafka message: %w", err)
	}

	return nil
}

func (ps *PubSub) withRetry(msg *kafka.Message) error {
	const op = "auth.Kafka.withRetry"

	retryBackoff := ps.cfg.ProducerRetryBackoff * time.Millisecond

	var err error
	for i := 0; i < ps.cfg.ProducerMaxRetries; i++ {
		err = ps.producer.Produce(msg, nil)
		if err == nil {
			return nil
		}

		// If the internal librdkafka queue is full, we retry with a short backoff.
		if kerr, ok := err.(kafka.Error); ok && kerr.Code() == kafka.ErrQueueFull {
			ps.log.With(slog.String("op", op)).Warn("kafka local queue full, retrying",
				slog.String("topic", *msg.TopicPartition.Topic),
				slog.Int("partition", int(msg.TopicPartition.Partition)),
				slog.Int("attempt", i+1),
				slog.Duration("backoff", retryBackoff),
			)

			if i < ps.cfg.ProducerMaxRetries-1 {
				time.Sleep(retryBackoff)
				retryBackoff *= 2
			}

			continue
		}

		// For any other non-transient error, we stop immediately.
		break
	}

	return fmt.Errorf("failed to enqueue kafka message after retries: %w", err)
}
