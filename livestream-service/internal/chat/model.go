package chat

import "time"

type Message struct {
	Type      string    `json:"type" example:"message"`
	UserID    string    `json:"user_id" example:"user_123"`
	Username  string    `json:"username" example:"alice"`
	Body      string    `json:"body" example:"hello"`
	AmountUSD float64   `json:"amount_usd" example:"0"`
	SentAt    time.Time `json:"sent_at" example:"2026-05-23T10:00:00Z"`
}
