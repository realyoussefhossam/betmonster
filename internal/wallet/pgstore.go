package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type PGStore struct {
	db *sql.DB
}

var ErrNotImplemented = errors.New("not implemented")

func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) CreateWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
	const q = `
		INSERT INTO wallets (user_id, currency, balance, version)
		VALUES ($1, $2, 0, 0)
		ON CONFLICT (user_id, currency) DO UPDATE SET updated_at = NOW()
		RETURNING id, user_id, currency, balance, version, created_at, updated_at
	`
	row := s.db.QueryRowContext(ctx, q, userID, currency)
	var w Wallet
	err := row.Scan(&w.ID, &w.UserID, &w.Currency, &w.Balance, &w.Version, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create wallet: %w", err)
	}
	return &w, nil
}

func (s *PGStore) GetWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) ReverseDebit(ctx context.Context, transactionID string) (*Transaction, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) CreateWithdrawalRequest(ctx context.Context, req *WithdrawalRequest) (*WithdrawalRequest, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*WithdrawalRequest, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error) {
	return nil, ErrNotImplemented
}
