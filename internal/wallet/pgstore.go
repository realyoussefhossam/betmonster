package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

type PGStore struct {
	db *sql.DB
}

var (
	ErrNotImplemented     = errors.New("not implemented")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrWalletNotFound     = errors.New("wallet not found")
	ErrWalletConflict     = errors.New("wallet update conflict")
)

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
	const q = `
		SELECT id, user_id, currency, balance, version, created_at, updated_at
		FROM wallets
		WHERE user_id = $1 AND currency = $2
	`
	row := s.db.QueryRowContext(ctx, q, userID, currency)
	var w Wallet
	err := row.Scan(&w.ID, &w.UserID, &w.Currency, &w.Balance, &w.Version, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	return &w, nil
}

func (s *PGStore) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if referenceID != "" {
		var existingID string
		err := tx.QueryRowContext(ctx, "SELECT id FROM transactions WHERE reference_id = $1", referenceID).Scan(&existingID)
		if err == nil {
			txn, err := s.getTransactionByID(ctx, tx, existingID)
			if err != nil {
				return nil, fmt.Errorf("get existing transaction: %w", err)
			}
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit: %w", err)
			}
			return txn, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("check reference id: %w", err)
		}
	}

	var w Wallet
	err = tx.QueryRowContext(ctx,
		"SELECT id, balance, version FROM wallets WHERE user_id = $1 AND currency = $2 FOR UPDATE",
		userID, currency,
	).Scan(&w.ID, &w.Balance, &w.Version)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx,
			"INSERT INTO wallets (user_id, currency, balance, version) VALUES ($1, $2, 0, 0) RETURNING id, balance, version",
			userID, currency,
		).Scan(&w.ID, &w.Balance, &w.Version)
	}
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}

	newBalance := addDecimal(w.Balance, amount)
	res, err := tx.ExecContext(ctx,
		"UPDATE wallets SET balance = $1, version = version + 1, updated_at = NOW() WHERE id = $2 AND version = $3",
		newBalance, w.ID, w.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("update wallet: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return nil, ErrWalletConflict
	}

	var refID *string
	if referenceID != "" {
		refID = &referenceID
	}

	var txn Transaction
	q := `
		INSERT INTO transactions (user_id, wallet_id, type, amount, balance_before, balance_after, status, reference_id, metadata)
		VALUES ($1, $2, 'deposit', $3, $4, $5, 'completed', $6, $7)
		RETURNING id, created_at
	`
	err = tx.QueryRowContext(ctx, q, userID, w.ID, amount, w.Balance, newBalance, refID, metadata).Scan(&txn.ID, &txn.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	txn.UserID = userID
	txn.WalletID = w.ID
	txn.Type = "deposit"
	txn.Amount = amount
	txn.BalanceBefore = w.Balance
	txn.BalanceAfter = newBalance
	txn.Status = "completed"
	txn.ReferenceID = referenceID
	return &txn, nil
}

func (s *PGStore) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var w Wallet
	err = tx.QueryRowContext(ctx,
		"SELECT id, balance, version FROM wallets WHERE user_id = $1 AND currency = $2 FOR UPDATE",
		userID, currency,
	).Scan(&w.ID, &w.Balance, &w.Version)
	if err == sql.ErrNoRows {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}

	currentBalance, err := decimal.NewFromString(w.Balance)
	if err != nil {
		return nil, fmt.Errorf("parse balance: %w", err)
	}
	debitAmount, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}
	if currentBalance.LessThan(debitAmount) {
		return nil, ErrInsufficientBalance
	}

	newBalance := subDecimal(w.Balance, amount)
	res, err := tx.ExecContext(ctx,
		"UPDATE wallets SET balance = $1, version = version + 1, updated_at = NOW() WHERE id = $2 AND version = $3",
		newBalance, w.ID, w.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("update wallet: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return nil, ErrWalletConflict
	}

	var refID *string
	if referenceID != "" {
		refID = &referenceID
	}

	var txn Transaction
	q := `
		INSERT INTO transactions (user_id, wallet_id, type, amount, balance_before, balance_after, status, reference_id)
		VALUES ($1, $2, 'withdrawal', $3, $4, $5, 'completed', $6)
		RETURNING id, created_at
	`
	err = tx.QueryRowContext(ctx, q, userID, w.ID, amount, w.Balance, newBalance, refID).Scan(&txn.ID, &txn.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	txn.UserID = userID
	txn.WalletID = w.ID
	txn.Type = "withdrawal"
	txn.Amount = amount
	txn.BalanceBefore = w.Balance
	txn.BalanceAfter = newBalance
	txn.Status = "completed"
	txn.ReferenceID = referenceID
	return &txn, nil
}

func (s *PGStore) getTransactionByID(ctx context.Context, tx *sql.Tx, id string) (*Transaction, error) {
	const q = `
		SELECT id, user_id, wallet_id, type, amount, balance_before, balance_after, status, reference_id, metadata, created_at
		FROM transactions
		WHERE id = $1
	`
	var t Transaction
	err := tx.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.UserID, &t.WalletID, &t.Type, &t.Amount, &t.BalanceBefore, &t.BalanceAfter,
		&t.Status, &t.ReferenceID, &t.Metadata, &t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *PGStore) ReverseDebit(ctx context.Context, transactionID string) (*Transaction, error) {
	return nil, ErrNotImplemented
}

func (s *PGStore) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error) {
	const q = `
		SELECT id, user_id, currency, chain, address, xcash_deposit_id, status, created_at
		FROM deposit_addresses
		WHERE user_id = $1 AND currency = $2 AND chain = $3 AND status = 'active'
		LIMIT 1
	`
	var addr DepositAddress
	err := s.db.QueryRowContext(ctx, q, userID, currency, chain).Scan(
		&addr.ID, &addr.UserID, &addr.Currency, &addr.Chain, &addr.Address, &addr.XCashDepositID, &addr.Status, &addr.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("deposit address not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get deposit address: %w", err)
	}
	return &addr, nil
}

func (s *PGStore) CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error) {
	const q = `
		INSERT INTO deposit_addresses (user_id, currency, chain, address, xcash_deposit_id, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
		RETURNING id, created_at
	`
	err := s.db.QueryRowContext(ctx, q, addr.UserID, addr.Currency, addr.Chain, addr.Address, addr.XCashDepositID).Scan(&addr.ID, &addr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create deposit address: %w", err)
	}
	addr.Status = "active"
	return addr, nil
}

func (s *PGStore) CreateWithdrawalRequest(ctx context.Context, req *WithdrawalRequest) (*WithdrawalRequest, error) {
	const q = `
		INSERT INTO withdrawal_requests (user_id, wallet_id, amount, currency, destination_address, chain, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		RETURNING id, status, created_at
	`
	err := s.db.QueryRowContext(ctx, q, req.UserID, req.WalletID, req.Amount, req.Currency, req.DestinationAddress, req.Chain).
		Scan(&req.ID, &req.Status, &req.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create withdrawal request: %w", err)
	}
	return req, nil
}

func (s *PGStore) ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	const q = `
		SELECT id, user_id, wallet_id, amount, currency, destination_address, chain, status, tx_hash, reviewed_by, created_at
		FROM withdrawal_requests
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2
	`
	rows, err := s.db.QueryContext(ctx, q, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending withdrawals: %w", err)
	}
	defer rows.Close()

	var out []WithdrawalRequest
	for rows.Next() {
		var w WithdrawalRequest
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.WalletID, &w.Amount, &w.Currency, &w.DestinationAddress, &w.Chain,
			&w.Status, &w.TxHash, &w.ReviewedBy, &w.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *PGStore) ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*WithdrawalRequest, error) {
	var status string
	switch action {
	case "approve":
		status = "approved"
	case "reject":
		status = "rejected"
	default:
		return nil, errors.New("invalid action")
	}

	const q = `
		UPDATE withdrawal_requests
		SET status = $1, tx_hash = $2, reviewed_by = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING id, user_id, wallet_id, amount, currency, destination_address, chain, status, tx_hash, reviewed_by, created_at
	`
	var w WithdrawalRequest
	err := s.db.QueryRowContext(ctx, q, status, txHash, reviewedBy, id).Scan(
		&w.ID, &w.UserID, &w.WalletID, &w.Amount, &w.Currency, &w.DestinationAddress, &w.Chain,
		&w.Status, &w.TxHash, &w.ReviewedBy, &w.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("withdrawal request not found")
	}
	if err != nil {
		return nil, fmt.Errorf("review withdrawal: %w", err)
	}
	return &w, nil
}

func (s *PGStore) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	const q = `
		SELECT id, user_id, wallet_id, type, amount, balance_before, balance_after, status, reference_id, metadata, created_at
		FROM transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, q, userID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.WalletID, &t.Type, &t.Amount, &t.BalanceBefore, &t.BalanceAfter,
			&t.Status, &t.ReferenceID, &t.Metadata, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
