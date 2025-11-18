package agents

import (
	"context"
	"crdb/ai_ml/fraud_detection/app/pkg/models"
	"fmt"
	"log"
	"time"
)

type AnomalyDetection struct {
	d *Dependencies

	delays chan time.Duration
}

func NewAnomalyDetection(d *Dependencies) *AnomalyDetection {
	return &AnomalyDetection{
		d:      d,
		delays: make(chan time.Duration, 10),
	}
}

// Run blocks forever.
func (a *AnomalyDetection) Run(ctx context.Context) {
	go a.log()

	a.d.Bus.Run(ctx, a.Process)
}

func (a *AnomalyDetection) Name() string {
	return "agent.anomaly-detection"
}

func (a *AnomalyDetection) Process(ctx context.Context, m models.Message) error {
	var msg models.PurchaseMessage
	if err := models.ParsePayload(m, &msg); err != nil {
		return fmt.Errorf("parsing purchase message: %w", err)
	}

	// Calculate delay between purchase creation and anomaly message received.
	delay := time.Since(msg.Timestamp)
	a.delays <- delay

	// Fetch distance from database.
	distance, err := a.fetchDistance(ctx, msg)
	if err != nil {
		return fmt.Errorf("fetching distance: %w", err)
	}

	// Create anomaly in the database if distance is sufficient.
	if distance <= 0.3 {
		return nil
	}

	log.Printf("anomalous purchase (distance: %0.3f)", distance)

	if err = a.createAnomaly(ctx, msg, distance); err != nil {
		return fmt.Errorf("inserting anomaly: %w", err)
	}

	return nil
}

func (a *AnomalyDetection) fetchDistance(ctx context.Context, msg models.PurchaseMessage) (float64, error) {
	const stmt = `SELECT * FROM purchase_distance_from_average($1, $2)`

	row := a.d.DB.QueryRowContext(ctx, stmt, msg.ID, msg.CustomerID)

	var score float64
	if err := row.Scan(&score); err != nil {
		return 0, fmt.Errorf("executing query: %w", err)
	}

	return score, nil
}

func (a *AnomalyDetection) createAnomaly(ctx context.Context, msg models.PurchaseMessage, score float64) error {
	const stmt = `INSERT INTO anomaly (purchase_id, customer_id, score) VALUES ($1, $2, $3)`

	_, err := a.d.DB.ExecContext(ctx, stmt, msg.ID, msg.CustomerID, score)
	if err != nil {
		return fmt.Errorf("executing query: %w", err)
	}

	return nil
}

func (a *AnomalyDetection) log() {
	var delays []time.Duration

	logTick := time.Tick(time.Second * 1)

	for {
		select {
		case d := <-a.delays:
			delays = append(delays, d)

		case <-logTick:
			if len(delays) == 0 {
				continue
			}

			var total time.Duration
			for _, d := range delays {
				total += d
			}

			log.Printf(
				"events processed: %d / average delay: %v",
				len(delays),
				total/time.Duration(len(delays)),
			)

			delays = []time.Duration{}
		}
	}
}
