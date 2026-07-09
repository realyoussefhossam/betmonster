package oddsfeed

import "context"

type ProviderConfig struct {
	Name     string
	GraphURL string
	WSURL    string
	APIKey   string
	Extra    map[string]string
}

type SportSnapshot struct {
	ProviderID string
	Slug       string
	Name       string
}

type LeagueSnapshot struct {
	ProviderID string
	SportID    string
	Name       string
	Country    string
}

type EventSnapshot struct {
	ProviderID      string
	LeagueID        string
	SportID         string
	HomeParticipant string
	AwayParticipant string
	StartsAt        string
	Status          string
	HomeScore       string
	AwayScore       string
	ScoreUpdatedAt  string
	Metadata        map[string]string
}

type MarketSnapshot struct {
	ProviderID string
	EventID    string
	Type       string
	Name       string
	Line       string
	Status     string
	Metadata   map[string]string
}

type OutcomeSnapshot struct {
	ProviderID string
	MarketID   string
	Name       string
	Odds       string
	Status     string
	Metadata   map[string]string
}

type Snapshot struct {
	Provider string
	Sports   []SportSnapshot
	Leagues  []LeagueSnapshot
	Events   []EventSnapshot
	Markets  []MarketSnapshot
	Outcomes []OutcomeSnapshot
}

type Update struct {
	Provider string
	Type     string
	EntityID string
	Payload  map[string]string
}

// FeedProvider fetches sports betting data from external sources.
type FeedProvider interface {
	// FetchSnapshot returns a full normalized snapshot for the given sport.
	FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*Snapshot, error)
	// FetchHierarchy returns sports, leagues, and events without markets/outcomes.
	// Used for incremental syncs where only changed events need fresh conditions.
	FetchHierarchy(ctx context.Context, sport string, params map[string]string) (*Snapshot, error)
	// FetchConditions returns markets and outcomes for the given game IDs.
	FetchConditions(ctx context.Context, gameIDs []string) (*Snapshot, error)
	// SubscribeLive streams real-time updates for the given sport. The caller must provide a buffered channel.
	SubscribeLive(ctx context.Context, sport string, updates chan<- Update) error
	// ValidateConfig checks whether the provider-specific configuration is valid.
	ValidateConfig(cfg ProviderConfig) error
	// Name returns the provider identifier.
	Name() string
}
