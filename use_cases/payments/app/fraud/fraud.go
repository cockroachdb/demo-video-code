package main

import (
	"context"
	"crdb-use-cases/_code/connect"
	"encoding/json"
	"fmt"
	"log"

	"github.com/samber/lo"
	"github.com/segmentio/kafka-go"
)

func main() {
	log.SetFlags(0)

	reader := connect.MustConnectKafkaReader("KAFKA_URL", "fraud_check")
	defer reader.Close()

	log.Println(reader.Config())

	fraudCheckEvents(reader)
}

type fraudCheckEvent struct {
	ID     string `json:"id"`
	Amount struct {
		Currency string  `json:"currency"`
		Value    float64 `json:"value"`
	} `json:"amount"`
}

func fraudCheckEvents(reader *kafka.Reader) {
	log.Println("listening for events")

	for {
		var e fraudCheckEvent
		commit, err := readAndParse(reader, &e)
		if err != nil {
			log.Printf("error processing fraud check event: %v", err)
			continue
		}

		// Perform fraud check.
		if err = performFraudCheck(e); err != nil {
			log.Println(err)
		}

		// Commit to remove from event queue.
		if err = commit(); err != nil {
			log.Printf("error committing fraud check event: %v", err)
		}
	}
}

func performFraudCheck(e fraudCheckEvent) error {
	if e.Amount.Value < 10000 {
		return nil
	}

	if lo.Contains([]string{"VEF", "IRR", "SDG", "PRB", "CUP"}, e.Amount.Currency) {
		return fmt.Errorf("large transaction of %q flagged as suspicious", e.Amount.Currency)
	}

	return nil
}

type commitFunc func() error

func readAndParse(reader *kafka.Reader, v any) (commitFunc, error) {
	msg, err := reader.FetchMessage(context.Background())
	if err != nil {
		return nil, fmt.Errorf("reading message: %w", err)
	}

	if err = json.Unmarshal(msg.Value, v); err != nil {
		return nil, fmt.Errorf("unmarshalling message: %w", err)
	}

	commit := func() error {
		return reader.CommitMessages(context.Background(), msg)
	}

	return commit, nil
}
