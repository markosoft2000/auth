package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Config struct {
	BootstrapServers      string
	ClientID              string
	Topic                 string
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

	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
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

	// Delivery report handler for produced messages
	go func() {
		log := log.With(slog.String("op", op))

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
	}()

	return &PubSub{
		producer: p,
		cfg:      cfg,
		log:      log,
	}, nil
}

func (ps *PubSub) Stop() {
	// Wait up to 15 seconds for outstanding messages to be delivered
	ps.producer.Flush(15 * 1000)
	ps.producer.Close()
}

func (ps *PubSub) Produce(key, data []byte) error {
	err := ps.producer.Produce(
		&kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &ps.cfg.Topic,
				Partition: kafka.PartitionAny,
			},
			Key:   key,
			Value: data,
		},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue kafka message: %w", err)
	}

	return nil
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
