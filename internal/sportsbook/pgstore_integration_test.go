//go:build integration

package sportsbook

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDSN string

func init() {
	testDSN = os.Getenv("TEST_DATABASE_URL")
	if testDSN == "" {
		testDSN = "postgres://wallet:wallet@localhost:5433/sportsbook?sslmode=disable"
	}
}

func setupSportsbookPGStore(t *testing.T) (*PGStore, func()) {
	t.Helper()

	db, err := sql.Open("pgx", testDSN)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = db.ExecContext(ctx, "TRUNCATE TABLE bets RESTART IDENTITY CASCADE;")
	require.NoError(t, err)

	return NewPGStore(db), func() { db.Close() }
}

func TestPGStoreCreateAndGetBet(t *testing.T) {
	store, cleanup := setupSportsbookPGStore(t)
	defer cleanup()

	ctx := context.Background()
	bet := Bet{
		UserID:          "user-1",
		EventID:         uuid.New().String(),
		MarketID:        uuid.New().String(),
		OutcomeID:       uuid.New().String(),
		Odds:            "2.50",
		Stake:           "10.00",
		PotentialPayout: "25",
		Currency:        "USDT",
		Status:          StatusPending,
		ReferenceID:     "ref-pg-1",
		CreatedAt:       time.Now(),
	}

	id, err := store.CreateBet(ctx, bet)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	got, err := store.GetBet(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "user-1", got.UserID)
	assert.Equal(t, StatusPending, got.Status)
}

func TestPGStoreGetBetByReference(t *testing.T) {
	store, cleanup := setupSportsbookPGStore(t)
	defer cleanup()

	ctx := context.Background()
	bet := Bet{
		UserID:          "user-1",
		EventID:         uuid.New().String(),
		MarketID:        uuid.New().String(),
		OutcomeID:       uuid.New().String(),
		Odds:            "2.00",
		Stake:           "5.00",
		PotentialPayout: "10",
		Currency:        "USDT",
		Status:          StatusPending,
		ReferenceID:     "ref-pg-2",
		CreatedAt:       time.Now(),
	}
	_, err := store.CreateBet(ctx, bet)
	require.NoError(t, err)

	got, err := store.GetBetByReference(ctx, "user-1", "ref-pg-2")
	require.NoError(t, err)
	assert.Equal(t, "ref-pg-2", got.ReferenceID)

	_, err = store.GetBetByReference(ctx, "user-1", "ref-missing")
	assert.ErrorIs(t, err, ErrBetNotFound)
}

func TestPGStoreUniqueReference(t *testing.T) {
	store, cleanup := setupSportsbookPGStore(t)
	defer cleanup()

	ctx := context.Background()
	bet := Bet{
		UserID:          "user-1",
		EventID:         uuid.New().String(),
		MarketID:        uuid.New().String(),
		OutcomeID:       uuid.New().String(),
		Odds:            "2.00",
		Stake:           "5.00",
		PotentialPayout: "10",
		Currency:        "USDT",
		Status:          StatusPending,
		ReferenceID:     "ref-pg-dup",
		CreatedAt:       time.Now(),
	}
	_, err := store.CreateBet(ctx, bet)
	require.NoError(t, err)

	_, err = store.CreateBet(ctx, bet)
	assert.Error(t, err)
}

func TestPGStoreListBets(t *testing.T) {
	store, cleanup := setupSportsbookPGStore(t)
	defer cleanup()

	ctx := context.Background()
	_, err := store.CreateBet(ctx, Bet{
		UserID: "user-1", EventID: uuid.New().String(), MarketID: uuid.New().String(), OutcomeID: uuid.New().String(),
		Odds: "2.00", Stake: "5.00", PotentialPayout: "10", Currency: "USDT",
		Status: StatusPending, ReferenceID: "ref-pg-list-1", CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	_, err = store.CreateBet(ctx, Bet{
		UserID: "user-1", EventID: uuid.New().String(), MarketID: uuid.New().String(), OutcomeID: uuid.New().String(),
		Odds: "2.00", Stake: "5.00", PotentialPayout: "10", Currency: "USDT",
		Status: StatusWon, ReferenceID: "ref-pg-list-2", CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	all, err := store.ListBets(ctx, "user-1", "", 1, 20)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	pending, err := store.ListBets(ctx, "user-1", StatusPending, 1, 20)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, StatusPending, pending[0].Status)
}

func TestPGStoreUpdateBetStatus(t *testing.T) {
	store, cleanup := setupSportsbookPGStore(t)
	defer cleanup()

	ctx := context.Background()
	id, err := store.CreateBet(ctx, Bet{
		UserID: "user-1", EventID: uuid.New().String(), MarketID: uuid.New().String(), OutcomeID: uuid.New().String(),
		Odds: "2.00", Stake: "5.00", PotentialPayout: "10", Currency: "USDT",
		Status: StatusPending, ReferenceID: "ref-pg-update", CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	now := time.Now()
	err = store.UpdateBetStatus(ctx, id, StatusWon, now)
	require.NoError(t, err)

	got, err := store.GetBet(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, StatusWon, got.Status)
	assert.NotNil(t, got.SettledAt)
}

func TestPGStoreUpdateBetStatusPreservesSettledAt(t *testing.T) {
	store, cleanup := setupSportsbookPGStore(t)
	defer cleanup()

	ctx := context.Background()
	id, err := store.CreateBet(ctx, Bet{
		UserID: "user-1", EventID: uuid.New().String(), MarketID: uuid.New().String(), OutcomeID: uuid.New().String(),
		Odds: "2.00", Stake: "5.00", PotentialPayout: "10", Currency: "USDT",
		Status: StatusPending, ReferenceID: "ref-pg-update-preserve", CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	settledAt := time.Now().UTC().Truncate(time.Millisecond)
	err = store.UpdateBetStatus(ctx, id, StatusWon, settledAt)
	require.NoError(t, err)

	// A subsequent update with a zero settled_at must not overwrite the existing value.
	err = store.UpdateBetStatus(ctx, id, StatusWon, time.Time{})
	require.NoError(t, err)

	got, err := store.GetBet(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got.SettledAt)
	assert.Equal(t, settledAt, got.SettledAt.UTC().Truncate(time.Millisecond))
}

func TestPGStoreListPendingBets(t *testing.T) {
	store, cleanup := setupSportsbookPGStore(t)
	defer cleanup()

	ctx := context.Background()
	_, err := store.CreateBet(ctx, Bet{
		UserID: "user-1", EventID: uuid.New().String(), MarketID: uuid.New().String(), OutcomeID: uuid.New().String(),
		Odds: "2.00", Stake: "5.00", PotentialPayout: "10", Currency: "USDT",
		Status: StatusPending, ReferenceID: "ref-pg-pending-1", CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	_, err = store.CreateBet(ctx, Bet{
		UserID: "user-2", EventID: uuid.New().String(), MarketID: uuid.New().String(), OutcomeID: uuid.New().String(),
		Odds: "2.00", Stake: "5.00", PotentialPayout: "10", Currency: "USDT",
		Status: StatusWon, ReferenceID: "ref-pg-pending-2", CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	pending, err := store.ListPendingBets(ctx, 1, 20)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, StatusPending, pending[0].Status)
}
