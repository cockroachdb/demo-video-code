package main

import (
	"context"
	"crdb/ai_ml/fraud_detection/app/pkg/agents"
	"crdb/ai_ml/fraud_detection/app/pkg/bus"
	"crdb/ai_ml/fraud_detection/app/pkg/models"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/codingconcepts/env"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	log.SetFlags(0)

	var e models.Environment
	if err := env.Set(&e); err != nil {
		log.Fatalf("setting variables from environment: %v", err)
	}

	b := bus.NewKafkaBus([]string{e.BusBroker}, e.GroupID)

	db, err := sql.Open(e.DatabaseDriver, e.DatabaseURL)
	if err != nil {
		log.Fatalf("opening database connection: %v", err)
	}

	llm := openai.NewClient(option.WithAPIKey(e.OpenAIAPIKey))

	dependencies := agents.NewDependencies(b, db, llm, e.Region, e.Topic)

	var a agents.Agent

	switch e.AgentType {
	case string(models.AgentTypeAnomalyDetection):
		a = agents.NewAnomalyDetection(dependencies)
	case string(models.AgentTypeReasoning):
		a = agents.NewReasoning(dependencies)
	case string(models.AgentTypeNotification):
		a, err = agents.NewNotification(dependencies)
		if err != nil {
			log.Fatalf("error creating agent: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Gracefully handle shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		a.Run(ctx)
	}()

	<-sigChan
	cancel()
}
