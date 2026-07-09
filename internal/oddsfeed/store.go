package oddsfeed

import "context"

type Store interface {
	UpsertSport(ctx context.Context, s Sport) (string, error)
	UpsertLeague(ctx context.Context, l League) (string, error)
	UpsertEvent(ctx context.Context, e Event) (string, error)
	UpsertMarket(ctx context.Context, m Market) (string, error)
	UpsertOutcome(ctx context.Context, o Outcome) (string, error)
	UpdateOutcomeOdds(ctx context.Context, provider, providerOutcomeID, odds string) (marketID, outcomeID string, err error)
	UpdateMarketStatus(ctx context.Context, provider, providerMarketID, status string) (marketID string, err error)
	UpdateOutcomeStatus(ctx context.Context, provider, providerOutcomeID, status string) (marketID, outcomeID string, err error)
	ListSports(ctx context.Context, page, pageSize int) ([]Sport, error)
	ListLeagues(ctx context.Context, sportID string, page, pageSize int) ([]League, error)
	ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]Event, error)
	GetEvent(ctx context.Context, id string) (*Event, error)
	ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]Market, error)
	ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]Outcome, error)
	ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) ([]Event, error)
}
