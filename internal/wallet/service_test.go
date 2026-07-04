package wallet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceCreditWallet(t *testing.T) {
	ctx := context.Background()
	store := newInMemoryStore()
	svc := NewService(store, nil, nil)

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
	store := newInMemoryStore()
	svc := NewService(store, nil, nil)

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
	store := newInMemoryStore()
	svc := NewService(store, nil, nil)

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
