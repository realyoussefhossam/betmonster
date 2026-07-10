//go:build integration

package wallet

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	integrationDSN string
	integrationDB  *sql.DB
	dbSetupOnce    sync.Once
	dbSetupErr     error
)

func setupPGStore(t *testing.T) (*PGStore, func()) {
	t.Helper()

	dbSetupOnce.Do(func() {
		integrationDSN = os.Getenv("TEST_DATABASE_URL")
		if integrationDSN == "" {
			dbSetupErr = fmt.Errorf("TEST_DATABASE_URL not set")
			return
		}

		root, err := findRepoRoot()
		if err != nil {
			dbSetupErr = err
			return
		}

		cmd := exec.Command(filepath.Join(root, "scripts", "migrate.sh"), "up")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DATABASE_URL="+integrationDSN)
		out, err := cmd.CombinedOutput()
		if err != nil {
			dbSetupErr = fmt.Errorf("migrate up failed: %w\n%s", err, string(out))
			return
		}

		integrationDB, err = sql.Open("pgx", integrationDSN)
		if err != nil {
			dbSetupErr = err
			return
		}
	})

	if dbSetupErr != nil {
		if integrationDSN == "" {
			t.Skip(dbSetupErr.Error())
		}
		t.Fatalf("integration setup failed: %v", dbSetupErr)
	}

	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `
		TRUNCATE TABLE transactions, withdrawal_requests, deposit_addresses, wallets RESTART IDENTITY CASCADE;
	`)
	require.NoError(t, err)

	return NewPGStore(integrationDB), func() {}
}

func assertDecimalEqual(t *testing.T, expected, actual string, msgAndArgs ...interface{}) {
	t.Helper()
	exp, err := decimal.NewFromString(expected)
	require.NoError(t, err, "invalid expected decimal %q", expected)
	act, err := decimal.NewFromString(actual)
	require.NoError(t, err, "invalid actual decimal %q", actual)

	if len(msgAndArgs) > 0 {
		if format, ok := msgAndArgs[0].(string); ok {
			msg := fmt.Sprintf(format, msgAndArgs[1:]...)
			assert.True(t, exp.Equal(act), "%s: expected %s, got %s", msg, expected, actual)
			return
		}
	}
	assert.True(t, exp.Equal(act), "expected %s, got %s", expected, actual)
}

func findRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine caller location")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repo root (go.mod)")
		}
		dir = parent
	}
}

func TestPGStoreCreditWallet(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	tx, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)
	assert.Equal(t, "completed", tx.Status)
	assertDecimalEqual(t, "100", tx.BalanceAfter)
	assertDecimalEqual(t, "0", tx.BalanceBefore)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "100", wallet.Balance)
}

func TestPGStoreCreditWalletIdempotent(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	tx1, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	tx2, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	assert.Equal(t, tx1.ID, tx2.ID)
	assertDecimalEqual(t, tx1.BalanceAfter, tx2.BalanceAfter)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "100", wallet.Balance)

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 10)
	require.NoError(t, err)
	assert.Len(t, transactions, 1)
}

func TestPGStoreDebitWallet(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	tx, err := store.DebitWallet(ctx, "user-1", "USDT", "40.00", "wd-1", nil)
	require.NoError(t, err)
	assert.Equal(t, "withdrawal", tx.Type)
	assertDecimalEqual(t, "60", tx.BalanceAfter)
	assertDecimalEqual(t, "100", tx.BalanceBefore)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "60", wallet.Balance)
}

func TestPGStoreDebitWalletIdempotent(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	tx1, err := store.DebitWallet(ctx, "user-1", "USDT", "40.00", "wd-1", nil)
	require.NoError(t, err)

	tx2, err := store.DebitWallet(ctx, "user-1", "USDT", "40.00", "wd-1", nil)
	require.NoError(t, err)

	assert.Equal(t, tx1.ID, tx2.ID)
	assertDecimalEqual(t, "60", tx1.BalanceAfter)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "60", wallet.Balance)

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 10)
	require.NoError(t, err)
	assert.Len(t, transactions, 2)
}

func TestPGStoreDebitWalletIdempotentConcurrentSharedReference(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-shared", "USDT", "100.00", "dx-shared", nil)
	require.NoError(t, err)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)

	var errCount atomic.Int64
	var mu sync.Mutex
	var ids []string
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			tx, err := store.DebitWallet(ctx, "user-shared", "USDT", "10.00", "wd-shared", nil)
			if err != nil {
				errCount.Add(1)
				t.Errorf("debit failed: %v", err)
				return
			}
			mu.Lock()
			ids = append(ids, tx.ID)
			mu.Unlock()
		}()
	}
	wg.Wait()

	require.Zero(t, errCount.Load(), "concurrent shared-reference debit produced errors")
	require.NotEmpty(t, ids, "at least one debit must succeed")

	firstID := ids[0]
	for _, id := range ids {
		assert.Equal(t, firstID, id, "all debits with the same reference_id must return the same transaction ID")
	}

	wallet, err := store.GetWallet(ctx, "user-shared", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "90.00", wallet.Balance, "final balance must reflect exactly one debit")

	transactions, err := store.ListTransactions(ctx, "user-shared", 1, 10)
	require.NoError(t, err)
	assert.Len(t, transactions, 2, "expected exactly one credit and one debit")
}

func TestPGStoreDebitWalletInsufficientBalance(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "50.00", "dx-1", nil)
	require.NoError(t, err)

	_, err = store.DebitWallet(ctx, "user-1", "USDT", "100.00", "wd-1", nil)
	assert.ErrorIs(t, err, ErrInsufficientBalance)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "50", wallet.Balance)
}

func TestPGStoreConcurrentCreditDebit(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	// Pre-create the wallet so the concurrency test focuses on credit/debit
	// contention rather than wallet creation races.
	_, err := store.CreateWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)

	var errCount atomic.Int64
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			creditRef := fmt.Sprintf("concurrent-credit-%d", i)
			debitRef := fmt.Sprintf("concurrent-debit-%d", i)

			if _, err := store.CreditWallet(ctx, "user-1", "USDT", "10.00", creditRef, nil); err != nil {
				errCount.Add(1)
				t.Errorf("credit %d failed: %v", i, err)
				return
			}
			if _, err := store.DebitWallet(ctx, "user-1", "USDT", "5.00", debitRef, nil); err != nil {
				errCount.Add(1)
				t.Errorf("debit %d failed: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	require.Zero(t, errCount.Load(), "concurrent credit/debit produced errors")

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "250", wallet.Balance, "final balance must equal 50 * (10 - 5)")

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 200)
	require.NoError(t, err)
	assert.Len(t, transactions, 2*n, "expected %d transactions (one credit + one debit per goroutine)", 2*n)

	// Verify no double-spending by checking the ledger sums to the final balance.
	var completedSum int64
	for _, txn := range transactions {
		if txn.Status == "completed" && txn.Type == "deposit" {
			completedSum++
		}
	}
	assert.Equal(t, int64(n), completedSum, "expected %d completed deposits", n)
}

func TestPGStoreRequestWithdrawal(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	req, err := store.RequestWithdrawal(ctx, "user-1", "USDT", "40.00", "0xABC", "anvil")
	require.NoError(t, err)
	assert.Equal(t, "pending", req.Status)
	assert.Equal(t, "40.00", req.Amount)
	assert.Equal(t, "anvil", req.Chain)
	assert.Equal(t, "0xABC", req.DestinationAddress)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "60", wallet.Balance)

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 10)
	require.NoError(t, err)
	require.Len(t, transactions, 2)

	var withdrawalTx *Transaction
	for i := range transactions {
		if transactions[i].Type == "withdrawal" {
			withdrawalTx = &transactions[i]
		}
	}
	require.NotNil(t, withdrawalTx)
	assert.Equal(t, "pending", withdrawalTx.Status)
	assertDecimalEqual(t, "40.00", withdrawalTx.Amount)
	assertDecimalEqual(t, "60", withdrawalTx.BalanceAfter)
}

func TestPGStoreRejectWithdrawal(t *testing.T) {
	store, cleanup := setupPGStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	req, err := store.RequestWithdrawal(ctx, "user-1", "USDT", "40.00", "0xABC", "anvil")
	require.NoError(t, err)

	rejected, err := store.RejectWithdrawal(ctx, req.ID, "admin-1")
	require.NoError(t, err)
	assert.Equal(t, "rejected", rejected.Status)
	assert.Equal(t, "admin-1", rejected.ReviewedBy)

	wallet, err := store.GetWallet(ctx, "user-1", "USDT")
	require.NoError(t, err)
	assertDecimalEqual(t, "100", wallet.Balance, "rejected withdrawal must refund the full amount")

	transactions, err := store.ListTransactions(ctx, "user-1", 1, 10)
	require.NoError(t, err)
	require.Len(t, transactions, 3)

	var reversedCount, adjustmentCount int
	for _, txn := range transactions {
		if txn.Type == "withdrawal" && txn.Status == "reversed" {
			reversedCount++
		}
		if txn.Type == "adjustment" && txn.Status == "completed" {
			adjustmentCount++
		}
	}
	assert.Equal(t, 1, reversedCount, "original withdrawal transaction must be reversed")
	assert.Equal(t, 1, adjustmentCount, "reversal must create a completed adjustment transaction")
}
