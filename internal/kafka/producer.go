package kafka

import (
	"ax-ingestion/internal/config"
	"ax-ingestion/pkg/models"
	"context"
	"encoding/json"
	"fmt"
	"time"

	kafkalib "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Producer struct {
	writer *kafkalib.Writer
}

func NewProducer(cfg *config.Config) *Producer {
	w := &kafkalib.Writer{
		Addr:                   kafkalib.TCP(cfg.KafkaURL),
		Topic:                  cfg.Topic,
		Balancer:               &kafkalib.LeastBytes{},
		AllowAutoTopicCreation: true,
		BatchSize:              100,
		BatchTimeout:           10 * time.Millisecond,
		Async:                  true,

		// 🚨 ASYNC DELIVERY GUARANTEE: Catch dropped messages
		Completion: func(messages []kafkalib.Message, err error) {
			if err != nil {
				config.Logger.Error("Kafka async batch delivery failed",
					zap.Error(err),
					zap.Int("message_count", len(messages)),
				)
				// In a full enterprise system, you would push these failed
				// messages to a Dead Letter Queue (DLQ) Redis list here.
			}
		},
	}

	config.Logger.Info("Kafka Producer initialized",
		zap.String("topic", cfg.Topic),
		zap.String("url", cfg.KafkaURL),
	)

	return &Producer{writer: w}
}

func (p *Producer) Publish(ctx context.Context, payload models.TelemetryPayload) error {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry: %w", err)
	}

	err = p.writer.WriteMessages(ctx, kafkalib.Message{
		Key:   []byte(payload.TenantID),
		Value: bytes,
	})

	if err != nil {
		return fmt.Errorf("failed to buffer to kafka: %w", err)
	}
	return nil
}

func (p *Producer) Close() error {
	config.Logger.Info("Flushing remaining Kafka messages...")
	return p.writer.Close()
}
