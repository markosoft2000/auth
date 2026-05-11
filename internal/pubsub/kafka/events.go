package kafka

import (
	"fmt"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

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
