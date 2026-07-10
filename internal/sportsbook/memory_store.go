package sportsbook

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type InMemoryStore struct {
	mu   sync.RWMutex
	bets map[string]Bet
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		bets: make(map[string]Bet),
	}
}

func (s *InMemoryStore) CreateBet(ctx context.Context, b Bet) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.bets {
		if existing.UserID == b.UserID && existing.ReferenceID == b.ReferenceID {
			return "", ErrBetNotFound // placeholder; caller checks GetBetByReference first
		}
	}

	id := b.ID
	if id == "" {
		id = uuid.New().String()
	}
	b.ID = id
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	s.bets[id] = b
	return id, nil
}

func (s *InMemoryStore) GetBet(ctx context.Context, id string) (*Bet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.bets[id]
	if !ok {
		return nil, ErrBetNotFound
	}
	return &b, nil
}

func (s *InMemoryStore) ListBets(ctx context.Context, userID, status string, page, pageSize int) ([]Bet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var filtered []Bet
	for _, b := range s.bets {
		if b.UserID != userID {
			continue
		}
		if status != "" && b.Status != status {
			continue
		}
		filtered = append(filtered, b)
	}

	// Sort by created_at descending (stable for tests by id if equal)
	sortBetsByCreatedDesc(filtered)

	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], nil
}

func sortBetsByCreatedDesc(bets []Bet) {
	// simple insertion sort for test determinism
	for i := 1; i < len(bets); i++ {
		j := i
		for j > 0 {
			if bets[j].CreatedAt.After(bets[j-1].CreatedAt) ||
				(bets[j].CreatedAt.Equal(bets[j-1].CreatedAt) && bets[j].ID > bets[j-1].ID) {
				bets[j], bets[j-1] = bets[j-1], bets[j]
				j--
			} else {
				break
			}
		}
	}
}

func (s *InMemoryStore) UpdateBetStatus(ctx context.Context, id, status string, settledAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bets[id]
	if !ok {
		return ErrBetNotFound
	}
	b.Status = status
	if !settledAt.IsZero() {
		b.SettledAt = &settledAt
	}
	s.bets[id] = b
	return nil
}

func (s *InMemoryStore) UpdateBetStatusAndDebitTx(ctx context.Context, id, status, debitTxID string, settledAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bets[id]
	if !ok {
		return ErrBetNotFound
	}
	b.Status = status
	b.DebitTransactionID = debitTxID
	if !settledAt.IsZero() {
		b.SettledAt = &settledAt
	}
	s.bets[id] = b
	return nil
}

func (s *InMemoryStore) UpdateBetStatusAndOutcome(ctx context.Context, id, status string, settledAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bets[id]
	if !ok {
		return ErrBetNotFound
	}
	b.Status = status
	if !settledAt.IsZero() {
		b.SettledAt = &settledAt
	}
	s.bets[id] = b
	return nil
}

func (s *InMemoryStore) SetCreditTransactionID(ctx context.Context, id, creditTxID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bets[id]
	if !ok {
		return ErrBetNotFound
	}
	b.CreditTransactionID = creditTxID
	s.bets[id] = b
	return nil
}

// ListPendingBets returns pending bets ordered oldest-first (created_at ASC),
// matching PGStore and the scheduler's expectations.
func (s *InMemoryStore) ListPendingBets(ctx context.Context, page, pageSize int) ([]Bet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var out []Bet
	for _, b := range s.bets {
		if b.Status == StatusPending {
			out = append(out, b)
		}
	}
	sortBetsByCreatedAsc(out)

	start := (page - 1) * pageSize
	if start > len(out) {
		start = len(out)
	}
	end := start + pageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], nil
}

func sortBetsByCreatedAsc(bets []Bet) {
	for i := 1; i < len(bets); i++ {
		j := i
		for j > 0 {
			if bets[j].CreatedAt.Before(bets[j-1].CreatedAt) ||
				(bets[j].CreatedAt.Equal(bets[j-1].CreatedAt) && bets[j].ID < bets[j-1].ID) {
				bets[j], bets[j-1] = bets[j-1], bets[j]
				j--
			} else {
				break
			}
		}
	}
}

func (s *InMemoryStore) GetBetByReference(ctx context.Context, userID, referenceID string) (*Bet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, b := range s.bets {
		if b.UserID == userID && b.ReferenceID == referenceID {
			copy := b
			return &copy, nil
		}
	}
	return nil, ErrBetNotFound
}
