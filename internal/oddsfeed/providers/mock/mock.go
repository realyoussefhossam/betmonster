package mock

import (
	"context"
	"time"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
)

const ProviderName = "mock"

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return ProviderName }

func (p *Provider) FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
	hier, err := p.FetchHierarchy(ctx, sport, params)
	if err != nil {
		return nil, err
	}
	conds, err := p.FetchConditions(ctx, []string{"mock-ev-1"})
	if err != nil {
		return nil, err
	}
	hier.Markets = conds.Markets
	hier.Outcomes = conds.Outcomes
	return hier, nil
}

func (p *Provider) FetchHierarchy(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
	now := time.Now()
	return &oddsfeed.Snapshot{
		Provider: ProviderName,
		Sports:   []oddsfeed.SportSnapshot{{ProviderID: "mock-sp-1", Slug: "soccer", Name: "Soccer"}},
		Leagues:  []oddsfeed.LeagueSnapshot{{ProviderID: "mock-lg-1", SportID: "mock-sp-1", Name: "Mock League", Country: "Mockland"}},
		Events: []oddsfeed.EventSnapshot{{
			ProviderID: "mock-ev-1", LeagueID: "mock-lg-1", SportID: "mock-sp-1",
			HomeParticipant: "Mock FC", AwayParticipant: "Test United",
			StartsAt: now.Add(2 * time.Hour).Format(time.RFC3339), Status: "upcoming",
		}},
	}, nil
}

func (p *Provider) FetchConditions(ctx context.Context, gameIDs []string) (*oddsfeed.Snapshot, error) {
	return &oddsfeed.Snapshot{
		Provider: ProviderName,
		Events: []oddsfeed.EventSnapshot{{
			ProviderID: "mock-ev-1", SportID: "mock-sp-1", LeagueID: "mock-lg-1", Status: "upcoming",
		}},
		Markets: []oddsfeed.MarketSnapshot{{
			ProviderID: "mock-mk-1", EventID: "mock-ev-1", Type: "1x2", Name: "Match Result", Status: "active",
		}},
		Outcomes: []oddsfeed.OutcomeSnapshot{
			{ProviderID: "mock-oc-1", MarketID: "mock-mk-1", Name: "Home", Odds: "2.10", Status: "active"},
			{ProviderID: "mock-oc-2", MarketID: "mock-mk-1", Name: "Draw", Odds: "3.40", Status: "active"},
			{ProviderID: "mock-oc-3", MarketID: "mock-mk-1", Name: "Away", Odds: "3.20", Status: "active"},
		},
	}, nil
}

func (p *Provider) SubscribeLive(ctx context.Context, sport string, updates chan<- oddsfeed.Update) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			select {
			case updates <- oddsfeed.Update{Provider: ProviderName, Type: "odds", EntityID: "mock-oc-1", Payload: map[string]string{"odds": "2.15"}}:
			default:
				// drop if channel is full; caller is responsible for buffer size
			}
		}
	}
}

func (p *Provider) ValidateConfig(cfg oddsfeed.ProviderConfig) error { return nil }
