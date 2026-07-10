package sportsbook

import "time"

type Bet struct {
	ID                  string
	UserID              string
	EventID             string
	MarketID            string
	OutcomeID           string
	Odds                string
	Stake               string
	PotentialPayout     string
	Currency            string
	Status              string
	ReferenceID         string
	DebitTransactionID  string
	CreditTransactionID string
	CreatedAt           time.Time
	SettledAt           *time.Time
}

type OddsSnapshot struct {
	OutcomeID string
	Odds      string
}

const (
	StatusDebitPending = "debit_pending"
	StatusPending      = "pending"
	StatusWon          = "won"
	StatusLost         = "lost"
	StatusCancelled    = "cancelled"
	StatusSettled      = "settled"
)
