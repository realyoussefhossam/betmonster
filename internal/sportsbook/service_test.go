package sportsbook

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*Service, *mockWalletClient, *mockOddsFeedClient, *InMemoryStore) {
	t.Helper()
	store := NewInMemoryStore()
	wallet := &mockWalletClient{}
	oddsfeed := newMockOddsFeedClient()
	return NewService(store, wallet, oddsfeed), wallet, oddsfeed, store
}

func TestPlaceBetLocksOddsAndCalculatesPayout(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	assert.Equal(t, "user-1", bet.UserID)
	assert.Equal(t, "event-1", bet.EventID)
	assert.Equal(t, "market-1", bet.MarketID)
	assert.Equal(t, "outcome-1", bet.OutcomeID)
	assert.Equal(t, "2.50", bet.Odds)
	assert.Equal(t, "10.00", bet.Stake)
	assert.Equal(t, "25", bet.PotentialPayout)
	assert.Equal(t, StatusPending, bet.Status)

	require.Len(t, wallet.debits, 1)
	assert.Equal(t, "user-1", wallet.debits[0].userID)
	assert.Equal(t, "10.00", wallet.debits[0].amount)
	assert.Equal(t, "ref-1", wallet.debits[0].referenceID)
}

func TestPlaceBetIdempotency(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet1, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	bet2, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	assert.Equal(t, bet1.ID, bet2.ID)
	assert.Len(t, wallet.debits, 1) // wallet idempotency means debit only once
}

func TestPlaceBetRequiresPositiveStake(t *testing.T) {
	svc, _, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	_, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "0", "USDT", "ref-1")
	assert.Error(t, err)

	_, err = svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "-5", "USDT", "ref-2")
	assert.Error(t, err)
}

func TestPlaceBetRequiresActiveOutcome(t *testing.T) {
	svc, _, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")
	oddsfeed.outcomes["outcome-1"].Status = "suspended"

	_, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outcome not found or not active")
}

func TestSettleWinCreditsPayout(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	settled, err := svc.SettleBet(context.Background(), bet.ID, StatusWon)
	require.NoError(t, err)

	assert.Equal(t, StatusWon, settled.Status)
	assert.NotNil(t, settled.SettledAt)

	require.Len(t, wallet.credits, 1)
	assert.Equal(t, "25", wallet.credits[0].amount)
	assert.Equal(t, "user-1", wallet.credits[0].userID)
}

func TestSettleLossDoesNotCredit(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	settled, err := svc.SettleBet(context.Background(), bet.ID, StatusLost)
	require.NoError(t, err)

	assert.Equal(t, StatusLost, settled.Status)
	assert.Empty(t, wallet.credits)
}

func TestSettleCancelRefundsStake(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	settled, err := svc.SettleBet(context.Background(), bet.ID, StatusCancelled)
	require.NoError(t, err)

	assert.Equal(t, StatusCancelled, settled.Status)
	require.Len(t, wallet.credits, 1)
	assert.Equal(t, "10.00", wallet.credits[0].amount)
}

func TestSettleIsIdempotent(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	_, err = svc.SettleBet(context.Background(), bet.ID, StatusWon)
	require.NoError(t, err)

	_, err = svc.SettleBet(context.Background(), bet.ID, StatusWon)
	assert.Error(t, err)
	assert.Len(t, wallet.credits, 1) // no second credit
}

func TestAutoSettleFromEvents(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)

	// Event finishes and outcome is marked won.
	oddsfeed.events["event-1"].Status = "finished"
	oddsfeed.outcomes["outcome-1"].Status = StatusWon

	err = svc.AutoSettleFromEvents(context.Background())
	require.NoError(t, err)

	settled, err := svc.GetBet(context.Background(), bet.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusWon, settled.Status)
	require.Len(t, wallet.credits, 1)
	assert.Equal(t, "25", wallet.credits[0].amount)
}

func TestListBetsFiltersByUser(t *testing.T) {
	svc, _, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	_, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-1")
	require.NoError(t, err)
	_, err = svc.PlaceBet(context.Background(), "user-2", "event-1", "market-1", "outcome-1", "5.00", "USDT", "ref-2")
	require.NoError(t, err)

	bets, err := svc.ListBets(context.Background(), "user-1", "", 1, 20)
	require.NoError(t, err)
	assert.Len(t, bets, 1)
	assert.Equal(t, "user-1", bets[0].UserID)
}

func TestPlaceBetRecordsBetBeforeDebit(t *testing.T) {
	store := NewInMemoryStore()
	oddsfeed := newMockOddsFeedClient()
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")
	recording := &recordingDebitWallet{store: store, t: t}
	svc := NewService(store, recording, oddsfeed)

	_, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-debit-record")
	require.Error(t, err)

	assert.True(t, recording.sawDebitPending, "bet should be in debit_pending when DebitWallet is called")

	bet, err := store.GetBetByReference(context.Background(), "user-1", "ref-debit-record")
	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, bet.Status)
}

type recordingDebitWallet struct {
	store           *InMemoryStore
	t               *testing.T
	sawDebitPending bool
}

func (r *recordingDebitWallet) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	bet, err := r.store.GetBetByReference(ctx, userID, referenceID)
	require.NoError(r.t, err)
	if bet.Status == StatusDebitPending {
		r.sawDebitPending = true
	}
	return "", errors.New("insufficient funds")
}

func (r *recordingDebitWallet) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	return "", nil
}

var _ WalletClient = (*recordingDebitWallet)(nil)

func TestPlaceBetReversesWhenDebitFails(t *testing.T) {
	svc, wallet, oddsfeed, store := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")
	wallet.debitErr = errors.New("insufficient funds")

	_, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-debit-fail")
	require.Error(t, err)

	bet, err := store.GetBetByReference(context.Background(), "user-1", "ref-debit-fail")
	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, bet.Status)
}

func TestSettleWinMarksWonBeforeCredit(t *testing.T) {
	svc, wallet, oddsfeed, store := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-settle-order")
	require.NoError(t, err)

	checkStoreWallet := &checkingCreditWallet{mock: wallet, store: store, betID: bet.ID, t: t}
	svcWithCheck := NewService(store, checkStoreWallet, oddsfeed)

	_, err = svcWithCheck.SettleBet(context.Background(), bet.ID, StatusWon)
	require.NoError(t, err)
}

type checkingCreditWallet struct {
	mock  *mockWalletClient
	store *InMemoryStore
	betID string
	t     *testing.T
}

func (c *checkingCreditWallet) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	bet, err := c.store.GetBet(ctx, c.betID)
	require.NoError(c.t, err)
	assert.Equal(c.t, StatusWon, bet.Status)
	return c.mock.CreditWallet(ctx, userID, currency, amount, referenceID, metadata)
}

func (c *checkingCreditWallet) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	return c.mock.DebitWallet(ctx, userID, currency, amount, referenceID, metadata)
}

var _ WalletClient = (*checkingCreditWallet)(nil)

func TestSettleWinIsIdempotent(t *testing.T) {
	svc, wallet, oddsfeed, _ := newTestService(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.50")

	bet, err := svc.PlaceBet(context.Background(), "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-idempotent")
	require.NoError(t, err)

	_, err = svc.SettleBet(context.Background(), bet.ID, StatusWon)
	require.NoError(t, err)

	_, err = svc.SettleBet(context.Background(), bet.ID, StatusWon)
	assert.Error(t, err)
	assert.Len(t, wallet.credits, 1) // no second credit
}

func TestSchedulerPaginatesPendingBets(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Now()
	for i := 0; i < 5; i++ {
		_, err := store.CreateBet(context.Background(), Bet{
			ID:              fmt.Sprintf("bet-%d", i),
			UserID:          "user-1",
			EventID:         "event-1",
			MarketID:        "market-1",
			OutcomeID:       "outcome-1",
			Odds:            "2.00",
			Stake:           "1.00",
			PotentialPayout: "2.00",
			Currency:        "USDT",
			Status:          StatusPending,
			ReferenceID:     fmt.Sprintf("ref-page-%d", i),
			CreatedAt:       now.Add(time.Duration(i) * time.Second),
		})
		require.NoError(t, err)
	}

	page1, err := store.ListPendingBets(context.Background(), 1, 2)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := store.ListPendingBets(context.Background(), 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	page3, err := store.ListPendingBets(context.Background(), 3, 2)
	require.NoError(t, err)
	assert.Len(t, page3, 1)

	// Verify ordering and no overlap.
	assert.Equal(t, "bet-0", page1[0].ID)
	assert.Equal(t, "bet-1", page1[1].ID)
	assert.Equal(t, "bet-2", page2[0].ID)
	assert.Equal(t, "bet-3", page2[1].ID)
	assert.Equal(t, "bet-4", page3[0].ID)
}

type mockWalletClient struct {
	debits     []walletCall
	credits    []walletCall
	debitErr   error
	debitTxID  string
	creditTxID string
	creditErr  error
}

type walletCall struct {
	userID      string
	currency    string
	amount      string
	referenceID string
	metadata    map[string]any
}

func (m *mockWalletClient) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	m.debits = append(m.debits, walletCall{userID: userID, currency: currency, amount: amount, referenceID: referenceID, metadata: metadata})
	if m.debitErr != nil {
		return "", m.debitErr
	}
	if m.debitTxID == "" {
		return "debit-tx-" + referenceID, nil
	}
	return m.debitTxID, nil
}

func (m *mockWalletClient) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	m.credits = append(m.credits, walletCall{userID: userID, currency: currency, amount: amount, referenceID: referenceID, metadata: metadata})
	if m.creditErr != nil {
		return "", m.creditErr
	}
	if m.creditTxID == "" {
		return "credit-tx-" + referenceID, nil
	}
	return m.creditTxID, nil
}

type mockOddsFeedClient struct {
	events   map[string]*pb.Event
	markets  map[string]*pb.Market
	outcomes map[string]*pb.Outcome
}

func newMockOddsFeedClient() *mockOddsFeedClient {
	return &mockOddsFeedClient{
		events:   make(map[string]*pb.Event),
		markets:  make(map[string]*pb.Market),
		outcomes: make(map[string]*pb.Outcome),
	}
}

func (m *mockOddsFeedClient) seedEvent(eventID, marketID, outcomeID, odds string) {
	m.events[eventID] = &pb.Event{Id: eventID, Status: "upcoming"}
	m.markets[marketID] = &pb.Market{Id: marketID, EventId: eventID, Status: "active"}
	m.outcomes[outcomeID] = &pb.Outcome{Id: outcomeID, MarketId: marketID, Odds: odds, Status: "active"}
}

func (m *mockOddsFeedClient) GetEvent(ctx context.Context, id string) (*pb.Event, error) {
	return m.events[id], nil
}

func (m *mockOddsFeedClient) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]*pb.Market, error) {
	var out []*pb.Market
	for _, mk := range m.markets {
		if mk.EventId == eventID && (status == "" || mk.Status == status) {
			out = append(out, mk)
		}
	}
	return out, nil
}

func (m *mockOddsFeedClient) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]*pb.Outcome, error) {
	var out []*pb.Outcome
	for _, oc := range m.outcomes {
		if oc.MarketId == marketID && (status == "" || oc.Status == status) {
			out = append(out, oc)
		}
	}
	return out, nil
}

func (m *mockOddsFeedClient) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]*pb.Event, error) {
	var out []*pb.Event
	for _, ev := range m.events {
		if status == "" || ev.Status == status {
			out = append(out, ev)
		}
	}
	return out, nil
}

var _ WalletClient = (*mockWalletClient)(nil)
var _ OddsFeedClient = (*mockOddsFeedClient)(nil)
