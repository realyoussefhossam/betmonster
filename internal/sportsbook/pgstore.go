package sportsbook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

type PGStore struct {
	db *sql.DB
}

func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) CreateBet(ctx context.Context, b Bet) (string, error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}

	const q = `
		INSERT INTO bets (id, user_id, event_id, market_id, outcome_id, odds, stake, potential_payout, currency, status, reference_id, debit_transaction_id, credit_transaction_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`
	var id string
	err := s.db.QueryRowContext(ctx, q,
		b.ID, b.UserID, b.EventID, b.MarketID, b.OutcomeID, b.Odds, b.Stake, b.PotentialPayout,
		b.Currency, b.Status, b.ReferenceID, b.DebitTransactionID, b.CreditTransactionID, b.CreatedAt,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", fmt.Errorf("create bet: %w", ErrBetNotFound)
		}
		return "", fmt.Errorf("create bet: %w", err)
	}
	return id, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgerrcode.UniqueViolation
	}
	return false
}

func (s *PGStore) GetBet(ctx context.Context, id string) (*Bet, error) {
	const q = `
		SELECT id, user_id, event_id, market_id, outcome_id, odds, stake, potential_payout, currency, status, reference_id, debit_transaction_id, credit_transaction_id, created_at, settled_at
		FROM bets
		WHERE id = $1
	`
	var b Bet
	var settledAt sql.NullTime
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&b.ID, &b.UserID, &b.EventID, &b.MarketID, &b.OutcomeID, &b.Odds, &b.Stake, &b.PotentialPayout,
		&b.Currency, &b.Status, &b.ReferenceID, &b.DebitTransactionID, &b.CreditTransactionID, &b.CreatedAt, &settledAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrBetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bet: %w", err)
	}
	if settledAt.Valid {
		b.SettledAt = &settledAt.Time
	}
	return &b, nil
}

func (s *PGStore) ListBets(ctx context.Context, userID, status string, page, pageSize int) ([]Bet, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var q string
	var rows *sql.Rows
	var err error
	if status != "" {
		q = `
			SELECT id, user_id, event_id, market_id, outcome_id, odds, stake, potential_payout, currency, status, reference_id, debit_transaction_id, credit_transaction_id, created_at, settled_at
			FROM bets
			WHERE user_id = $1 AND status = $2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4
		`
		rows, err = s.db.QueryContext(ctx, q, userID, status, pageSize, offset)
	} else {
		q = `
			SELECT id, user_id, event_id, market_id, outcome_id, odds, stake, potential_payout, currency, status, reference_id, debit_transaction_id, credit_transaction_id, created_at, settled_at
			FROM bets
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = s.db.QueryContext(ctx, q, userID, pageSize, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list bets: %w", err)
	}
	defer rows.Close()

	var out []Bet
	for rows.Next() {
		var b Bet
		var settledAt sql.NullTime
		if err := rows.Scan(
			&b.ID, &b.UserID, &b.EventID, &b.MarketID, &b.OutcomeID, &b.Odds, &b.Stake, &b.PotentialPayout,
			&b.Currency, &b.Status, &b.ReferenceID, &b.DebitTransactionID, &b.CreditTransactionID, &b.CreatedAt, &settledAt,
		); err != nil {
			return nil, fmt.Errorf("scan bet: %w", err)
		}
		if settledAt.Valid {
			b.SettledAt = &settledAt.Time
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list bets rows: %w", err)
	}
	return out, nil
}

func (s *PGStore) UpdateBetStatus(ctx context.Context, id, status string, settledAt time.Time) error {
	var q string
	var args []any
	if settledAt.IsZero() {
		q = `UPDATE bets SET status = $1 WHERE id = $2 RETURNING id`
		args = []any{status, id}
	} else {
		q = `UPDATE bets SET status = $1, settled_at = $2 WHERE id = $3 RETURNING id`
		args = []any{status, settledAt, id}
	}
	var returnedID string
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&returnedID)
	if err == sql.ErrNoRows {
		return ErrBetNotFound
	}
	if err != nil {
		return fmt.Errorf("update bet status: %w", err)
	}
	return nil
}

func (s *PGStore) UpdateBetStatusAndDebitTx(ctx context.Context, id, status, debitTxID string, settledAt time.Time) error {
	var q string
	var args []any
	if settledAt.IsZero() {
		q = `UPDATE bets SET status = $1, debit_transaction_id = $2 WHERE id = $3 RETURNING id`
		args = []any{status, debitTxID, id}
	} else {
		q = `UPDATE bets SET status = $1, debit_transaction_id = $2, settled_at = $3 WHERE id = $4 RETURNING id`
		args = []any{status, debitTxID, settledAt, id}
	}
	var returnedID string
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&returnedID)
	if err == sql.ErrNoRows {
		return ErrBetNotFound
	}
	if err != nil {
		return fmt.Errorf("update bet status and debit tx: %w", err)
	}
	return nil
}

func (s *PGStore) UpdateBetStatusAndOutcome(ctx context.Context, id, status string, settledAt time.Time) error {
	var q string
	var args []any
	if settledAt.IsZero() {
		q = `UPDATE bets SET status = $1 WHERE id = $2 RETURNING id`
		args = []any{status, id}
	} else {
		q = `UPDATE bets SET status = $1, settled_at = $2 WHERE id = $3 RETURNING id`
		args = []any{status, settledAt, id}
	}
	var returnedID string
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&returnedID)
	if err == sql.ErrNoRows {
		return ErrBetNotFound
	}
	if err != nil {
		return fmt.Errorf("update bet status and outcome: %w", err)
	}
	return nil
}

func (s *PGStore) SetCreditTransactionID(ctx context.Context, id, creditTxID string) error {
	const q = `
		UPDATE bets
		SET credit_transaction_id = $1
		WHERE id = $2
		RETURNING id
	`
	var returnedID string
	err := s.db.QueryRowContext(ctx, q, creditTxID, id).Scan(&returnedID)
	if err == sql.ErrNoRows {
		return ErrBetNotFound
	}
	if err != nil {
		return fmt.Errorf("set credit transaction id: %w", err)
	}
	return nil
}

// ListPendingBets returns pending bets ordered oldest-first (created_at ASC),
// matching PGStore and the scheduler's expectations.
func (s *PGStore) ListPendingBets(ctx context.Context, page, pageSize int) ([]Bet, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	const q = `
		SELECT id, user_id, event_id, market_id, outcome_id, odds, stake, potential_payout, currency, status, reference_id, debit_transaction_id, credit_transaction_id, created_at, settled_at
		FROM bets
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, q, StatusPending, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending bets: %w", err)
	}
	defer rows.Close()

	var out []Bet
	for rows.Next() {
		var b Bet
		var settledAt sql.NullTime
		if err := rows.Scan(
			&b.ID, &b.UserID, &b.EventID, &b.MarketID, &b.OutcomeID, &b.Odds, &b.Stake, &b.PotentialPayout,
			&b.Currency, &b.Status, &b.ReferenceID, &b.DebitTransactionID, &b.CreditTransactionID, &b.CreatedAt, &settledAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending bet: %w", err)
		}
		if settledAt.Valid {
			b.SettledAt = &settledAt.Time
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pending bets rows: %w", err)
	}
	return out, nil
}

func (s *PGStore) GetBetByReference(ctx context.Context, userID, referenceID string) (*Bet, error) {
	const q = `
		SELECT id, user_id, event_id, market_id, outcome_id, odds, stake, potential_payout, currency, status, reference_id, debit_transaction_id, credit_transaction_id, created_at, settled_at
		FROM bets
		WHERE user_id = $1 AND reference_id = $2
	`
	var b Bet
	var settledAt sql.NullTime
	err := s.db.QueryRowContext(ctx, q, userID, referenceID).Scan(
		&b.ID, &b.UserID, &b.EventID, &b.MarketID, &b.OutcomeID, &b.Odds, &b.Stake, &b.PotentialPayout,
		&b.Currency, &b.Status, &b.ReferenceID, &b.DebitTransactionID, &b.CreditTransactionID, &b.CreatedAt, &settledAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrBetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bet by reference: %w", err)
	}
	if settledAt.Valid {
		b.SettledAt = &settledAt.Time
	}
	return &b, nil
}
