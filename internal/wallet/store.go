package wallet

import (
	"context"
	"errors"
)

var ErrWalletNotFound = errors.New("wallet not found")

type Store interface {
	GetWallet(ctx context.Context, userID, currency string) (*Wallet, error)
	CreateWallet(ctx context.Context, userID, currency string) (*Wallet, error)
	CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error)
	DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error)
	ReverseDebit(ctx context.Context, transactionID string) (*Transaction, error)
	GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error)
	CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error)
	RequestWithdrawal(ctx context.Context, userID, currency, amount, destinationAddress, chain string) (*WithdrawalRequest, error)
	ApproveWithdrawal(ctx context.Context, id, txHash, reviewedBy string) (*WithdrawalRequest, error)
	RejectWithdrawal(ctx context.Context, id, reviewedBy string) (*WithdrawalRequest, error)
	ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error)
	ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error)
}
