package sportsbook

import (
	"context"
	"testing"

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

type mockWalletClient struct {
	debits  []walletCall
	credits []walletCall
}

type walletCall struct {
	userID      string
	currency    string
	amount      string
	referenceID string
	metadata    map[string]any
}

func (m *mockWalletClient) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) error {
	m.debits = append(m.debits, walletCall{userID: userID, currency: currency, amount: amount, referenceID: referenceID, metadata: metadata})
	return nil
}

func (m *mockWalletClient) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) error {
	m.credits = append(m.credits, walletCall{userID: userID, currency: currency, amount: amount, referenceID: referenceID, metadata: metadata})
	return nil
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
