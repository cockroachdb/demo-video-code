package models

import (
	"encoding/json"
	"fmt"
	"time"
)

type Message struct {
	Key     []string        `json:"Key,omitempty"`
	Topic   string          `json:"Topic,omitempty"`
	Payload json.RawMessage `json:"Value"`
}

type PurchaseMessage struct {
	ID         string       `json:"id"`
	CustomerID string       `json:"customer_id"`
	Amount     float64      `json:"amount"`
	Location   LatLon       `json:"lat_lon"`
	Timestamp  time.Time    `json:"ts"`
	Vector     VectorString `json:"vec"`
}

type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type NotificationMessage struct {
	PurchaseID string    `json:"purchase_id"`
	CustomerID string    `json:"customer_id"`
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"ts"`
}

type AnomalyMessage struct {
	ID         string    `json:"id"`
	PurchaseID string    `json:"purchase_id"`
	CustomerID string    `json:"customer_id"`
	Score      float64   `json:"score"`
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"ts"`
}

func ParsePayload(msg Message, val any) error {
	if err := json.Unmarshal(msg.Payload, &val); err != nil {
		return fmt.Errorf("unmarshalling message: %w", err)
	}

	return nil
}
