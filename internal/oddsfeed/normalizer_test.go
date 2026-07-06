package oddsfeed

import (
	"testing"
	"time"
)

func TestNormalizeSnapshot(t *testing.T) {
	snap := &Snapshot{
		Provider: "mock",
		Sports:   []SportSnapshot{{ProviderID: "sp-1", Slug: "soccer", Name: "Soccer"}},
		Leagues:  []LeagueSnapshot{{ProviderID: "lg-1", SportID: "sp-1", Name: "League A", Country: "A"}},
		Events: []EventSnapshot{{
			ProviderID: "ev-1", LeagueID: "lg-1", SportID: "sp-1",
			HomeParticipant: "A", AwayParticipant: "B",
			StartsAt: time.Now().Add(time.Hour).Format(time.RFC3339), Status: "upcoming",
		}},
		Markets:  []MarketSnapshot{{ProviderID: "mk-1", EventID: "ev-1", Type: "1x2", Name: "Result", Status: "active"}},
		Outcomes: []OutcomeSnapshot{{ProviderID: "oc-1", MarketID: "mk-1", Name: "A", Odds: "2.00", Status: "active"}},
	}
	sports, leagues, events, markets, outcomes, err := NormalizeSnapshot(snap)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(sports) != 1 || len(leagues) != 1 || len(events) != 1 || len(markets) != 1 || len(outcomes) != 1 {
		t.Fatalf("unexpected counts: %d %d %d %d %d", len(sports), len(leagues), len(events), len(markets), len(outcomes))
	}
	if events[0].SportID != sports[0].ID || events[0].LeagueID != leagues[0].ID {
		t.Fatalf("event did not map to correct sport/league")
	}
	if outcomes[0].MarketID != markets[0].ID {
		t.Fatalf("outcome did not map to correct market")
	}
	if outcomes[0].Odds != "2.00" {
		t.Fatalf("expected odds 2.00, got %s", outcomes[0].Odds)
	}
}

func TestNormalizeSnapshotInvalidTime(t *testing.T) {
	snap := &Snapshot{
		Provider: "mock",
		Events: []EventSnapshot{{
			ProviderID:      "ev-1",
			LeagueID:        "lg-1",
			SportID:         "sp-1",
			HomeParticipant: "Team A",
			AwayParticipant: "Team B",
			StartsAt:        "invalid-time",
			Status:          "upcoming",
		}},
	}
	_, _, _, _, _, err := NormalizeSnapshot(snap)
	if err == nil {
		t.Fatal("expected error for invalid time format")
	}
}
