package bus

import (
	"context"
	"crdb/ai_ml/fraud_detection/app/pkg/models"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
)

type PulsarBus struct {
	consumer pulsar.Consumer
	messages chan pulsar.ConsumerMessage
	workers  int
}

func NewPulsarBus(broker string, subscription, topic string) (*PulsarBus, error) {
	bus := PulsarBus{
		messages: make(chan pulsar.ConsumerMessage, 10),
		workers:  4,
	}

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:               broker,
		OperationTimeout:  30 * time.Second,
		ConnectionTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("creating pulsar client: %v", err)
	}

	options := pulsar.ConsumerOptions{
		Topic:                          topic,
		SubscriptionName:               subscription,
		Type:                           pulsar.Shared,
		ReceiverQueueSize:              1000,
		EnableDefaultNackBackoffPolicy: true,
		MessageChannel:                 bus.messages,
	}

	bus.consumer, err = client.Subscribe(options)
	if err != nil {
		log.Fatal(err)
	}

	return &bus, nil
}

func (b *PulsarBus) Run(ctx context.Context, f func(context.Context, models.Message) error) {
	defer b.consumer.Close()

	workChan := make(chan pulsar.Message, b.workers*10)

	var wg sync.WaitGroup

	for i := 0; i < b.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			b.worker(ctx, workerID, workChan, f)
		}(i)
	}

	var messageBatch []pulsar.Message
	batchSize := 50
	messageFlushTick := time.NewTicker(time.Millisecond * 100)
	defer messageFlushTick.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("agent shutting down")

			// Flush on shut down.
			for _, m := range messageBatch {
				workChan <- m
			}
			close(workChan)
			wg.Wait()
			return

		case msg := <-b.messages:
			messageBatch = append(messageBatch, msg)

			// Flush on batch full.
			if len(messageBatch) >= batchSize {
				for _, m := range messageBatch {
					workChan <- m
				}
				messageBatch = messageBatch[:0]
			}

		// Flush on timer tick.
		case <-messageFlushTick.C:
			if len(messageBatch) > 0 {
				for _, m := range messageBatch {
					workChan <- m
				}
				messageBatch = messageBatch[:0]
			}
		}
	}
}

func (b *PulsarBus) worker(ctx context.Context, id int, workChan <-chan pulsar.Message, f func(context.Context, models.Message) error) {
	log.Printf("worker %d started", id)
	defer log.Printf("worker %d stopped", id)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-workChan:
			if !ok {
				return
			}
			if err := b.handleMessage(ctx, msg, f); err != nil {
				log.Printf("worker %d: error processing message: %v", id, err)
			}
		}
	}
}

func (b *PulsarBus) handleMessage(ctx context.Context, m pulsar.Message, f func(context.Context, models.Message) error) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var msg models.Message
	if err := json.Unmarshal(m.Payload(), &msg); err != nil {
		b.consumer.Nack(m)
		return fmt.Errorf("parsing payload: %w", err)
	}

	if err := f(ctx, msg); err != nil {
		b.consumer.Nack(m)
		return fmt.Errorf("handling message: %w", err)
	}

	if err := b.consumer.Ack(m); err != nil {
		b.consumer.Nack(m)
		return fmt.Errorf("acknowledging message: %w", err)
	}

	return nil
}
