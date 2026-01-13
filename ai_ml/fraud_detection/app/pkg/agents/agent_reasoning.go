package agents

import (
	"context"
	"crdb/ai_ml/fraud_detection/app/pkg/models"
	"fmt"
	"log"

	"github.com/openai/openai-go/v3"
)

type Reasoning struct {
	d *Dependencies
}

func NewReasoning(d *Dependencies) *Reasoning {
	return &Reasoning{
		d: d,
	}
}

// Run blocks forever.
func (a *Reasoning) Run(ctx context.Context) {
	c := a.d.Bus.NewConsumer(a.d.Topic)
	c.Run(ctx, a.Process)
}

func (a *Reasoning) Name() string {
	return "agent.reasoning"
}

func (a *Reasoning) Process(ctx context.Context, m models.Message) error {
	var msg models.AnomalyMessage
	if err := models.ParsePayload(m, &msg); err != nil {
		return fmt.Errorf("parsing anomaly message: %w", err)
	}
	log.Printf("anomaly received")

	// Fetch purchase context.
	context, err := a.fetchContext(ctx, msg)
	if err != nil {
		return fmt.Errorf("fetching context for reasoning: %w", err)
	}
	log.Printf("purchase context fetched")

	resp, err := a.performLLMRequest(context.String())
	if err != nil {
		return fmt.Errorf("performing reasoning: %w", err)
	}
	log.Printf("llm response received")

	// Store the response alongside the anomaly.
	if err = a.storeNotification(ctx, resp, msg); err != nil {
		return fmt.Errorf("storing reasoning: %w", err)
	}
	log.Printf("llm response stored")

	return nil
}

type llmContext struct {
	purchaseID            string
	amountContribution    float64
	hourOfDayContribution float64
	locationContribution  float64
}

func (ctx llmContext) String() string {
	const messageFormat = `A customer purchase (id: %s) has been deemed to be
												 anomalous.

												 Compose a very brief message to them explaining why
												 their purchase has been flagged, sharing just the
												 primary reason for the flagging (in a clear and human
												 way; nothing too formal and don't use business or
												 technical lingo):
												 
												 For context, here are the things that contributed to
												 the detection:

												 - Purchase amount contributed %.4f to the detection
												 - Time of day contributed %.4f to the detection
												 - Location contributed %.4f to the detection
												 
												 Our company name is "ACME Corp."
												 Don't use a placeholder for their name.`

	return fmt.Sprintf(
		ctx.purchaseID,
		messageFormat,
		ctx.amountContribution,
		ctx.hourOfDayContribution,
		ctx.locationContribution,
	)
}

func (a *Reasoning) performLLMRequest(prompt string) (string, error) {
	chatCompletion, err := a.d.LLM.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Model: openai.ChatModelGPT4o,
	})

	if len(chatCompletion.Choices) == 0 {
		return "", fmt.Errorf("empty chat completion")
	}

	return chatCompletion.Choices[0].Message.Content, err
}

func (a *Reasoning) fetchContext(ctx context.Context, msg models.AnomalyMessage) (llmContext, error) {
	const stmt = `SELECT dimension_name, contribution_pct FROM purchase_distance_breakdown($1, $2)`

	rows, err := a.d.DB.QueryContext(ctx, stmt, msg.PurchaseID, msg.CustomerID)
	if err != nil {
		return llmContext{}, fmt.Errorf("making query: %w", err)
	}

	var name string
	var percent float64
	context := llmContext{
		purchaseID: msg.PurchaseID,
	}

	for rows.Next() {
		if err = rows.Scan(&name, &percent); err != nil {
			return llmContext{}, fmt.Errorf("scanning row: %w", err)
		}

		switch name {
		case "amount":
			context.amountContribution = percent

		case "hour_of_day":
			context.hourOfDayContribution = percent

		case "location":
			context.locationContribution = percent
		}
	}

	return context, nil
}

func (a *Reasoning) storeNotification(ctx context.Context, resp string, msg models.AnomalyMessage) error {
	const stmt = `INSERT INTO notification (purchase_id, customer_id, reasoning) VALUES ($1, $2, $3)`

	if _, err := a.d.DB.ExecContext(ctx, stmt, msg.PurchaseID, msg.CustomerID, resp); err != nil {
		return fmt.Errorf("executing query: %w", err)
	}

	return nil
}
