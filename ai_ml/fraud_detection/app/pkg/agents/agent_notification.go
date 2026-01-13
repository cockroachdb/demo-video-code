package agents

import (
	"context"
	"crdb/ai_ml/fraud_detection/app/pkg/models"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
)

type Notification struct {
	d         *Dependencies
	snsClient *ses.Client
}

func NewNotification(d *Dependencies) (*Notification, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(d.Region))
	if err != nil {
		return nil, fmt.Errorf("loading default config: %w", err)
	}

	return &Notification{
		d:         d,
		snsClient: ses.NewFromConfig(cfg),
	}, nil
}

// Run blocks forever.
func (a *Notification) Run(ctx context.Context) {
	c := a.d.Bus.NewConsumer(a.d.Topic)
	c.Run(ctx, a.Process)
}

func (a *Notification) Name() string {
	return "agent.notification"
}

func (a *Notification) Process(ctx context.Context, m models.Message) error {
	var msg models.NotificationMessage
	if err := models.ParsePayload(m, &msg); err != nil {
		return fmt.Errorf("parsing notification message: %w", err)
	}
	log.Printf("notification received")

	// Fetch purchase context.
	context, err := a.fetchContext(ctx, msg)
	if err != nil {
		return fmt.Errorf("fetching context for reasoning: %w", err)
	}
	log.Printf("context fetched")

	log.Printf("sending notification: %+v", context)

	return nil
}

type notificationContext struct {
	channel string
	target  string
	message string
}

func (a *Notification) fetchContext(ctx context.Context, msg models.NotificationMessage) (notificationContext, error) {
	const stmt = `SELECT channel, target, message FROM fetch_notification_context($1, $2)`

	row := a.d.DB.QueryRowContext(ctx, stmt, msg.PurchaseID, msg.CustomerID)

	var nc notificationContext
	if err := row.Scan(&nc.channel, &nc.target, &nc.message); err != nil {
		return notificationContext{}, fmt.Errorf("scanning row: %w", err)
	}

	return nc, nil
}
