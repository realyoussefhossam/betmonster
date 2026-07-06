package oddsfeed

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// localMockProvider is a test-only provider that returns the same deterministic
// snapshot as the external mock provider without creating an import cycle.
type localMockProvider struct{}

func (p *localMockProvider) Name() string { return "mock" }

func (p *localMockProvider) FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*Snapshot, error) {
	now := time.Now()
	return &Snapshot{
		Provider: "mock",
		Sports:   []SportSnapshot{{ProviderID: "mock-sp-1", Slug: "soccer", Name: "Soccer"}},
		Leagues:  []LeagueSnapshot{{ProviderID: "mock-lg-1", SportID: "mock-sp-1", Name: "Mock League", Country: "Mockland"}},
		Events: []EventSnapshot{{
			ProviderID: "mock-ev-1", LeagueID: "mock-lg-1", SportID: "mock-sp-1",
			HomeParticipant: "Mock FC", AwayParticipant: "Test United",
			StartsAt: now.Add(2 * time.Hour).Format(time.RFC3339), Status: "upcoming",
		}},
		Markets: []MarketSnapshot{{
			ProviderID: "mock-mk-1", EventID: "mock-ev-1", Type: "1x2", Name: "Match Result", Status: "active",
		}},
		Outcomes: []OutcomeSnapshot{
			{ProviderID: "mock-oc-1", MarketID: "mock-mk-1", Name: "Home", Odds: "2.10", Status: "active"},
			{ProviderID: "mock-oc-2", MarketID: "mock-mk-1", Name: "Draw", Odds: "3.40", Status: "active"},
			{ProviderID: "mock-oc-3", MarketID: "mock-mk-1", Name: "Away", Odds: "3.20", Status: "active"},
		},
	}, nil
}

func (p *localMockProvider) SubscribeLive(ctx context.Context, sport string, updates chan<- Update) error {
	<-ctx.Done()
	return ctx.Err()
}

func (p *localMockProvider) ValidateConfig(cfg ProviderConfig) error { return nil }

func TestServiceSyncProvider(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store, []FeedProvider{&localMockProvider{}}, nil, nil, slog.Default())
	if err := svc.SyncProvider(context.Background(), "mock"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(store.sports) != 1 {
		t.Fatalf("expected 1 sport, got %d", len(store.sports))
	}
	if len(store.leagues) != 1 {
		t.Fatalf("expected 1 league, got %d", len(store.leagues))
	}
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	if len(store.markets) != 1 {
		t.Fatalf("expected 1 market, got %d", len(store.markets))
	}
	if len(store.outcomes) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(store.outcomes))
	}

	// Verify cross-references were preserved through normalization.
	sport := store.sports[sportKey("mock", "mock-sp-1")]
	league := store.leagues[leagueKey("mock", "mock-lg-1")]
	event := store.events[eventKey("mock", "mock-ev-1")]
	market := store.markets[marketKey("mock", "mock-mk-1")]
	if event.SportID != sport.ID || event.LeagueID != league.ID {
		t.Fatalf("event cross-references incorrect")
	}
	if market.EventID != event.ID {
		t.Fatalf("market event cross-reference incorrect")
	}
	for _, o := range store.outcomes {
		if o.MarketID != market.ID {
			t.Fatalf("outcome market cross-reference incorrect")
		}
	}
}

func TestServiceSyncProviderUnknown(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store, []FeedProvider{&localMockProvider{}}, nil, nil, slog.Default())
	if err := svc.SyncProvider(context.Background(), "unknown"); err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}

func TestServiceQueryMethods(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store, []FeedProvider{&localMockProvider{}}, nil, nil, slog.Default())
	ctx := context.Background()
	if err := svc.SyncProvider(ctx, "mock"); err != nil {
		t.Fatalf("sync: %v", err)
	}

	sports, err := svc.ListSports(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list sports: %v", err)
	}
	if len(sports) != 1 {
		t.Fatalf("expected 1 sport, got %d", len(sports))
	}

	events, err := svc.ListEvents(ctx, "", "", "", 1, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	markets, err := svc.ListMarkets(ctx, events[0].ID, "", 1, 10)
	if err != nil {
		t.Fatalf("list markets: %v", err)
	}
	if len(markets) != 1 {
		t.Fatalf("expected 1 market, got %d", len(markets))
	}

	outcomes, err := svc.ListOutcomes(ctx, markets[0].ID, "", 1, 10)
	if err != nil {
		t.Fatalf("list outcomes: %v", err)
	}
	if len(outcomes) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(outcomes))
	}
}
