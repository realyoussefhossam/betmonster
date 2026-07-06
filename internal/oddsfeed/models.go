package oddsfeed

import "time"

type Sport struct {
	ID              string
	Provider        string
	ProviderSportID string
	Slug            string
	Name            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type League struct {
	ID               string
	Provider         string
	ProviderLeagueID string
	SportID          string
	Name             string
	Country          string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Event struct {
	ID              string
	Provider        string
	ProviderEventID string
	LeagueID        string
	SportID         string
	HomeParticipant string
	AwayParticipant string
	StartsAt        time.Time
	Status          string
	HomeScore       string
	AwayScore       string
	ScoreUpdatedAt  time.Time
	Metadata        map[string]string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Market struct {
	ID               string
	Provider         string
	ProviderMarketID string
	EventID          string
	Type             string
	Name             string
	Line             string
	Status           string
	Metadata         map[string]string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Outcome struct {
	ID                string
	Provider          string
	ProviderOutcomeID string
	MarketID          string
	Name              string
	Odds              string
	Status            string
	Metadata          map[string]string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
