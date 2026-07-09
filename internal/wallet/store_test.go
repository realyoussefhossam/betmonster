package wallet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateWallet(t *testing.T) {
	store := NewInMemoryStore()
	wallet, err := store.CreateWallet(context.Background(), "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "user-1", wallet.UserID)
	assert.Equal(t, "USDT", wallet.Currency)
	assert.Equal(t, "0", wallet.Balance)
}

func TestCreditWallet(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	tx, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)
	assert.Equal(t, "completed", tx.Status)
	assert.Equal(t, "100", tx.BalanceAfter)

	// idempotent
	tx2, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)
	assert.Equal(t, tx.ID, tx2.ID)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "100", wallet.Balance)
}

func TestDebitWallet(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	tx, err := store.DebitWallet(ctx, "user-1", "USDT", "40.00", "wd-1")
	require.NoError(t, err)
	assert.Equal(t, "withdrawal", tx.Type)
	assert.Equal(t, "60", tx.BalanceAfter)

	// idempotent
	tx2, err := store.DebitWallet(ctx, "user-1", "USDT", "40.00", "wd-1")
	assert.NoError(t, err)
	assert.Equal(t, tx.ID, tx2.ID)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assert.Equal(t, "60", wallet.Balance)
}

func TestDebitWalletInsufficientBalance(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "50.00", "dx-1", nil)
	require.NoError(t, err)

	_, err = store.DebitWallet(ctx, "user-1", "USDT", "100.00", "wd-1")
	assert.Error(t, err)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assert.Equal(t, "50", wallet.Balance)
}

func TestCreditWalletInvalidAmount(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "invalid", "dx-1", nil)
	assert.Error(t, err)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assert.Equal(t, "0", wallet.Balance)
}

func TestDebitWalletInvalidAmount(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	_, err = store.DebitWallet(ctx, "user-1", "USDT", "invalid", "wd-1")
	assert.Error(t, err)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assert.Equal(t, "100", wallet.Balance)
}
