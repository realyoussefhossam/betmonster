package oddsfeed

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Service orchestrates provider sync, live cache updates, event publishing, and queries.
type Service struct {
	store     Store
	providers map[string]FeedProvider
	cache     *Cache
	bus       *EventBus
	logger    *slog.Logger
	syncMu    sync.Mutex
}

// NewService creates a new odds feed service.
func NewService(store Store, providers []FeedProvider, cache *Cache, bus *EventBus, logger *slog.Logger) *Service {
	pm := make(map[string]FeedProvider, len(providers))
	for _, p := range providers {
		pm[p.Name()] = p
	}
	return &Service{store: store, providers: pm, cache: cache, bus: bus, logger: logger}
}

// SyncProvider fetches a full snapshot from the named provider and applies it to the store.
// Concurrent calls are serialized to avoid overlapping syncs for the same provider.
func (s *Service) SyncProvider(ctx context.Context, providerName string) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	p, ok := s.providers[providerName]
	if !ok {
		return fmt.Errorf("unknown provider: %s", providerName)
	}
	snap, err := p.FetchSnapshot(ctx, "", nil)
	if err != nil {
		return fmt.Errorf("fetch snapshot: %w", err)
	}
	return s.applySnapshot(ctx, snap)
}

// ApplyUpdate applies a single incremental update from a provider (e.g. live odds change).
func (s *Service) ApplyUpdate(ctx context.Context, u Update) error {
	switch u.Type {
	case "odds":
		marketID, outcomeID, err := s.store.UpdateOutcomeOdds(ctx, u.Provider, u.EntityID, u.Payload["odds"])
		if err != nil {
			return fmt.Errorf("apply odds update: %w", err)
		}
		if s.cache != nil {
			if err := s.cache.SetLiveOdds(ctx, marketID, map[string]string{outcomeID: u.Payload["odds"]}); err != nil {
				s.logger.Warn("cache live odds", slog.String("error", err.Error()))
			}
		}
		s.maybeEmit(ctx, "feed.odds.changed", outcomeID)
	case "status":
		marketID, err := s.store.UpdateMarketStatus(ctx, u.Provider, u.EntityID, u.Payload["status"])
		if err == nil {
			s.maybeEmit(ctx, "feed.market.updated", marketID)
			return nil
		}
		marketID, outcomeID, err := s.store.UpdateOutcomeStatus(ctx, u.Provider, u.EntityID, u.Payload["status"])
		if err != nil {
			return fmt.Errorf("apply status update: %w", err)
		}
		s.maybeEmit(ctx, "feed.odds.changed", outcomeID)
	default:
		return fmt.Errorf("unknown update type: %s", u.Type)
	}
	return nil
}

func (s *Service) applySnapshot(ctx context.Context, snap *Snapshot) error {
	sports, leagues, events, markets, outcomes, err := NormalizeSnapshot(snap)
	if err != nil {
		return fmt.Errorf("normalize snapshot: %w", err)
	}
	for _, sp := range sports {
		id, err := s.store.UpsertSport(ctx, sp)
		if err != nil {
			return fmt.Errorf("upsert sport: %w", err)
		}
		s.maybeEmit(ctx, "feed.sport.updated", id)
	}
	for _, l := range leagues {
		id, err := s.store.UpsertLeague(ctx, l)
		if err != nil {
			return fmt.Errorf("upsert league: %w", err)
		}
		s.maybeEmit(ctx, "feed.league.updated", id)
	}
	liveBySport := map[string][]string{}
	for _, e := range events {
		id, err := s.store.UpsertEvent(ctx, e)
		if err != nil {
			return fmt.Errorf("upsert event: %w", err)
		}
		s.maybeEmit(ctx, "feed.event.updated", id)
		if e.Status == "live" && e.SportID != "" {
			liveBySport[e.SportID] = append(liveBySport[e.SportID], id)
		}
	}
	if s.cache != nil {
		for sportID, ids := range liveBySport {
			if err := s.cache.SetLiveEventIDs(ctx, sportID, ids); err != nil {
				s.logger.Warn("cache live events", slog.String("error", err.Error()))
			}
		}
	}
	for _, m := range markets {
		id, err := s.store.UpsertMarket(ctx, m)
		if err != nil {
			return fmt.Errorf("upsert market: %w", err)
		}
		s.maybeEmit(ctx, "feed.market.updated", id)
	}
	for _, o := range outcomes {
		id, err := s.store.UpsertOutcome(ctx, o)
		if err != nil {
			return fmt.Errorf("upsert outcome: %w", err)
		}
		if s.cache != nil {
			if err := s.cache.SetLiveOdds(ctx, o.MarketID, map[string]string{id: o.Odds}); err != nil {
				s.logger.Warn("cache live odds", slog.String("error", err.Error()))
			}
		}
		s.maybeEmit(ctx, "feed.odds.changed", id)
	}
	return nil
}

func (s *Service) maybeEmit(ctx context.Context, subject, entityID string) {
	if s.bus == nil {
		return
	}
	if err := s.bus.Publish(ctx, subject, map[string]string{"id": entityID}); err != nil {
		s.logger.Warn("emit event", slog.String("error", err.Error()))
	}
}

// ListSports returns a paginated list of sports.
func (s *Service) ListSports(ctx context.Context, page, pageSize int) ([]Sport, error) {
	return s.store.ListSports(ctx, page, pageSize)
}

// ListLeagues returns a paginated list of leagues, optionally filtered by sport.
func (s *Service) ListLeagues(ctx context.Context, sportID string, page, pageSize int) ([]League, error) {
	return s.store.ListLeagues(ctx, sportID, page, pageSize)
}

// ListEvents returns a paginated list of events, optionally filtered by sport, league, or status.
func (s *Service) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]Event, error) {
	return s.store.ListEvents(ctx, sportID, leagueID, status, page, pageSize)
}

// GetEvent returns a single event by ID.
func (s *Service) GetEvent(ctx context.Context, id string) (*Event, error) {
	return s.store.GetEvent(ctx, id)
}

// ListMarkets returns a paginated list of markets for an event.
func (s *Service) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]Market, error) {
	return s.store.ListMarkets(ctx, eventID, status, page, pageSize)
}

// ListOutcomes returns a paginated list of outcomes for a market.
func (s *Service) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]Outcome, error) {
	return s.store.ListOutcomes(ctx, marketID, status, page, pageSize)
}

// ListLiveScores returns a paginated list of currently live events with scores.
func (s *Service) ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) ([]Event, error) {
	return s.store.ListLiveScores(ctx, sportID, leagueID, page, pageSize)
}
