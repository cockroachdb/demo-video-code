package bus

import (
	"context"
	"crdb/ai_ml/fraud_detection/app/pkg/models"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaBus struct {
	brokers []string
	groupID string
}

func NewKafkaBus(brokers []string, groupID string) *KafkaBus {
	return &KafkaBus{
		brokers: brokers,
		groupID: groupID,
	}
}

func (b *KafkaBus) NewConsumer(topic string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: b.brokers,
			GroupID: b.groupID,
			Topic:   topic,
		}),
	}
}

func (b *KafkaBus) Close() error { return nil }

func (b *KafkaBus) Drain(ctx context.Context) error { return nil }

type Consumer struct{ reader *kafka.Reader }

func (c *Consumer) Run(ctx context.Context, f func(context.Context, models.Message) error) {
	defer c.reader.Close()

	for {
		if err := c.handleMessage(f); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				continue
			}

			log.Printf("error handling message: %v", err)
		}
	}
}

func (c *Consumer) handleMessage(f func(context.Context, models.Message) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	m, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return fmt.Errorf("fetching message: %w", err)
	}

	mm := models.Message{
		Key:     []string{string(m.Key)},
		Topic:   m.Topic,
		Payload: m.Value,
	}

	if err := f(ctx, mm); err != nil {
		return fmt.Errorf("handing message: %w", err)
	}

	if err := c.reader.CommitMessages(ctx, m); err != nil {
		return fmt.Errorf("committing message: %w", err)
	}

	return nil
}
