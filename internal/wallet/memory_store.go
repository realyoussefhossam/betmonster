package wallet

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type memoryStore struct {
	mu          sync.Mutex
	wallets     map[string]*Wallet
	txns        map[string]*Transaction
	addresses   map[string]*DepositAddress
	withdrawals map[string]*WithdrawalRequest
}

func newInMemoryStore() *memoryStore {
	return &memoryStore{
		wallets:     map[string]*Wallet{},
		txns:        map[string]*Transaction{},
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
		return nil, errors.New("wallet not found")
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
		if existing, ok := s.txns[referenceID]; ok {
			return existing, nil
		}
	}
	w := s.createWallet(userID, currency)
	newBalance := addDecimal(w.Balance, amount)
	txn := &Transaction{
		ID:            uuid.NewString(),
		UserID:        userID,
		WalletID:      w.ID,
		Type:          "deposit",
		Amount:        amount,
		BalanceBefore: w.Balance,
		BalanceAfter:  newBalance,
		Status:        "completed",
		ReferenceID:   referenceID,
		CreatedAt:     time.Now(),
	}
	if referenceID != "" {
		s.txns[referenceID] = txn
	}
	w.Balance = newBalance
	w.Version++
	w.UpdatedAt = time.Now()
	return txn, nil
}

func (s *memoryStore) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	newBalance := subDecimal(w.Balance, amount)
	txn := &Transaction{
		ID:            uuid.NewString(),
		UserID:        userID,
		WalletID:      w.ID,
		Type:          "withdrawal",
		Amount:        amount,
		BalanceBefore: w.Balance,
		BalanceAfter:  newBalance,
		Status:        "completed",
		ReferenceID:   referenceID,
		CreatedAt:     time.Now(),
	}
	if referenceID != "" {
		s.txns[referenceID] = txn
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
	return nil, errors.New("not found")
}

func (s *memoryStore) CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error) {
	addr.ID = uuid.NewString()
	addr.Status = "active"
	addr.CreatedAt = time.Now()
	s.addresses[addr.ID] = addr
	return addr, nil
}

func (s *memoryStore) CreateWithdrawalRequest(ctx context.Context, req *WithdrawalRequest) (*WithdrawalRequest, error) {
	req.ID = uuid.NewString()
	req.Status = "pending"
	req.CreatedAt = time.Now()
	s.withdrawals[req.ID] = req
	return req, nil
}

func (s *memoryStore) ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error) {
	return nil, nil
}

func (s *memoryStore) ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*WithdrawalRequest, error) {
	return nil, errors.New("not implemented in unit test stub")
}

func (s *memoryStore) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error) {
	return nil, nil
}
