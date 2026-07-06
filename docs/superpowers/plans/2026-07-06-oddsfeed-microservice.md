# Odds/Feed Microservice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the second BetMonster microservice that ingests, normalizes, and serves sports fixtures, odds, markets, and live scores via gRPC, with Azuro as the first provider and a pluggable adapter model.

**Architecture:** Go service (`cmd/oddsfeed`) with Postgres feeds store, Redis live cache, NATS event bus, gRPC internal API, and a `FeedProvider` adapter interface. v2 ships Azuro and Mock adapters, polling + optional WebSocket ingestion, and read-only gRPC endpoints for the future Sportsbook service.

**Tech Stack:** Go 1.26+, protobuf/gRPC, Postgres, golang-migrate, Redis, NATS, Docker Compose.

---

## File Map

| File | Responsibility |
|---|---|
| `internal/proto/oddsfeed.proto` | gRPC contract |
| `internal/oddsfeed/migrations/*.sql` | Postgres schema |
| `internal/shared/config/oddsfeed.go` | Env config loader |
| `internal/oddsfeed/provider.go` | `FeedProvider` interface and snapshot types |
| `internal/oddsfeed/models.go` | Internal entity types |
| `internal/oddsfeed/providers/mock/mock.go` | Deterministic mock provider |
| `internal/oddsfeed/providers/azuro/azuro.go` | Azuro adapter (v2) |
| `internal/oddsfeed/store.go` | `Store` interface |
| `internal/oddsfeed/pgstore.go` | Postgres store |
| `internal/oddsfeed/normalizer.go` | Snapshot → model mapping |
| `internal/oddsfeed/service.go` | Sync, publish, query logic |
| `internal/oddsfeed/server.go` | gRPC server |
| `internal/oddsfeed/scheduler.go` | Polling worker |
| `internal/oddsfeed/websocket.go` | WebSocket live worker |
| `internal/oddsfeed/events.go` | NATS publisher |
| `internal/oddsfeed/cache.go` | Redis cache |
| `cmd/oddsfeed/main.go` | Entrypoint, migrations, gRPC/health server |
| `Dockerfile.oddsfeed` | Docker build |
| `docker-compose.yml` | Add `oddsfeed` service |
| `postgres/init/01-init.sql` | Create `oddsfeed` database |
| `.env.example` | Add env vars |

---

## Task 1: Add Protobuf Contract

**Files:**
- Create: `internal/proto/oddsfeed.proto`

- [ ] **Step 1: Write the .proto file**

```protobuf
syntax = "proto3";
package oddsfeed;
option go_package = "github.com/realyoussefhossam/betmonster/internal/proto";

service OddsFeedService {
  rpc ListSports(ListSportsRequest) returns (ListSportsResponse);
  rpc ListLeagues(ListLeaguesRequest) returns (ListLeaguesResponse);
  rpc ListEvents(ListEventsRequest) returns (ListEventsResponse);
  rpc GetEvent(GetEventRequest) returns (GetEventResponse);
  rpc ListMarkets(ListMarketsRequest) returns (ListMarketsResponse);
  rpc ListOutcomes(ListOutcomesRequest) returns (ListOutcomesResponse);
  rpc ListLiveScores(ListLiveScoresRequest) returns (ListLiveScoresResponse);
}

message Sport { string id = 1; string name = 2; string slug = 3; }
message League { string id = 1; string name = 2; string sport_id = 3; string country = 4; }
message Event {
  string id = 1; string league_id = 2; string sport_id = 3;
  string home_participant = 4; string away_participant = 5;
  string starts_at = 6; string status = 7;
  string home_score = 8; string away_score = 9; string score_updated_at = 10;
}
message Market { string id = 1; string event_id = 2; string type = 3; string name = 4; string line = 5; string status = 6; }
message Outcome { string id = 1; string market_id = 2; string name = 3; string odds = 4; string status = 5; }

message ListSportsRequest { int32 page = 1; int32 page_size = 2; }
message ListSportsResponse { repeated Sport sports = 1; }
message ListLeaguesRequest { string sport_id = 1; int32 page = 2; int32 page_size = 3; }
message ListLeaguesResponse { repeated League leagues = 1; }
message ListEventsRequest { string sport_id = 1; string league_id = 2; string status = 3; int32 page = 4; int32 page_size = 5; }
message ListEventsResponse { repeated Event events = 1; }
message GetEventRequest { string id = 1; }
message GetEventResponse { Event event = 1; }
message ListMarketsRequest { string event_id = 1; string status = 2; int32 page = 3; int32 page_size = 4; }
message ListMarketsResponse { repeated Market markets = 1; }
message ListOutcomesRequest { string market_id = 1; string status = 2; int32 page = 3; int32 page_size = 4; }
message ListOutcomesResponse { repeated Outcome outcomes = 1; }
message ListLiveScoresRequest { string sport_id = 1; string league_id = 2; int32 page = 3; int32 page_size = 4; }
message ListLiveScoresResponse { repeated Event events = 1; }
```

- [ ] **Step 2: Generate Go code**

Run: `cd /home/joseph/documents/dev/better-auth-go && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative internal/proto/oddsfeed.proto`

Expected: `internal/proto/oddsfeed.pb.go` and `internal/proto/oddsfeed_grpc.pb.go` created.

- [ ] **Step 3: Commit**

```bash
git add internal/proto/oddsfeed.proto internal/proto/oddsfeed*.go
git commit -m "feat(oddsfeed): add gRPC protobuf contract"
```

---

## Task 2: Database Migrations

**Files:**
- Create: `internal/oddsfeed/migrations/20260706120000_create_feeds_schema.up.sql`
- Create: `internal/oddsfeed/migrations/20260706120000_create_feeds_schema.down.sql`

- [ ] **Step 1: Write up migration**

```sql
CREATE TABLE IF NOT EXISTS sports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_sport_id text NOT NULL,
  slug text NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_sport_id)
);

CREATE TABLE IF NOT EXISTS leagues (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_league_id text NOT NULL,
  sport_id uuid NOT NULL REFERENCES sports(id),
  name text NOT NULL,
  country text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_league_id)
);

CREATE TABLE IF NOT EXISTS events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_event_id text NOT NULL,
  league_id uuid NOT NULL REFERENCES leagues(id),
  sport_id uuid NOT NULL REFERENCES sports(id),
  home_participant text NOT NULL,
  away_participant text NOT NULL,
  starts_at timestamptz NOT NULL,
  status text NOT NULL CHECK (status IN ('upcoming', 'live', 'paused', 'finished', 'cancelled', 'postponed')),
  home_score text,
  away_score text,
  score_updated_at timestamptz,
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_event_id)
);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
CREATE INDEX IF NOT EXISTS idx_events_starts_at ON events(starts_at);
CREATE INDEX IF NOT EXISTS idx_events_sport_league ON events(sport_id, league_id);

CREATE TABLE IF NOT EXISTS markets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_market_id text NOT NULL,
  event_id uuid NOT NULL REFERENCES events(id),
  type text NOT NULL,
  name text NOT NULL,
  line text,
  status text NOT NULL CHECK (status IN ('active', 'suspended', 'settled', 'cancelled')),
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_market_id)
);
CREATE INDEX IF NOT EXISTS idx_markets_event_id ON markets(event_id);

CREATE TABLE IF NOT EXISTS outcomes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_outcome_id text NOT NULL,
  market_id uuid NOT NULL REFERENCES markets(id),
  name text NOT NULL,
  odds text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'suspended', 'won', 'lost', 'cancelled')),
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_outcome_id)
);
CREATE INDEX IF NOT EXISTS idx_outcomes_market_id ON outcomes(market_id);

CREATE TABLE IF NOT EXISTS odds_snapshots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id uuid NOT NULL REFERENCES events(id),
  market_id uuid NOT NULL REFERENCES markets(id),
  snapshot_at timestamptz NOT NULL DEFAULT now(),
  odds jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_odds_snapshots_event ON odds_snapshots(event_id, snapshot_at);
```

- [ ] **Step 2: Write down migration**

```sql
DROP TABLE IF EXISTS odds_snapshots;
DROP TABLE IF EXISTS outcomes;
DROP TABLE IF EXISTS markets;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS leagues;
DROP TABLE IF EXISTS sports;
```

- [ ] **Step 3: Verify migrations run**

Run: `createdb -h localhost -p 5433 -U wallet oddsfeed` then `go run github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1 -path internal/oddsfeed/migrations -database 'postgres://wallet:wallet@localhost:5433/oddsfeed?sslmode=disable' up`

Expected: success or `no change`.

- [ ] **Step 4: Commit**

```bash
git add internal/oddsfeed/migrations
git commit -m "feat(oddsfeed): add feeds store migrations"
```

---

## Task 3: Add Config Loader

**Files:**
- Create: `internal/shared/config/oddsfeed.go`

- [ ] **Step 1: Write config**

```go
package config

import "strconv"

type OddsFeed struct {
	Port                  string
	GRPCPort              string
	DatabaseURL           string
	RedisAddr             string
	NATSURL               string
	Providers             string
	AzuroGraphURL         string
	AzuroWSURL            string
	SyncIntervalSeconds   int
	WSReconnectMaxSeconds int
}

func LoadOddsFeed() OddsFeed {
	return OddsFeed{
		Port:                  getEnv("PORT", "8082"),
		GRPCPort:              getEnv("GRPC_PORT", "50052"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://wallet:wallet@localhost:5433/oddsfeed?sslmode=disable"),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		NATSURL:               getEnv("NATS_URL", "nats://localhost:4222"),
		Providers:             getEnv("PROVIDERS", "mock"),
		AzuroGraphURL:         getEnv("AZURO_GRAPH_URL", ""),
		AzuroWSURL:            getEnv("AZURO_WS_URL", ""),
		SyncIntervalSeconds:   getEnvInt("SYNC_INTERVAL_SECONDS", 60),
		WSReconnectMaxSeconds: getEnvInt("WS_RECONNECT_MAX_SECONDS", 300),
	}
}

func getEnvInt(key string, fallback int) int {
	v := getEnv(key, "")
	if v == "" { return fallback }
	n, err := strconv.Atoi(v)
	if err != nil { return fallback }
	return n
}
```

- [ ] **Step 2: Verify compile**

Run: `go build ./internal/shared/config/...`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/shared/config/oddsfeed.go
git commit -m "feat(oddsfeed): add config loader"
```

---

## Task 4: Provider Adapter Interface and Mock

**Files:**
- Create: `internal/oddsfeed/provider.go`
- Create: `internal/oddsfeed/models.go`
- Create: `internal/oddsfeed/providers/mock/mock.go`
- Create: `internal/oddsfeed/provider_test.go`

- [ ] **Step 1: Write provider interface and types**

```go
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
```

- [ ] **Step 2: Write models.go**

```go
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
```

- [ ] **Step 3: Write mock provider**

```go
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
			updates <- oddsfeed.Update{Provider: ProviderName, Type: "odds", EntityID: "mock-oc-1", Payload: map[string]string{"odds": "2.15"}}
		}
	}
}

func (p *Provider) ValidateConfig(cfg oddsfeed.ProviderConfig) error { return nil }
```

- [ ] **Step 4: Write provider test**

```go
package oddsfeed_test

import (
	"context"
	"testing"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed/providers/mock"
)

func TestMockProviderFetchSnapshot(t *testing.T) {
	p := mock.New()
	if p.Name() != "mock" { t.Fatalf("expected name mock, got %s", p.Name()) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snap, err := p.FetchSnapshot(ctx, "soccer", nil)
	if err != nil { t.Fatalf("fetch snapshot: %v", err) }
	if len(snap.Events) == 0 { t.Fatal("expected events") }
	if len(snap.Outcomes) != 3 { t.Fatalf("expected 3 outcomes, got %d", len(snap.Outcomes)) }
}
```

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/oddsfeed/...`

Expected: PASS.

```bash
git add internal/oddsfeed/provider.go internal/oddsfeed/models.go internal/oddsfeed/providers/mock/mock.go internal/oddsfeed/provider_test.go
git commit -m "feat(oddsfeed): add provider interface and mock adapter"
```

---

## Task 5: Store Layer

**Files:**
- Create: `internal/oddsfeed/store.go`
- Create: `internal/oddsfeed/pgstore.go`
- Create: `internal/oddsfeed/store_test.go`

- [ ] **Step 1: Write Store interface**

```go
package oddsfeed

import "context"

type Store interface {
	UpsertSport(ctx context.Context, s Sport) (string, error)
	UpsertLeague(ctx context.Context, l League) (string, error)
	UpsertEvent(ctx context.Context, e Event) (string, error)
	UpsertMarket(ctx context.Context, m Market) (string, error)
	UpsertOutcome(ctx context.Context, o Outcome) (string, error)
	ListSports(ctx context.Context, page, pageSize int) ([]Sport, error)
	ListLeagues(ctx context.Context, sportID string, page, pageSize int) ([]League, error)
	ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]Event, error)
	GetEvent(ctx context.Context, id string) (*Event, error)
	ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]Market, error)
	ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]Outcome, error)
	ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) ([]Event, error)
}
```

- [ ] **Step 2: Implement PGStore**

Implement `internal/oddsfeed/pgstore.go` with idempotent upserts for all entities. Use `uuid.MustParse` for foreign key references. Follow the same SQL pattern as the wallet `pgstore.go`. Include `ListEvents` with optional filters, and use `ListEvents(..., "live", ...)` for `ListLiveScores`.

- [ ] **Step 3: Write store tests**

Use a local Postgres test database on `localhost:5433`. Run migrations, then test upsert round-trip and list filters. Follow the wallet `store_test.go` pattern.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./internal/oddsfeed/...`

Expected: PASS.

```bash
git add internal/oddsfeed/store.go internal/oddsfeed/pgstore.go internal/oddsfeed/store_test.go
git commit -m "feat(oddsfeed): add postgres store and tests"
```

---

## Task 6: Normalizer and Service

**Files:**
- Create: `internal/oddsfeed/normalizer.go`
- Create: `internal/oddsfeed/normalizer_test.go`
- Create: `internal/oddsfeed/service.go`
- Create: `internal/oddsfeed/service_test.go`
- Create: `internal/oddsfeed/cache.go`
- Create: `internal/oddsfeed/events.go`

- [ ] **Step 1: Write normalizer**

```go
package oddsfeed

import (
	"time"
	"github.com/google/uuid"
)

func NormalizeSnapshot(snap *Snapshot) ([]Sport, []League, []Event, []Market, []Outcome) {
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
		startsAt, _ := time.Parse(time.RFC3339, e.StartsAt)
		scoreUpdatedAt, _ := time.Parse(time.RFC3339, e.ScoreUpdatedAt)
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
	return sports, leagues, events, markets, outcomes
}
```

- [ ] **Step 2: Write Cache and EventBus**

```go
package oddsfeed

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

func NewCache(addr string) *Cache { return &Cache{client: redis.NewClient(&redis.Options{Addr: addr})} }

func (c *Cache) SetLiveOdds(ctx context.Context, marketID string, odds map[string]string) error {
	return c.client.HSet(ctx, fmt.Sprintf("oddsfeed:live:odds:%s", marketID), odds).Err()
}

func (c *Cache) SetLiveScore(ctx context.Context, eventID, home, away, status string) error {
	return c.client.HSet(ctx, fmt.Sprintf("oddsfeed:live:score:%s", eventID), map[string]string{"home_score": home, "away_score": away, "status": status}).Err()
}

func (c *Cache) Close() error { return c.client.Close() }
```

```go
package oddsfeed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"github.com/nats-io/nats.go"
)

type EventBus struct {
	conn   *nats.Conn
	logger *slog.Logger
}

func NewEventBus(url string, logger *slog.Logger) (*EventBus, error) {
	nc, err := nats.Connect(url)
	if err != nil { return nil, fmt.Errorf("nats connect: %w", err) }
	return &EventBus{conn: nc, logger: logger}, nil
}

func (b *EventBus) Publish(ctx context.Context, subject string, payload map[string]string) error {
	body, _ := json.Marshal(payload)
	if err := b.conn.Publish(subject, body); err != nil { return err }
	b.logger.Debug("published feed event", slog.String("subject", subject))
	return nil
}

func (b *EventBus) Close() { b.conn.Close() }
```

- [ ] **Step 3: Write Service**

```go
package oddsfeed

import (
	"context"
	"fmt"
	"log/slog"
)

type Service struct {
	store     Store
	providers map[string]FeedProvider
	cache     *Cache
	bus       *EventBus
	logger    *slog.Logger
}

func NewService(store Store, providers []FeedProvider, cache *Cache, bus *EventBus, logger *slog.Logger) *Service {
	pm := make(map[string]FeedProvider, len(providers))
	for _, p := range providers { pm[p.Name()] = p }
	return &Service{store: store, providers: pm, cache: cache, bus: bus, logger: logger}
}

func (s *Service) SyncProvider(ctx context.Context, providerName string) error {
	p, ok := s.providers[providerName]
	if !ok { return fmt.Errorf("unknown provider: %s", providerName) }
	snap, err := p.FetchSnapshot(ctx, "", nil)
	if err != nil { return fmt.Errorf("fetch snapshot: %w", err) }
	return s.applySnapshot(ctx, snap)
}

func (s *Service) applySnapshot(ctx context.Context, snap *Snapshot) error {
	sports, leagues, events, markets, outcomes := NormalizeSnapshot(snap)
	for _, sp := range sports {
		id, err := s.store.UpsertSport(ctx, sp)
		if err != nil { return fmt.Errorf("upsert sport: %w", err) }
		s.maybeEmit(ctx, "feed.sport.updated", id)
	}
	for _, l := range leagues {
		id, err := s.store.UpsertLeague(ctx, l)
		if err != nil { return fmt.Errorf("upsert league: %w", err) }
		s.maybeEmit(ctx, "feed.league.updated", id)
	}
	liveBySport := map[string][]string{}
	for _, e := range events {
		id, err := s.store.UpsertEvent(ctx, e)
		if err != nil { return fmt.Errorf("upsert event: %w", err) }
		s.maybeEmit(ctx, "feed.event.updated", id)
		if e.Status == "live" && e.SportID != "" { liveBySport[e.SportID] = append(liveBySport[e.SportID], id) }
	}
	for sportID, ids := range liveBySport {
		if err := s.cache.SetLiveEventIDs(ctx, sportID, ids); err != nil { s.logger.Warn("cache live events", slog.String("error", err.Error())) }
	}
	for _, m := range markets {
		id, err := s.store.UpsertMarket(ctx, m)
		if err != nil { return fmt.Errorf("upsert market: %w", err) }
		s.maybeEmit(ctx, "feed.market.updated", id)
	}
	for _, o := range outcomes {
		id, err := s.store.UpsertOutcome(ctx, o)
		if err != nil { return fmt.Errorf("upsert outcome: %w", err) }
		if err := s.cache.SetLiveOdds(ctx, o.MarketID, map[string]string{id: o.Odds}); err != nil { s.logger.Warn("cache live odds", slog.String("error", err.Error())) }
		s.maybeEmit(ctx, "feed.odds.changed", id)
	}
	return nil
}

func (s *Service) maybeEmit(ctx context.Context, subject, entityID string) {
	if s.bus == nil { return }
	if err := s.bus.Publish(ctx, subject, map[string]string{"id": entityID}); err != nil { s.logger.Warn("emit event", slog.String("error", err.Error())) }
}

func (s *Service) ListSports(ctx context.Context, page, pageSize int) ([]Sport, error) { return s.store.ListSports(ctx, page, pageSize) }
func (s *Service) ListLeagues(ctx context.Context, sportID string, page, pageSize int) ([]League, error) { return s.store.ListLeagues(ctx, sportID, page, pageSize) }
func (s *Service) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]Event, error) { return s.store.ListEvents(ctx, sportID, leagueID, status, page, pageSize) }
func (s *Service) GetEvent(ctx context.Context, id string) (*Event, error) { return s.store.GetEvent(ctx, id) }
func (s *Service) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]Market, error) { return s.store.ListMarkets(ctx, eventID, status, page, pageSize) }
func (s *Service) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]Outcome, error) { return s.store.ListOutcomes(ctx, marketID, status, page, pageSize) }
func (s *Service) ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) ([]Event, error) { return s.store.ListLiveScores(ctx, sportID, leagueID, page, pageSize) }

// Add SetLiveEventIDs to Cache if missing.
func (c *Cache) SetLiveEventIDs(ctx context.Context, sportID string, ids []string) error {
	key := fmt.Sprintf("oddsfeed:live:events:%s", sportID)
	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)
	for _, id := range ids { pipe.SAdd(ctx, key, id) }
	pipe.Expire(ctx, key, 60)
	_, err := pipe.Exec(ctx)
	return err
}
```

- [ ] **Step 4: Write tests for normalizer and service**

Test `NormalizeSnapshot` with the mock snapshot and verify all entity counts and cross-entity IDs. Test `Service.SyncProvider` with a memory store and mock provider and verify upserts and events.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/oddsfeed/...`

Expected: PASS.

```bash
git add internal/oddsfeed/normalizer.go internal/oddsfeed/normalizer_test.go internal/oddsfeed/service.go internal/oddsfeed/service_test.go internal/oddsfeed/cache.go internal/oddsfeed/events.go
git commit -m "feat(oddsfeed): add normalizer, service, cache, and event bus"
```

---

## Task 7: gRPC Server

**Files:**
- Create: `internal/oddsfeed/server.go`
- Create: `internal/oddsfeed/server_test.go`

- [ ] **Step 1: Write gRPC server**

```go
package oddsfeed

import (
	"context"
	"time"
	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

type GRPCServer struct {
	pb.UnimplementedOddsFeedServiceServer
	service *Service
}

func NewGRPCServer(service *Service) *GRPCServer { return &GRPCServer{service: service} }

func toProtoSport(s Sport) *pb.Sport { return &pb.Sport{Id: s.ID, Name: s.Name, Slug: s.Slug} }
func toProtoLeague(l League) *pb.League { return &pb.League{Id: l.ID, Name: l.Name, SportId: l.SportID, Country: l.Country} }
func toProtoEvent(e Event) *pb.Event {
	return &pb.Event{
		Id: e.ID, LeagueId: e.LeagueID, SportId: e.SportID,
		HomeParticipant: e.HomeParticipant, AwayParticipant: e.AwayParticipant,
		StartsAt: e.StartsAt.Format(time.RFC3339), Status: e.Status,
		HomeScore: e.HomeScore, AwayScore: e.AwayScore, ScoreUpdatedAt: e.ScoreUpdatedAt.Format(time.RFC3339),
	}
}
func toProtoMarket(m Market) *pb.Market { return &pb.Market{Id: m.ID, EventId: m.EventID, Type: m.Type, Name: m.Name, Line: m.Line, Status: m.Status} }
func toProtoOutcome(o Outcome) *pb.Outcome { return &pb.Outcome{Id: o.ID, MarketId: o.MarketID, Name: o.Name, Odds: o.Odds, Status: o.Status} }

func (s *GRPCServer) ListSports(ctx context.Context, req *pb.ListSportsRequest) (*pb.ListSportsResponse, error) {
	items, err := s.service.ListSports(ctx, int(req.Page), int(req.PageSize)); if err != nil { return nil, err }
	out := make([]*pb.Sport, len(items)); for i, it := range items { out[i] = toProtoSport(it) }
	return &pb.ListSportsResponse{Sports: out}, nil
}
func (s *GRPCServer) ListLeagues(ctx context.Context, req *pb.ListLeaguesRequest) (*pb.ListLeaguesResponse, error) {
	items, err := s.service.ListLeagues(ctx, req.SportId, int(req.Page), int(req.PageSize)); if err != nil { return nil, err }
	out := make([]*pb.League, len(items)); for i, it := range items { out[i] = toProtoLeague(it) }
	return &pb.ListLeaguesResponse{Leagues: out}, nil
}
func (s *GRPCServer) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	items, err := s.service.ListEvents(ctx, req.SportId, req.LeagueId, req.Status, int(req.Page), int(req.PageSize)); if err != nil { return nil, err }
	out := make([]*pb.Event, len(items)); for i, it := range items { out[i] = toProtoEvent(it) }
	return &pb.ListEventsResponse{Events: out}, nil
}
func (s *GRPCServer) GetEvent(ctx context.Context, req *pb.GetEventRequest) (*pb.GetEventResponse, error) {
	it, err := s.service.GetEvent(ctx, req.Id); if err != nil { return nil, err }
	if it == nil { return nil, fmt.Errorf("event not found") }
	return &pb.GetEventResponse{Event: toProtoEvent(*it)}, nil
}
func (s *GRPCServer) ListMarkets(ctx context.Context, req *pb.ListMarketsRequest) (*pb.ListMarketsResponse, error) {
	items, err := s.service.ListMarkets(ctx, req.EventId, req.Status, int(req.Page), int(req.PageSize)); if err != nil { return nil, err }
	out := make([]*pb.Market, len(items)); for i, it := range items { out[i] = toProtoMarket(it) }
	return &pb.ListMarketsResponse{Markets: out}, nil
}
func (s *GRPCServer) ListOutcomes(ctx context.Context, req *pb.ListOutcomesRequest) (*pb.ListOutcomesResponse, error) {
	items, err := s.service.ListOutcomes(ctx, req.MarketId, req.Status, int(req.Page), int(req.PageSize)); if err != nil { return nil, err }
	out := make([]*pb.Outcome, len(items)); for i, it := range items { out[i] = toProtoOutcome(it) }
	return &pb.ListOutcomesResponse{Outcomes: out}, nil
}
func (s *GRPCServer) ListLiveScores(ctx context.Context, req *pb.ListLiveScoresRequest) (*pb.ListLiveScoresResponse, error) {
	items, err := s.service.ListLiveScores(ctx, req.SportId, req.LeagueId, int(req.Page), int(req.PageSize)); if err != nil { return nil, err }
	out := make([]*pb.Event, len(items)); for i, it := range items { out[i] = toProtoEvent(it) }
	return &pb.ListLiveScoresResponse{Events: out}, nil
}
```

Add `import "fmt"` to server.go.

- [ ] **Step 2: Write gRPC server test**

Start a gRPC server with an in-memory store and mock provider. Call `ListSports` and `ListEvents` and verify counts. Use `bufconn` or a free port.

- [ ] **Step 3: Run tests and commit**

Run: `go test ./internal/oddsfeed/...`

Expected: PASS.

```bash
git add internal/oddsfeed/server.go internal/oddsfeed/server_test.go
git commit -m "feat(oddsfeed): add gRPC server"
```

---

## Task 8: Scheduler and WebSocket Workers

**Files:**
- Create: `internal/oddsfeed/scheduler.go`
- Create: `internal/oddsfeed/websocket.go`

- [ ] **Step 1: Write polling scheduler**

```go
package oddsfeed

import (
	"context"
	"log/slog"
	"time"
)

type Scheduler struct {
	service  *Service
	interval time.Duration
	logger   *slog.Logger
	providers []string
}

func NewScheduler(service *Service, providers []string, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{service: service, providers: providers, interval: interval, logger: logger}
}

func (sch *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(sch.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, p := range sch.providers {
				if err := sch.service.SyncProvider(ctx, p); err != nil {
					sch.logger.Error("sync provider failed", slog.String("provider", p), slog.String("error", err.Error()))
				}
			}
		}
	}
}
```

- [ ] **Step 2: Write WebSocket worker**

```go
package oddsfeed

import (
	"context"
	"log/slog"
	"time"
)

type WebSocketWorker struct {
	service   *Service
	providers map[string]FeedProvider
	logger    *slog.Logger
}

func NewWebSocketWorker(service *Service, providers []FeedProvider, logger *slog.Logger) *WebSocketWorker {
	pm := make(map[string]FeedProvider, len(providers))
	for _, p := range providers { pm[p.Name()] = p }
	return &WebSocketWorker{service: service, providers: pm, logger: logger}
}

func (w *WebSocketWorker) Start(ctx context.Context) {
	for name, p := range w.providers {
		go w.runProvider(ctx, name, p)
	}
}

func (w *WebSocketWorker) runProvider(ctx context.Context, name string, p FeedProvider) {
	updates := make(chan Update, 100)
	for {
		w.logger.Info("websocket subscribing", slog.String("provider", name))
		if err := p.SubscribeLive(ctx, "", updates); err != nil {
			w.logger.Error("websocket error", slog.String("provider", name), slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
```

- [ ] **Step 3: Verify compile**

Run: `go build ./internal/oddsfeed/...`

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/oddsfeed/scheduler.go internal/oddsfeed/websocket.go
git commit -m "feat(oddsfeed): add scheduler and websocket workers"
```

---

## Task 9: Service Entrypoint and Dockerfile

**Files:**
- Create: `cmd/oddsfeed/main.go`
- Create: `Dockerfile.oddsfeed`

- [ ] **Step 1: Write main.go**

```go
package main

import (
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed/providers/mock"
	"github.com/realyoussefhossam/betmonster/internal/shared/config"
	"github.com/realyoussefhossam/betmonster/internal/shared/logging"
)

func main() {
	cfg := config.LoadOddsFeed()
	logger := logging.New()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil { logger.Error("open db", slog.String("error", err.Error())); os.Exit(1) }
	if err := db.Ping(); err != nil { logger.Error("ping db", slog.String("error", err.Error())); os.Exit(1) }
	if err := runMigrations(cfg.DatabaseURL); err != nil { logger.Error("migrations", slog.String("error", err.Error())); os.Exit(1) }

	store := oddsfeed.NewPGStore(db)
	cache := oddsfeed.NewCache(cfg.RedisAddr)
	bus, err := oddsfeed.NewEventBus(cfg.NATSURL, logger)
	if err != nil { logger.Error("nats", slog.String("error", err.Error())); os.Exit(1) }
	defer bus.Close()

	providers := []oddsfeed.FeedProvider{mock.New()}
	svc := oddsfeed.NewService(store, providers, cache, bus, logger)

	grpcServer := grpc.NewServer()
	pb.RegisterOddsFeedServiceServer(grpcServer, oddsfeed.NewGRPCServer(svc))

	go startHealthServer(logger, cfg.Port)

	scheduler := oddsfeed.NewScheduler(svc, []string{"mock"}, time.Duration(cfg.SyncIntervalSeconds)*time.Second, logger)
	go scheduler.Start(context.Background())

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil { logger.Error("listen", slog.String("error", err.Error())); os.Exit(1) }
	logger.Info("oddsfeed gRPC starting", slog.String("addr", cfg.GRPCPort))
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("grpc stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func runMigrations(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil { return err }
	defer db.Close()
	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil { return err }
	m, err := migrate.NewWithDatabaseInstance("file://internal/oddsfeed/migrations", "pgx", driver)
	if err != nil { return err }
	if err := m.Up(); err != nil && err != migrate.ErrNoChange { return err }
	return nil
}

func startHealthServer(logger *slog.Logger, port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"oddsfeed"}`))
	})
	logger.Info("oddsfeed health starting", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("health stopped", slog.String("error", err.Error()))
	}
}
```

Add `import "context"` and remove unused `strings` if not used.

- [ ] **Step 2: Write Dockerfile**

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o oddsfeed ./cmd/oddsfeed

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/oddsfeed .
EXPOSE 8082 50052
CMD ["./oddsfeed"]
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/oddsfeed`

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/oddsfeed/main.go Dockerfile.oddsfeed
git commit -m "feat(oddsfeed): add entrypoint and Dockerfile"
```

---

## Task 10: Docker Compose and Env Integration

**Files:**
- Modify: `docker-compose.yml`
- Modify: `postgres/init/01-init.sql`
- Modify: `.env.example`

- [ ] **Step 1: Add oddsfeed database to Postgres init**

```sql
CREATE DATABASE better_auth;
GRANT ALL PRIVILEGES ON DATABASE better_auth TO wallet;
CREATE DATABASE oddsfeed;
GRANT ALL PRIVILEGES ON DATABASE oddsfeed TO wallet;
```

- [ ] **Step 2: Add oddsfeed service to docker-compose.yml**

```yaml
  oddsfeed:
    build:
      context: .
      dockerfile: Dockerfile.oddsfeed
    image: betmonster/oddsfeed
    ports:
      - "8082:8082"
      - "50052:50052"
    environment:
      PORT: "8082"
      GRPC_PORT: "50052"
      DATABASE_URL: postgres://wallet:wallet@postgres:5432/oddsfeed?sslmode=disable
      REDIS_ADDR: redis:6379
      NATS_URL: nats://nats:4222
      PROVIDERS: ${ODDSFEED_PROVIDERS:-mock}
      AZURO_GRAPH_URL: ${ODDSFEED_AZURO_GRAPH_URL:-}
      AZURO_WS_URL: ${ODDSFEED_AZURO_WS_URL:-}
      SYNC_INTERVAL_SECONDS: ${ODDSFEED_SYNC_INTERVAL_SECONDS:-60}
      WS_RECONNECT_MAX_SECONDS: ${ODDSFEED_WS_RECONNECT_MAX_SECONDS:-300}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
      nats:
        condition: service_started
```

- [ ] **Step 3: Add env vars to .env.example**

```bash
# Odds/Feed
ODDSFEED_PROVIDERS=mock
ODDSFEED_AZURO_GRAPH_URL=
ODDSFEED_AZURO_WS_URL=
ODDSFEED_SYNC_INTERVAL_SECONDS=60
ODDSFEED_WS_RECONNECT_MAX_SECONDS=300
```

- [ ] **Step 4: Build and smoke test in Docker Compose**

Run: `docker compose build oddsfeed && docker compose up -d oddsfeed && curl http://localhost:8082/health`

Expected: `{"status":"healthy","service":"oddsfeed"}`.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml postgres/init/01-init.sql .env.example
git commit -m "feat(oddsfeed): wire docker compose and env"
```

---

## Task 11: Azuro Adapter

**Files:**
- Create: `internal/oddsfeed/providers/azuro/azuro.go`
- Create: `internal/oddsfeed/providers/azuro/azuro_test.go`

- [ ] **Step 1: Implement Azuro adapter skeleton**

```go
package azuro

import (
	"context"
	"fmt"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
)

const ProviderName = "azuro"

type Provider struct {
	graphURL string
	wsURL    string
}

func New(graphURL, wsURL string) *Provider { return &Provider{graphURL: graphURL, wsURL: wsURL} }

func (p *Provider) Name() string { return ProviderName }

func (p *Provider) FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
	if p.graphURL == "" { return nil, fmt.Errorf("azuro graph URL not configured") }
	// TODO: query Azuro Graph API and normalize into oddsfeed.Snapshot.
	return &oddsfeed.Snapshot{Provider: ProviderName}, nil
}

func (p *Provider) SubscribeLive(ctx context.Context, sport string, updates chan<- oddsfeed.Update) error {
	if p.wsURL == "" { return fmt.Errorf("azuro websocket URL not configured") }
	// TODO: connect to Azuro WebSocket and push oddsfeed.Update messages.
	<-ctx.Done()
	return ctx.Err()
}

func (p *Provider) ValidateConfig(cfg oddsfeed.ProviderConfig) error {
	if cfg.GraphURL == "" { return fmt.Errorf("graph URL required") }
	return nil
}
```

- [ ] **Step 2: Add Azuro adapter tests**

Test `ValidateConfig` with missing URL and verify error. Test `FetchSnapshot` returns empty snapshot when not configured.

- [ ] **Step 3: Run tests and commit**

Run: `go test ./internal/oddsfeed/...`

Expected: PASS.

```bash
git add internal/oddsfeed/providers/azuro
git commit -m "feat(oddsfeed): add azuro adapter skeleton"
```

Note: The full Azuro Graph/WebSocket implementation is left for a follow-up task once the exact Azuro API endpoints and response shapes are confirmed.

---

## Self-Review Checklist

1. **Spec coverage:**
   - Provider adapter interface: Task 4
   - Mock provider: Task 4
   - Azuro adapter: Task 11
   - Data model and migrations: Task 2
   - gRPC API: Tasks 1 and 7
   - Ingestion pipeline: Tasks 6 and 8
   - Redis cache: Task 6
   - NATS events: Task 6
   - Docker deployment: Tasks 9 and 10

2. **Placeholder scan:** No TBD/TODO/\"later\" placeholders. The Azuro `FetchSnapshot`/`SubscribeLive` are skeletons with explicit TODOs in code, which is acceptable because the exact Azuro API shape is pending confirmation.

3. **Type consistency:** All `Store` methods, `Service` methods, and `GRPCServer` methods use the same `Sport`, `League`, `Event`, `Market`, `Outcome` types defined in Task 4.

4. **Gaps:** Full Azuro API integration requires confirming the exact GraphQL schema and WebSocket payload. The skeleton is in place so the rest of the service can be built and tested with Mock first.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-07-06-oddsfeed-microservice.md`.**

Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — I execute tasks in this session using `executing-plans`, batch execution with checkpoints for review.

Which approach do you want?
