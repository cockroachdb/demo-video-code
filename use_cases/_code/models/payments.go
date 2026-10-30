package models

import "time"

// Card represents a payment card stored in the database.
type Card struct {
	Number     string    `json:"number"`
	HolderName string    `json:"holder_name"`
	Expiry     time.Time `json:"expiry"`
	IsDefault  bool      `json:"is_default"`
}

// Payment represents a payment request from a customer.
type Payment struct {
	Card
	CVV    string  `json:"cvv"`
	Amount float64 `json:"amount"`
}
