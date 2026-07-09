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
		id := deterministicID(snap.Provider, "sport:"+sp.ProviderID)
		sportIDs[sp.ProviderID] = id
		sports = append(sports, Sport{ID: id, Provider: snap.Provider, ProviderSportID: sp.ProviderID, Slug: sp.Slug, Name: sp.Name})
	}

	leagueIDs := map[string]string{}
	for _, l := range snap.Leagues {
		id := deterministicID(snap.Provider, "league:"+l.ProviderID)
		leagueIDs[l.ProviderID] = id
		sportID, ok := sportIDs[l.SportID]
		if !ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("league %s references unknown sport %s", l.ProviderID, l.SportID)
		}
		leagues = append(leagues, League{ID: id, Provider: snap.Provider, ProviderLeagueID: l.ProviderID, SportID: sportID, Name: l.Name, Country: l.Country})
	}

	eventIDs := map[string]string{}
	for _, e := range snap.Events {
		id := deterministicID(snap.Provider, "event:"+e.ProviderID)
		eventIDs[e.ProviderID] = id
		var startsAt time.Time
		if e.StartsAt != "" {
			var err error
			startsAt, err = time.Parse(time.RFC3339, e.StartsAt)
			if err != nil {
				return nil, nil, nil, nil, nil, fmt.Errorf("parse event starts_at: %w", err)
			}
		}
		var scoreUpdatedAt time.Time
		if e.ScoreUpdatedAt != "" {
			var err error
			scoreUpdatedAt, err = time.Parse(time.RFC3339, e.ScoreUpdatedAt)
			if err != nil {
				return nil, nil, nil, nil, nil, fmt.Errorf("parse event score_updated_at: %w", err)
			}
		}
		leagueID := leagueIDs[e.LeagueID]
		sportID := sportIDs[e.SportID]
		events = append(events, Event{
			ID: id, Provider: snap.Provider, ProviderEventID: e.ProviderID,
			LeagueID: leagueID, SportID: sportID,
			HomeParticipant: e.HomeParticipant, AwayParticipant: e.AwayParticipant,
			StartsAt: startsAt, Status: e.Status,
			HomeScore: e.HomeScore, AwayScore: e.AwayScore, ScoreUpdatedAt: scoreUpdatedAt, Metadata: e.Metadata,
		})
	}

	marketIDs := map[string]string{}
	for _, m := range snap.Markets {
		id := deterministicID(snap.Provider, "market:"+m.ProviderID)
		marketIDs[m.ProviderID] = id
		eventID, ok := eventIDs[m.EventID]
		if !ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("market %s references unknown event %s", m.ProviderID, m.EventID)
		}
		markets = append(markets, Market{ID: id, Provider: snap.Provider, ProviderMarketID: m.ProviderID, EventID: eventID, Type: m.Type, Name: m.Name, Line: m.Line, Status: m.Status, Metadata: m.Metadata})
	}

	for _, o := range snap.Outcomes {
		marketID, ok := marketIDs[o.MarketID]
		if !ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("outcome %s references unknown market %s", o.ProviderID, o.MarketID)
		}
		outcomes = append(outcomes, Outcome{
			ID: deterministicID(snap.Provider, "outcome:"+o.ProviderID), Provider: snap.Provider, ProviderOutcomeID: o.ProviderID,
			MarketID: marketID, Name: o.Name, Odds: o.Odds, Status: o.Status, Metadata: o.Metadata,
		})
	}
	return sports, leagues, events, markets, outcomes, nil
}

func deterministicID(provider, key string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(provider+":"+key)).String()
}
