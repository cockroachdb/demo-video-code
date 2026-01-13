package agents

import (
	"context"
	"crdb/ai_ml/fraud_detection/app/pkg/bus"
	"crdb/ai_ml/fraud_detection/app/pkg/models"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/openai/openai-go/v3"
)

type Agent interface {
	Name() string
	Run(ctx context.Context)
	Process(ctx context.Context, msg models.Message) error
}

type Dependencies struct {
	Bus    *bus.KafkaBus
	DB     *sql.DB
	LLM    openai.Client
	Region string
	Topic  string
}

func NewDependencies(bus *bus.KafkaBus, db *sql.DB, llm openai.Client, region, topic string) *Dependencies {
	return &Dependencies{
		Bus:    bus,
		DB:     db,
		LLM:    llm,
		Region: region,
		Topic:  topic,
	}
}
