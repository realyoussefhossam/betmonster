package wallet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCreditWallet(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	tx, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)
	assert.Equal(t, "completed", tx.Status)
	assert.Equal(t, "100", tx.BalanceAfter)

	// idempotent
	tx2, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)
	assert.Equal(t, tx.ID, tx2.ID)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "100", wallet.Balance)
}

func TestServiceDebitWalletInsufficientBalance(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "50.00", "dx-1", nil)
	assert.NoError(t, err)

	_, err = svc.DebitWallet(ctx, "user-1", "USDT", "100.00", "wd-1")
	assert.Error(t, err)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "50", wallet.Balance)
}

func TestServiceDebitWallet(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	tx, err := svc.DebitWallet(ctx, "user-1", "USDT", "40.00", "wd-1")
	assert.NoError(t, err)
	assert.Equal(t, "withdrawal", tx.Type)
	assert.Equal(t, "60", tx.BalanceAfter)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "60", wallet.Balance)
}

func TestServiceGetBalanceCreatesWallet(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	wallet, err := svc.GetBalance(ctx, "new-user", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "USDT", wallet.Currency)
	assert.Equal(t, "0", wallet.Balance)

	// A second call should return the same wallet, not create another.
	wallet2, err := svc.GetBalance(ctx, "new-user", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, wallet.ID, wallet2.ID)
}

func TestServiceRequestWithdrawal(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	req, err := svc.RequestWithdrawal(ctx, "user-1", "USDT", "40.00", "0xABC", "anvil")
	assert.NoError(t, err)
	assert.Equal(t, "pending", req.Status)
	assert.Equal(t, "40.00", req.Amount)
	assert.Equal(t, "anvil", req.Chain)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "60", wallet.Balance)

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, transactions, 2)

	var withdrawalTx *Transaction
	for i := range transactions {
		if transactions[i].Type == "withdrawal" {
			withdrawalTx = &transactions[i]
		}
	}
	assert.NotNil(t, withdrawalTx)
	assert.Equal(t, "pending", withdrawalTx.Status)
	assert.Equal(t, "40.00", withdrawalTx.Amount)
	assert.Equal(t, "60", withdrawalTx.BalanceAfter)
}

func TestServiceRequestWithdrawalInsufficientBalance(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	_, err = svc.RequestWithdrawal(ctx, "user-1", "USDT", "150.00", "0xABC", "anvil")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInsufficientBalance)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "100", wallet.Balance)
}

func TestServiceApproveWithdrawal(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	req, err := svc.RequestWithdrawal(ctx, "user-1", "USDT", "40.00", "0xABC", "anvil")
	assert.NoError(t, err)

	approved, err := svc.ReviewWithdrawal(ctx, req.ID, "approve", "0xTXHASH", "admin-1")
	assert.NoError(t, err)
	assert.Equal(t, "approved", approved.Status)
	assert.Equal(t, "0xTXHASH", approved.TxHash)
	assert.Equal(t, "admin-1", approved.ReviewedBy)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "60", wallet.Balance)

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 10)
	assert.NoError(t, err)
	var withdrawalTx *Transaction
	for i := range transactions {
		if transactions[i].Type == "withdrawal" {
			withdrawalTx = &transactions[i]
		}
	}
	assert.NotNil(t, withdrawalTx)
	assert.Equal(t, "completed", withdrawalTx.Status)
}

func TestServiceRejectWithdrawal(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	req, err := svc.RequestWithdrawal(ctx, "user-1", "USDT", "40.00", "0xABC", "anvil")
	assert.NoError(t, err)

	rejected, err := svc.ReviewWithdrawal(ctx, req.ID, "reject", "", "admin-1")
	assert.NoError(t, err)
	assert.Equal(t, "rejected", rejected.Status)
	assert.Equal(t, "admin-1", rejected.ReviewedBy)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "100", wallet.Balance)

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 10)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(transactions), 3)

	var withdrawalTx, reversalTx *Transaction
	for i := range transactions {
		switch transactions[i].Type {
		case "withdrawal":
			withdrawalTx = &transactions[i]
		case "adjustment":
			reversalTx = &transactions[i]
		}
	}
	assert.NotNil(t, withdrawalTx)
	assert.Equal(t, "reversed", withdrawalTx.Status)
	assert.NotNil(t, reversalTx)
	assert.Equal(t, "40.00", reversalTx.Amount)
	assert.Equal(t, "completed", reversalTx.Status)
	assert.Equal(t, "100", reversalTx.BalanceAfter)
}

func TestServiceUnsupportedCurrency(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil", "BNB:bsc"})

	_, err := svc.GetBalance(ctx, "user-1", "BTC")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported currency")

	_, err = svc.GetDepositAddress(ctx, "user-1", "BNB", "anvil")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported currency-chain pair")

	_, err = svc.RequestWithdrawal(ctx, "user-1", "BTC", "10", "0x123", "anvil")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported currency-chain pair")
}

func TestServiceCreditWalletInvalidAmount(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "invalid", "dx-1", nil)
	assert.Error(t, err)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assert.Equal(t, "0", wallet.Balance)
}

func TestServiceDebitWalletInvalidAmount(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})

	_, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	_, err = svc.DebitWallet(ctx, "user-1", "USDT", "invalid", "wd-1")
	assert.Error(t, err)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assert.Equal(t, "100", wallet.Balance)
}
