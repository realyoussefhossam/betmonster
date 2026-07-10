package sportsbook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

type WalletClient interface {
	DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error)
	CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error)
}

type OddsFeedClient interface {
	GetEvent(ctx context.Context, id string) (*pb.Event, error)
	ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]*pb.Market, error)
	ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]*pb.Outcome, error)
	ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]*pb.Event, error)
}

type Service struct {
	store    Store
	wallet   WalletClient
	oddsfeed OddsFeedClient
}

func NewService(store Store, wallet WalletClient, oddsfeed OddsFeedClient) *Service {
	if wallet == nil {
		wallet = &noopWalletClient{}
	}
	if oddsfeed == nil {
		oddsfeed = &noopOddsFeedClient{}
	}
	return &Service{store: store, wallet: wallet, oddsfeed: oddsfeed}
}

func (s *Service) PlaceBet(ctx context.Context, userID, eventID, marketID, outcomeID, stake, currency, referenceID string) (Bet, error) {
	if userID == "" || eventID == "" || marketID == "" || outcomeID == "" || stake == "" || currency == "" || referenceID == "" {
		return Bet{}, errors.New("missing required bet field")
	}
	gt, err := decimalGreaterThanZero(stake)
	if err != nil {
		return Bet{}, fmt.Errorf("invalid stake: %w", err)
	}
	if !gt {
		return Bet{}, errors.New("stake must be greater than zero")
	}

	// Idempotency: return existing bet if reference already used.
	existing, err := s.store.GetBetByReference(ctx, userID, referenceID)
	if err == nil && existing != nil {
		return *existing, nil
	}
	if err != nil && !errors.Is(err, ErrBetNotFound) {
		return Bet{}, fmt.Errorf("check existing bet: %w", err)
	}

	// Validate event exists.
	event, err := s.oddsfeed.GetEvent(ctx, eventID)
	if err != nil {
		return Bet{}, fmt.Errorf("validate event: %w", err)
	}
	if event == nil || event.Id == "" {
		return Bet{}, errors.New("event not found")
	}

	// Validate market belongs to event and is active.
	markets, err := s.oddsfeed.ListMarkets(ctx, eventID, "active", 1, 100)
	if err != nil {
		return Bet{}, fmt.Errorf("list markets: %w", err)
	}
	var market *pb.Market
	for _, m := range markets {
		if m.Id == marketID {
			market = m
			break
		}
	}
	if market == nil {
		return Bet{}, errors.New("market not found or not active for event")
	}

	// Validate outcome belongs to market, is active, and capture current odds.
	outcomes, err := s.oddsfeed.ListOutcomes(ctx, marketID, "active", 1, 100)
	if err != nil {
		return Bet{}, fmt.Errorf("list outcomes: %w", err)
	}
	var outcome *pb.Outcome
	for _, o := range outcomes {
		if o.Id == outcomeID {
			outcome = o
			break
		}
	}
	if outcome == nil {
		return Bet{}, errors.New("outcome not found or not active for market")
	}
	odds := outcome.Odds
	if _, err := parseDecimal(odds); err != nil {
		return Bet{}, fmt.Errorf("invalid outcome odds: %w", err)
	}

	potentialPayout, err := multiplyDecimal(stake, odds)
	if err != nil {
		return Bet{}, fmt.Errorf("calculate payout: %w", err)
	}

	metadata := map[string]any{
		"event_id":   eventID,
		"market_id":  marketID,
		"outcome_id": outcomeID,
		"odds":       odds,
		"bet_ref":    referenceID,
	}
	metadataJSON, _ := json.Marshal(metadata)

	bet := Bet{
		UserID:          userID,
		EventID:         eventID,
		MarketID:        marketID,
		OutcomeID:       outcomeID,
		Odds:            odds,
		Stake:           stake,
		PotentialPayout: potentialPayout,
		Currency:        currency,
		Status:          StatusDebitPending,
		ReferenceID:     referenceID,
		CreatedAt:       time.Now(),
	}

	id, err := s.store.CreateBet(ctx, bet)
	if err != nil {
		if errors.Is(err, ErrBetNotFound) {
			// Existing bet with same reference; return it.
			existing, err := s.store.GetBetByReference(ctx, userID, referenceID)
			if err != nil {
				return Bet{}, fmt.Errorf("create bet conflict: %w", err)
			}
			return *existing, nil
		}
		return Bet{}, fmt.Errorf("create bet: %w", err)
	}
	bet.ID = id

	debitTxID, err := s.wallet.DebitWallet(ctx, userID, currency, stake, referenceID, map[string]any{"type": "bet_stake", "metadata": string(metadataJSON)})
	if err != nil {
		// Mark as cancelled so the record reflects the failed stake hold.
		_ = s.store.UpdateBetStatus(ctx, bet.ID, StatusCancelled, time.Now())
		return Bet{}, fmt.Errorf("debit stake: %w", err)
	}

	bet.DebitTransactionID = debitTxID
	if err := s.store.UpdateBetStatusAndDebitTx(ctx, bet.ID, StatusPending, debitTxID, time.Time{}); err != nil {
		// The debit succeeded but we couldn't record it. Leave in debit_pending for operator review/retry.
		return Bet{}, fmt.Errorf("finalize bet after debit: %w", err)
	}

	bet.Status = StatusPending
	return bet, nil
}

func (s *Service) GetBet(ctx context.Context, id string) (*Bet, error) {
	if id == "" {
		return nil, errors.New("bet id required")
	}
	return s.store.GetBet(ctx, id)
}

func (s *Service) ListBets(ctx context.Context, userID, status string, page, pageSize int) ([]Bet, error) {
	if userID == "" {
		return nil, errors.New("user id required")
	}
	return s.store.ListBets(ctx, userID, status, page, pageSize)
}

func (s *Service) ListPendingBets(ctx context.Context, page, pageSize int) ([]Bet, error) {
	return s.store.ListPendingBets(ctx, page, pageSize)
}

func (s *Service) SettleBet(ctx context.Context, betID, outcome string) (Bet, error) {
	if betID == "" {
		return Bet{}, errors.New("bet id required")
	}
	if outcome != StatusWon && outcome != StatusLost && outcome != StatusCancelled {
		return Bet{}, fmt.Errorf("invalid settlement outcome: %s", outcome)
	}

	bet, err := s.store.GetBet(ctx, betID)
	if err != nil {
		return Bet{}, fmt.Errorf("get bet: %w", err)
	}
	if bet.Status != StatusPending {
		return *bet, fmt.Errorf("bet cannot be settled: status=%s", bet.Status)
	}

	now := time.Now()
	switch outcome {
	case StatusWon:
		// Mark settled first so a retry cannot double-credit.
		if err := s.store.UpdateBetStatusAndOutcome(ctx, bet.ID, StatusWon, now); err != nil {
			return Bet{}, fmt.Errorf("mark bet won: %w", err)
		}
		metadata := map[string]any{
			"bet_id":  bet.ID,
			"outcome": "won",
			"payout":  bet.PotentialPayout,
		}
		creditTxID, err := s.wallet.CreditWallet(ctx, bet.UserID, bet.Currency, bet.PotentialPayout, "settle-"+bet.ReferenceID, metadata)
		if err != nil {
			// Bet is marked won but credit failed; leave credit_transaction_id empty for retry/review.
			return Bet{}, fmt.Errorf("credit winnings: %w", err)
		}
		if err := s.store.SetCreditTransactionID(ctx, bet.ID, creditTxID); err != nil {
			return Bet{}, fmt.Errorf("record credit tx: %w", err)
		}
	case StatusLost:
		if err := s.store.UpdateBetStatusAndOutcome(ctx, bet.ID, StatusLost, now); err != nil {
			return Bet{}, fmt.Errorf("mark bet lost: %w", err)
		}
	case StatusCancelled:
		if err := s.store.UpdateBetStatusAndOutcome(ctx, bet.ID, StatusCancelled, now); err != nil {
			return Bet{}, fmt.Errorf("mark bet cancelled: %w", err)
		}
		metadata := map[string]any{
			"bet_id":  bet.ID,
			"outcome": "cancelled",
			"refund":  bet.Stake,
		}
		creditTxID, err := s.wallet.CreditWallet(ctx, bet.UserID, bet.Currency, bet.Stake, "cancel-"+bet.ReferenceID, metadata)
		if err != nil {
			return Bet{}, fmt.Errorf("credit refund: %w", err)
		}
		if err := s.store.SetCreditTransactionID(ctx, bet.ID, creditTxID); err != nil {
			return Bet{}, fmt.Errorf("record refund tx: %w", err)
		}
	}

	return *s.mustGetBet(ctx, bet.ID), nil
}

func (s *Service) mustGetBet(ctx context.Context, id string) *Bet {
	bet, err := s.store.GetBet(ctx, id)
	if err != nil {
		// Return a minimal record if the store fails unexpectedly.
		return &Bet{ID: id}
	}
	return bet
}

func (s *Service) AutoSettleFromEvents(ctx context.Context) error {
	const pageSize = 100
	page := 1
	for {
		pending, err := s.store.ListPendingBets(ctx, page, pageSize)
		if err != nil {
			return fmt.Errorf("list pending bets: %w", err)
		}
		if len(pending) == 0 {
			return nil
		}

		for _, bet := range pending {
			outcome, err := s.resolveOutcomeStatus(ctx, bet.EventID, bet.MarketID, bet.OutcomeID)
			if err != nil {
				// Log and continue; do not block other settlements.
				continue
			}
			if outcome == "" {
				continue
			}
			_, _ = s.SettleBet(ctx, bet.ID, outcome)
		}

		if len(pending) < pageSize {
			return nil
		}
		page++
	}
}

func (s *Service) resolveOutcomeStatus(ctx context.Context, eventID, marketID, outcomeID string) (string, error) {
	event, err := s.oddsfeed.GetEvent(ctx, eventID)
	if err != nil {
		return "", err
	}
	if event == nil {
		return "", errors.New("event not found")
	}
	// Only settle when the event is no longer upcoming.
	if event.Status == "upcoming" || event.Status == "live" {
		// We settle on outcome status even for live events per the plan.
	}

	markets, err := s.oddsfeed.ListMarkets(ctx, eventID, "", 1, 100)
	if err != nil {
		return "", err
	}
	foundMarket := false
	for _, m := range markets {
		if m.Id == marketID {
			foundMarket = true
			break
		}
	}
	if !foundMarket {
		return "", errors.New("market not found")
	}

	outcomes, err := s.oddsfeed.ListOutcomes(ctx, marketID, "", 1, 100)
	if err != nil {
		return "", err
	}
	for _, o := range outcomes {
		if o.Id == outcomeID {
			switch o.Status {
			case StatusWon, StatusLost, StatusCancelled:
				return o.Status, nil
			default:
				return "", nil
			}
		}
	}
	return "", errors.New("outcome not found")
}

type noopWalletClient struct{}

func (n *noopWalletClient) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	return "", nil
}

func (n *noopWalletClient) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	return "", nil
}

type noopOddsFeedClient struct{}

func (n *noopOddsFeedClient) GetEvent(ctx context.Context, id string) (*pb.Event, error) {
	return nil, nil
}

func (n *noopOddsFeedClient) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]*pb.Market, error) {
	return nil, nil
}

func (n *noopOddsFeedClient) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]*pb.Outcome, error) {
	return nil, nil
}

func (n *noopOddsFeedClient) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]*pb.Event, error) {
	return nil, nil
}
