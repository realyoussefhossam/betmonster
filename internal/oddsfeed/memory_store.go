package oddsfeed

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type memoryStore struct {
	mu       sync.Mutex
	sports   map[string]*Sport
	leagues  map[string]*League
	events   map[string]*Event
	markets  map[string]*Market
	outcomes map[string]*Outcome
}

func NewInMemoryStore() *memoryStore {
	return &memoryStore{
		sports:   map[string]*Sport{},
		leagues:  map[string]*League{},
		events:   map[string]*Event{},
		markets:  map[string]*Market{},
		outcomes: map[string]*Outcome{},
	}
}

func sportKey(provider, providerSportID string) string {
	return provider + ":" + providerSportID
}

func leagueKey(provider, providerLeagueID string) string {
	return provider + ":" + providerLeagueID
}

func eventKey(provider, providerEventID string) string {
	return provider + ":" + providerEventID
}

func marketKey(provider, providerMarketID string) string {
	return provider + ":" + providerMarketID
}

func outcomeKey(provider, providerOutcomeID string) string {
	return provider + ":" + providerOutcomeID
}

func now() time.Time { return time.Now().UTC() }

func (s *memoryStore) UpsertSport(ctx context.Context, sp Sport) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sportKey(sp.Provider, sp.ProviderSportID)
	if existing, ok := s.sports[key]; ok {
		existing.Slug = sp.Slug
		existing.Name = sp.Name
		existing.UpdatedAt = now()
		return existing.ID, nil
	}
	if sp.ID == "" {
		sp.ID = uuid.NewString()
	}
	sp.CreatedAt = now()
	sp.UpdatedAt = sp.CreatedAt
	s.sports[key] = &sp
	return sp.ID, nil
}

func (s *memoryStore) UpsertLeague(ctx context.Context, l League) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := leagueKey(l.Provider, l.ProviderLeagueID)
	if existing, ok := s.leagues[key]; ok {
		existing.SportID = l.SportID
		existing.Name = l.Name
		existing.Country = l.Country
		existing.UpdatedAt = now()
		return existing.ID, nil
	}
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	l.CreatedAt = now()
	l.UpdatedAt = l.CreatedAt
	s.leagues[key] = &l
	return l.ID, nil
}

func (s *memoryStore) UpsertEvent(ctx context.Context, e Event) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := eventKey(e.Provider, e.ProviderEventID)
	if existing, ok := s.events[key]; ok {
		existing.LeagueID = e.LeagueID
		existing.SportID = e.SportID
		existing.HomeParticipant = e.HomeParticipant
		existing.AwayParticipant = e.AwayParticipant
		existing.StartsAt = e.StartsAt
		existing.Status = e.Status
		existing.HomeScore = e.HomeScore
		existing.AwayScore = e.AwayScore
		existing.ScoreUpdatedAt = e.ScoreUpdatedAt
		existing.Metadata = e.Metadata
		existing.UpdatedAt = now()
		return existing.ID, nil
	}
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	e.CreatedAt = now()
	e.UpdatedAt = e.CreatedAt
	s.events[key] = &e
	return e.ID, nil
}

func (s *memoryStore) UpsertMarket(ctx context.Context, m Market) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := marketKey(m.Provider, m.ProviderMarketID)
	if existing, ok := s.markets[key]; ok {
		existing.EventID = m.EventID
		existing.Type = m.Type
		existing.Name = m.Name
		existing.Line = m.Line
		existing.Status = m.Status
		existing.Metadata = m.Metadata
		existing.UpdatedAt = now()
		return existing.ID, nil
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	m.CreatedAt = now()
	m.UpdatedAt = m.CreatedAt
	s.markets[key] = &m
	return m.ID, nil
}

func (s *memoryStore) UpsertOutcome(ctx context.Context, o Outcome) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := outcomeKey(o.Provider, o.ProviderOutcomeID)
	if existing, ok := s.outcomes[key]; ok {
		existing.MarketID = o.MarketID
		existing.Name = o.Name
		existing.Odds = o.Odds
		existing.Status = o.Status
		existing.Metadata = o.Metadata
		existing.UpdatedAt = now()
		return existing.ID, nil
	}
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	o.CreatedAt = now()
	o.UpdatedAt = o.CreatedAt
	s.outcomes[key] = &o
	return o.ID, nil
}

func (s *memoryStore) ListSports(ctx context.Context, page, pageSize int) ([]Sport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Sport
	for _, sp := range s.sports {
		out = append(out, *sp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return paginate(out, page, pageSize), nil
}

func (s *memoryStore) ListLeagues(ctx context.Context, sportID string, page, pageSize int) ([]League, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []League
	for _, l := range s.leagues {
		if sportID != "" && l.SportID != sportID {
			continue
		}
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return paginate(out, page, pageSize), nil
}

func (s *memoryStore) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, e := range s.events {
		if sportID != "" && e.SportID != sportID {
			continue
		}
		if leagueID != "" && e.LeagueID != leagueID {
			continue
		}
		if status != "" && e.Status != status {
			continue
		}
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.After(out[j].StartsAt) })
	return paginate(out, page, pageSize), nil
}

func (s *memoryStore) GetEvent(ctx context.Context, id string) (*Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e.ID == id {
			copy := *e
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *memoryStore) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]Market, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Market
	for _, m := range s.markets {
		if m.EventID != eventID {
			continue
		}
		if status != "" && m.Status != status {
			continue
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return paginate(out, page, pageSize), nil
}

func (s *memoryStore) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Outcome
	for _, o := range s.outcomes {
		if o.MarketID != marketID {
			continue
		}
		if status != "" && o.Status != status {
			continue
		}
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return paginate(out, page, pageSize), nil
}

func (s *memoryStore) ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) ([]Event, error) {
	return s.ListEvents(ctx, sportID, leagueID, "live", page, pageSize)
}

func paginate[T any](items []T, page, pageSize int) []T {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
