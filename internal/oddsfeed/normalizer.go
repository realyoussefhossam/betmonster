package oddsfeed

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NormalizeSnapshot maps provider snapshot IDs to internal UUIDs and cross-references
// sports -> leagues -> events -> markets -> outcomes.
func NormalizeSnapshot(snap *Snapshot) ([]Sport, []League, []Event, []Market, []Outcome, error) {
	sports := make([]Sport, 0, len(snap.Sports))
	leagues := make([]League, 0, len(snap.Leagues))
	events := make([]Event, 0, len(snap.Events))
	markets := make([]Market, 0, len(snap.Markets))
	outcomes := make([]Outcome, 0, len(snap.Outcomes))

	sportIDs := map[string]string{}
	for _, sp := range snap.Sports {
		id := uuid.NewString()
		sportIDs[sp.ProviderID] = id
		sports = append(sports, Sport{ID: id, Provider: snap.Provider, ProviderSportID: sp.ProviderID, Slug: sp.Slug, Name: sp.Name})
	}

	leagueIDs := map[string]string{}
	for _, l := range snap.Leagues {
		id := uuid.NewString()
		leagueIDs[l.ProviderID] = id
		leagues = append(leagues, League{ID: id, Provider: snap.Provider, ProviderLeagueID: l.ProviderID, SportID: sportIDs[l.SportID], Name: l.Name, Country: l.Country})
	}

	eventIDs := map[string]string{}
	for _, e := range snap.Events {
		id := uuid.NewString()
		eventIDs[e.ProviderID] = id
		startsAt, err := time.Parse(time.RFC3339, e.StartsAt)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("parse event starts_at: %w", err)
		}
		var scoreUpdatedAt time.Time
		if e.ScoreUpdatedAt != "" {
			scoreUpdatedAt, err = time.Parse(time.RFC3339, e.ScoreUpdatedAt)
			if err != nil {
				return nil, nil, nil, nil, nil, fmt.Errorf("parse event score_updated_at: %w", err)
			}
		}
		events = append(events, Event{
			ID: id, Provider: snap.Provider, ProviderEventID: e.ProviderID,
			LeagueID: leagueIDs[e.LeagueID], SportID: sportIDs[e.SportID],
			HomeParticipant: e.HomeParticipant, AwayParticipant: e.AwayParticipant,
			StartsAt: startsAt, Status: e.Status,
			HomeScore: e.HomeScore, AwayScore: e.AwayScore, ScoreUpdatedAt: scoreUpdatedAt, Metadata: e.Metadata,
		})
	}

	marketIDs := map[string]string{}
	for _, m := range snap.Markets {
		id := uuid.NewString()
		marketIDs[m.ProviderID] = id
		markets = append(markets, Market{ID: id, Provider: snap.Provider, ProviderMarketID: m.ProviderID, EventID: eventIDs[m.EventID], Type: m.Type, Name: m.Name, Line: m.Line, Status: m.Status, Metadata: m.Metadata})
	}

	for _, o := range snap.Outcomes {
		outcomes = append(outcomes, Outcome{
			ID: uuid.NewString(), Provider: snap.Provider, ProviderOutcomeID: o.ProviderID,
			MarketID: marketIDs[o.MarketID], Name: o.Name, Odds: o.Odds, Status: o.Status, Metadata: o.Metadata,
		})
	}
	return sports, leagues, events, markets, outcomes, nil
}
