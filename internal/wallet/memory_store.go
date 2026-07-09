package wallet

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type memoryStore struct {
	mu          sync.Mutex
	wallets     map[string]*Wallet
	txns        map[string]*Transaction
	txnByRef    map[string]*Transaction
	addresses   map[string]*DepositAddress
	withdrawals map[string]*WithdrawalRequest
}

func NewInMemoryStore() *memoryStore {
	return &memoryStore{
		wallets:     map[string]*Wallet{},
		txns:        map[string]*Transaction{},
		txnByRef:    map[string]*Transaction{},
		addresses:   map[string]*DepositAddress{},
		withdrawals: map[string]*WithdrawalRequest{},
	}
}

func (s *memoryStore) walletKey(userID, currency string) string {
	return userID + ":" + currency
}

func (s *memoryStore) createWallet(userID, currency string) *Wallet {
	key := s.walletKey(userID, currency)
	if w, ok := s.wallets[key]; ok {
		return w
	}
	w := &Wallet{ID: uuid.NewString(), UserID: userID, Currency: currency, Balance: "0", Version: 0, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.wallets[key] = w
	return w
}

func (s *memoryStore) GetWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.wallets[s.walletKey(userID, currency)]
	if !ok {
		return nil, ErrWalletNotFound
	}
	return w, nil
}

func (s *memoryStore) CreateWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createWallet(userID, currency), nil
}

func (s *memoryStore) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if referenceID != "" {
		if existing, ok := s.txnByRef[referenceID]; ok {
			return existing, nil
		}
	}
	w := s.createWallet(userID, currency)
	newBalance, err := addDecimal(w.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("credit wallet: %w", err)
	}
	txn := &Transaction{
		ID:            uuid.NewString(),
		UserID:        userID,
		WalletID:      w.ID,
		Currency:      currency,
		Type:          "deposit",
		Amount:        amount,
		BalanceBefore: w.Balance,
		BalanceAfter:  newBalance,
		Status:        "completed",
		ReferenceID:   referenceID,
		CreatedAt:     time.Now(),
	}
	s.txns[txn.ID] = txn
	if referenceID != "" {
		s.txnByRef[referenceID] = txn
	}
	w.Balance = newBalance
	w.Version++
	w.UpdatedAt = time.Now()
	return txn, nil
}

func (s *memoryStore) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if referenceID != "" {
		if existing, ok := s.txnByRef[referenceID]; ok {
			return existing, nil
		}
	}

	key := s.walletKey(userID, currency)
	w, ok := s.wallets[key]
	if !ok {
		return nil, errors.New("wallet not found")
	}

	currentBalance, err := decimal.NewFromString(w.Balance)
	if err != nil {
		return nil, err
	}
	debitAmount, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	if currentBalance.LessThan(debitAmount) {
		return nil, errors.New("insufficient balance")
	}

	newBalance, err := subDecimal(w.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("debit wallet: %w", err)
	}
	txn := &Transaction{
		ID:            uuid.NewString(),
		UserID:        userID,
		WalletID:      w.ID,
		Currency:      currency,
		Type:          "withdrawal",
		Amount:        amount,
		BalanceBefore: w.Balance,
		BalanceAfter:  newBalance,
		Status:        "completed",
		ReferenceID:   referenceID,
		CreatedAt:     time.Now(),
	}
	s.txns[txn.ID] = txn
	if referenceID != "" {
		s.txnByRef[referenceID] = txn
	}
	w.Balance = newBalance
	w.Version++
	w.UpdatedAt = time.Now()
	return txn, nil
}

func (s *memoryStore) ReverseDebit(ctx context.Context, transactionID string) (*Transaction, error) {
	return nil, errors.New("not implemented in unit test stub")
}

func (s *memoryStore) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, addr := range s.addresses {
		if addr.UserID == userID && addr.Currency == currency && addr.Chain == chain && addr.Status == "active" {
			return addr, nil
		}
	}
	return nil, errors.New("not found")
}

func (s *memoryStore) CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addr.ID = uuid.NewString()
	addr.Status = "active"
	addr.CreatedAt = time.Now()
	s.addresses[addr.ID] = addr
	return addr, nil
}

func (s *memoryStore) RequestWithdrawal(ctx context.Context, userID, currency, amount, destinationAddress, chain string) (*WithdrawalRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.walletKey(userID, currency)
	w, ok := s.wallets[key]
	if !ok {
		return nil, ErrWalletNotFound
	}

	currentBalance, err := decimal.NewFromString(w.Balance)
	if err != nil {
		return nil, err
	}
	withdrawalAmount, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	if currentBalance.LessThan(withdrawalAmount) {
		return nil, ErrInsufficientBalance
	}

	newBalance, err := subDecimal(w.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("debit wallet for withdrawal: %w", err)
	}
	w.Balance = newBalance
	w.Version++
	w.UpdatedAt = time.Now()

	reqID := uuid.NewString()
	now := time.Now()

	withdrawalTx := &Transaction{
		ID:            uuid.NewString(),
		UserID:        userID,
		WalletID:      w.ID,
		Currency:      currency,
		Type:          "withdrawal",
		Amount:        amount,
		BalanceBefore: currentBalance.String(),
		BalanceAfter:  newBalance,
		Status:        "pending",
		ReferenceID:   reqID,
		CreatedAt:     now,
	}
	s.txns[withdrawalTx.ID] = withdrawalTx
	s.txnByRef[reqID] = withdrawalTx

	req := &WithdrawalRequest{
		ID:                 reqID,
		UserID:             userID,
		WalletID:           w.ID,
		Amount:             amount,
		Currency:           currency,
		DestinationAddress: destinationAddress,
		Chain:              chain,
		Status:             "pending",
		CreatedAt:          now,
	}
	s.withdrawals[req.ID] = req

	return req, nil
}

func (s *memoryStore) ApproveWithdrawal(ctx context.Context, id, txHash, reviewedBy string) (*WithdrawalRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.withdrawals[id]
	if !ok {
		return nil, errors.New("withdrawal request not found")
	}
	if req.Status != "pending" {
		return nil, errors.New("withdrawal request not pending")
	}

	req.Status = "approved"
	req.TxHash = txHash
	req.ReviewedBy = reviewedBy

	if tx, ok := s.txnByRef[id]; ok {
		tx.Status = "completed"
	}

	return req, nil
}

func (s *memoryStore) RejectWithdrawal(ctx context.Context, id, reviewedBy string) (*WithdrawalRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.withdrawals[id]
	if !ok {
		return nil, errors.New("withdrawal request not found")
	}
	if req.Status != "pending" {
		return nil, errors.New("withdrawal request not pending")
	}

	w, ok := s.wallets[s.walletKey(req.UserID, req.Currency)]
	if !ok {
		return nil, ErrWalletNotFound
	}

	balanceBefore := w.Balance
	reversedBalance, err := addDecimal(w.Balance, req.Amount)
	if err != nil {
		return nil, fmt.Errorf("reverse withdrawal debit: %w", err)
	}
	w.Balance = reversedBalance
	w.Version++
	w.UpdatedAt = time.Now()

	req.Status = "rejected"
	req.ReviewedBy = reviewedBy

	if tx, ok := s.txnByRef[id]; ok {
		tx.Status = "reversed"
	}

	reversalTx := &Transaction{
		ID:            uuid.NewString(),
		UserID:        req.UserID,
		WalletID:      w.ID,
		Currency:      req.Currency,
		Type:          "adjustment",
		Amount:        req.Amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  reversedBalance,
		Status:        "completed",
		ReferenceID:   id + "-reversal",
		CreatedAt:     time.Now(),
	}
	s.txns[reversalTx.ID] = reversalTx

	return req, nil
}

func (s *memoryStore) ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []WithdrawalRequest
	for _, w := range s.withdrawals {
		if w.Status == "pending" {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (s *memoryStore) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Transaction
	for _, t := range s.txns {
		if t.UserID == userID {
			out = append(out, *t)
		}
	}
	return out, nil
}
