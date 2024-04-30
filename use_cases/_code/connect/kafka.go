package connect

import (
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

func MustConnectKafkaReader(urlEnvVar, topic string) *kafka.Reader {
	url, ok := os.LookupEnv(urlEnvVar)
	if !ok {
		log.Fatalf("missing %q env var", urlEnvVar)
	}

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{url},
		GroupID:     uuid.NewString(),
		Topic:       topic,
		StartOffset: kafka.LastOffset,
	})

	return kafkaReader
}
