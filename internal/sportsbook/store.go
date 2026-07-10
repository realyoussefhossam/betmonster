package sportsbook

import (
	"context"
	"errors"
	"time"
)

var (
	ErrBetNotFound   = errors.New("bet not found")
	ErrInvalidStatus = errors.New("invalid bet status")
)

type Store interface {
	CreateBet(ctx context.Context, b Bet) (string, error)
	GetBet(ctx context.Context, id string) (*Bet, error)
	ListBets(ctx context.Context, userID, status string, page, pageSize int) ([]Bet, error)
	ListPendingBets(ctx context.Context) ([]Bet, error)
	UpdateBetStatus(ctx context.Context, id, status string, settledAt time.Time) error
	GetBetByReference(ctx context.Context, userID, referenceID string) (*Bet, error)
}
