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

type FeedProvider interface {
	Name() string
	FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*Snapshot, error)
	SubscribeLive(ctx context.Context, sport string, updates chan<- Update) error
	ValidateConfig(cfg ProviderConfig) error
}
