package wallet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateWallet(t *testing.T) {
	store := newInMemoryStore()
	wallet, err := store.CreateWallet(context.Background(), "user-1", "USDT")
	assert.NoError(t, err)
	assert.Equal(t, "user-1", wallet.UserID)
	assert.Equal(t, "USDT", wallet.Currency)
	assert.Equal(t, "0", wallet.Balance)
}

func TestCreditWallet(t *testing.T) {
	store := newInMemoryStore()
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
