# BetMonster Odds/Feed Microservice Design

**Scope:** second slice of the BetMonster open-source, self-hosted sportsbook/casino platform.

**Date:** 2026-07-06  
**Status:** design

## 1. Context

BetMonster is an open-source, self-hosted sportsbook/casino platform. v1 established the wallet microservice (gateway + wallet) for crypto deposits and withdrawals. v2 starts the sportsbook stack with the **Odds/Feed service**, which ingests external sports data, normalizes it, and exposes it to the future Sportsbook service.

The first external provider is **Azuro** (`https://gem.azuro.org/hub`), a free, decentralized betting data provider. The service is designed so operators can later plug in Sportradar, Genius Sports, or any other feed provider.

## 2. Goals

- Ingest fixtures, odds, markets, and live scores from external providers.
- Normalize provider-specific data into a common internal data model.
- Cache live odds and scores for fast reads by the Sportsbook service.
- Publish change events via NATS so other services can react in real time.
- Provide a pluggable provider adapter interface so operators can swap or add feeds.

## 3. Non-Goals

- No bet placement, settlement, or wallet operations in this service.
- No odds compilation or risk management in this service.
- No user-facing frontend logic in this service.
- No real-money transaction handling in this service.
- No KYC/AML or compliance logic in this service.

## 4. Architecture

```
┌─────────────┐      ┌──────────────┐      ┌──────────────┐
│   Azuro     │◀────▶│  Odds/Feed   │◀────▶│  Sportsbook  │
│   APIs      │      │   Service    │ gRPC  │   Service    │
└─────────────┘      │              │       │   (future)   │
                     │   ┌──────┐   │       └──────────────┘
                     ├──▶│Postgres│◀──┤              │
                     │   │Feeds   │   │              │
                     │   │Store   │   │              │
                     │   └──────┘   │              │
                     │   ┌──────┐   │              ▼
                     ├──▶│ Redis│◀──┤         ┌──────────────┐
                     │   │ Live │   │         │   Next.js    │
                     │   │Cache │   │         │   (future)   │
                     │   └──────┘   │         └──────────────┘
                     │       │      │
                     └───────┼──────┘
                             ▼
                     ┌──────────────┐
                     │    NATS      │
                     │  feed.*      │
                     └──────────────┘
```

### Components

- **Odds/Feed service** (`cmd/oddsfeed`): owns the feeds database, provider adapters, ingestion pipeline, and gRPC API.
- **Azuro adapter** (`internal/oddsfeed/providers/azuro`): consumes Azuro Graph API and WebSocket API.
- **Feeds Store** (Postgres): stores normalized sports, leagues, events, markets, outcomes, and live scores.
- **Redis live cache**: caches the current live snapshot for fast reads.
- **NATS**: emits events like `feed.event.updated`, `feed.odds.changed`, `feed.score.changed`.
- **Sportsbook service** (future): consumes Odds/Feed gRPC API and NATS events for bet placement and settlement.

## 5. Provider Adapter Model

The core abstraction is the `FeedProvider` interface. Every external feed implements this interface. The Odds/Feed service does not know provider-specific details beyond the adapter.

```go
type FeedProvider interface {
    Name() string
    // FetchSnapshot returns a full normalized snapshot for a given sport.
    FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*Snapshot, error)
    // FetchHierarchy returns sports, leagues, and events without markets/outcomes.
    // Used for incremental syncs where only changed events need fresh conditions.
    FetchHierarchy(ctx context.Context, sport string, params map[string]string) (*Snapshot, error)
    // FetchConditions returns markets and outcomes for a specific set of game IDs.
    FetchConditions(ctx context.Context, gameIDs []string) (*Snapshot, error)
    // SubscribeLive opens a live update channel (websocket, SSE, or long-polling).
    // Returns when the context is cancelled or on fatal error.
    SubscribeLive(ctx context.Context, sport string, updates chan<- Update) error
    // ValidateConfig checks provider-specific configuration.
    ValidateConfig(cfg ProviderConfig) error
}
```

### v2 Adapters

- **Azuro** (primary): Azuro public **Backend REST API** for snapshots, WebSocket API for live updates.
- **Mock** (for local tests): returns deterministic fixtures and odds without external calls.

#### Why not the Graph API for feed data?

Azuro’s Graph API is reserved for **bet history and transaction-related data**. The protocol explicitly moved all feed data (games, conditions, current odds, outcomes) to the Backend REST API. Querying the Graph API for feed data is deprecated and no longer returns current odds/conditions. Odds/Feed therefore uses:

- `GET /market-manager/sports` — sport/country/league/game hierarchy
- `POST /market-manager/conditions-by-game-ids` — markets and outcomes for a set of game IDs
- `wss://streams.onchainfeed.org/v1/streams/conditions` — live condition/outcome updates

#### Supported Azuro Environments

The adapter supports every environment exposed by Azuro’s REST API. Set `ODDSFEED_AZURO_ENVIRONMENT` to one of the following values:

| Environment | Network | Notes |
|---|---|---|
| `PolygonUSDT` | Polygon mainnet | Default, tested |
| `PolygonAmoyUSDT` | Polygon Amoy testnet | Dev |
| `PolygonAmoyAZUSD` | Polygon Amoy testnet | Dev, AZUSD token |
| `GnosisXDAI` | Gnosis mainnet | Deprecated by Azuro |
| `GnosisDevXDAI` | Gnosis devnet | Deprecated by Azuro |
| `BaseWETH` | Base mainnet | |
| `BaseSepoliaWETH` | Base Sepolia testnet | Dev |
| `ChilizWCHZ` | Chiliz mainnet | Deprecated by Azuro |
| `ChilizSpicyWCHZ` | Chiliz Spicy testnet | Dev, deprecated by Azuro |
| `BscUSDT` | BNB Chain mainnet | Deprecated by Azuro |
| `BscDevUSDT` | BNB Chain devnet | Dev, deprecated by Azuro |

### Future Adapters

- Sportradar
- Genius Sports
- The Odds API
- BetConstruct
- Polymarket

## 6. Internal Data Model

```sql
sports
  id uuid primary key
  provider text not null
  provider_sport_id text not null
  slug text not null
  name text not null
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
  unique(provider, provider_sport_id)

leagues
  id uuid primary key
  provider text not null
  provider_league_id text not null
  sport_id uuid not null references sports(id)
  name text not null
  country text
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
  unique(provider, provider_league_id)

events
  id uuid primary key
  provider text not null
  provider_event_id text not null
  league_id uuid not null references leagues(id)
  sport_id uuid not null references sports(id)
  home_participant text not null
  away_participant text not null
  starts_at timestamptz not null
  status text not null check (status in ('upcoming', 'live', 'paused', 'finished', 'cancelled', 'postponed'))
  home_score text
  away_score text
  score_updated_at timestamptz
  metadata jsonb
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
  unique(provider, provider_event_id)

markets
  id uuid primary key
  provider text not null
  provider_market_id text not null
  event_id uuid not null references events(id)
  type text not null
  name text not null
  line text
  status text not null check (status in ('active', 'suspended', 'settled', 'cancelled'))
  metadata jsonb
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
  unique(provider, provider_market_id)

outcomes
  id uuid primary key
  provider text not null
  provider_outcome_id text not null
  market_id uuid not null references markets(id)
  name text not null
  odds text not null
  status text not null check (status in ('active', 'suspended', 'won', 'lost', 'cancelled'))
  metadata jsonb
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
  unique(provider, provider_outcome_id)

odds_snapshots
  id uuid primary key
  event_id uuid not null references events(id)
  market_id uuid not null references markets(id)
  snapshot_at timestamptz not null default now()
  odds jsonb not null
  created_at timestamptz not null default now()
```

### Notes

- `odds` is stored as a string to preserve exact decimal precision, matching the wallet service pattern.
- `provider_*_id` fields are used for idempotent upserts and provider reconciliation.
- `metadata` holds provider-specific fields without polluting the schema.
- `odds_snapshots` is optional in v2; it enables historical odds analysis and audit later.

## 7. Ingestion Pipeline

### 7.1 Snapshot Sync (Polling)

1. A scheduler triggers periodic syncs (configurable, default 60 seconds).
2. The adapter first calls `FetchHierarchy` to get the latest sports, leagues, and events.
3. The service compares each event's status against the stored status. Conditions are only refetched for:
   - New events not yet in the database
   - Events whose status changed (e.g. `upcoming` → `live`, `live` → `finished`)
   - Events currently marked as `live`
4. The adapter calls `FetchConditions` only for the changed game IDs.
5. The normalizer maps provider-specific structures to internal entities.
6. The store performs idempotent upserts using `provider_*_id`.
7. The cache writes a live snapshot to Redis.
8. The event bus emits change events if data differs from the previous state.

This avoids fetching full market data for every stable upcoming event on every tick, which is the main cost of the Azuro REST API.

### 7.2 Live Updates (WebSocket)

1. A background worker opens a WebSocket connection per enabled provider/sport.
2. For Azuro, the worker subscribes to all currently live condition IDs on `wss://streams.onchainfeed.org/v1/streams/conditions`.
3. Incremental updates are pushed through `SubscribeLive`.
4. Each update is normalized and applied through the same store + cache + emit pipeline.
5. On disconnect, the worker reconnects with exponential backoff (1s, 2s, 4s, ...) up to `ODDSFEED_WS_RECONNECT_MAX_SECONDS`.
6. If the WebSocket is unhealthy for longer than a threshold, the service falls back to polling.

> **Operational note:** as of the latest test, the Azuro `streams.onchainfeed.org` endpoint returns HTTP 502 during the WebSocket handshake. The implementation is correct per Azuro's documented URL and message format; the 502 appears to be a provider-side issue. The worker keeps reconnecting with backoff and the incremental snapshot poller continues to refresh live events.

### 7.3 Change Events

NATS subjects:

- `feed.sport.updated`
- `feed.league.updated`
- `feed.event.created`
- `feed.event.updated`
- `feed.market.updated`
- `feed.odds.changed`
- `feed.score.changed`
- `feed.status.changed`

Event payloads contain the internal entity ID and minimal metadata. Consumers fetch full details from the gRPC API or cache if needed.

## 8. gRPC API

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

message Sport {
  string id = 1;
  string name = 2;
  string slug = 3;
}

message League {
  string id = 1;
  string name = 2;
  string sport_id = 3;
  string country = 4;
}

message Event {
  string id = 1;
  string league_id = 2;
  string sport_id = 3;
  string home_participant = 4;
  string away_participant = 5;
  string starts_at = 6;
  string status = 7;
  string home_score = 8;
  string away_score = 9;
  string score_updated_at = 10;
}

message Market {
  string id = 1;
  string event_id = 2;
  string type = 3;
  string name = 4;
  string line = 5;
  string status = 6;
}

message Outcome {
  string id = 1;
  string market_id = 2;
  string name = 3;
  string odds = 4;
  string status = 5;
}

message ListSportsRequest { int32 page = 1; int32 page_size = 2; }
message ListSportsResponse { repeated Sport sports = 1; }

message ListLeaguesRequest { string sport_id = 1; int32 page = 2; int32 page_size = 3; }
message ListLeaguesResponse { repeated League leagues = 1; }

message ListEventsRequest {
  string sport_id = 1;
  string league_id = 2;
  string status = 3;
  int32 page = 4;
  int32 page_size = 5;
}
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

### Notes

- Pagination follows the existing wallet service pattern.
- `status` filters support the normalized status values.
- Streaming RPCs for live push can be added in v3 when moving to WebSocket-first.

### Service Endpoints

| Endpoint | Port | Purpose | Access |
|---|---|---|---|
| gRPC `OddsFeedService` | `50052` | Internal API consumed by gateway/sportsbook | Internal only |
| HTTP `/health` | `8082` | Health/readiness check | Internal only |

There is no public REST API for oddsfeed; the gateway will expose sportsbook data through its own routes and forward internal requests via gRPC.

## 9. Redis Cache Model

Redis keys:

- `oddsfeed:live:events:{sport_id}` — sorted set of live event IDs by score updated time.
- `oddsfeed:live:odds:{market_id}` — hash of current outcomes and odds.
- `oddsfeed:live:score:{event_id}` — hash of home_score, away_score, status, time.
- `oddsfeed:sync:state:{provider}` — last successful sync timestamp and cursor.

TTLs:

- Live odds: 60 seconds, refreshed on every update.
- Live scores: 60 seconds, refreshed on every update.
- Sync state: no TTL, updated after each sync.

## 10. Configuration

Environment variables (the service reads these without the `ODDSFEED_` prefix inside the container; Docker Compose maps `ODDSFEED_*` to the bare names):

| Variable | Description | Example |
|---|---|---|
| `ODDSFEED_PROVIDERS` | Comma-separated enabled providers | `azuro` |
| `ODDSFEED_AZURO_GRAPH_URL` | Azuro Backend API base URL | `https://api.onchainfeed.org/api/v1/public` |
| `ODDSFEED_AZURO_WS_URL` | Azuro WebSocket URL | `wss://streams.onchainfeed.org/v1/streams/conditions` |
| `ODDSFEED_AZURO_ENVIRONMENT` | Azuro environment (see table in §5) | `PolygonUSDT` |
| `ODDSFEED_SYNC_INTERVAL_SECONDS` | Polling interval | `60` |
| `ODDSFEED_WS_RECONNECT_MAX_SECONDS` | Max reconnect backoff | `300` |

Internal-only variables set by `docker-compose.yml`:

| Variable | Description | Example |
|---|---|---|
| `PORT` | HTTP health port | `8082` |
| `GRPC_PORT` | gRPC port | `50052` |
| `DATABASE_URL` | Postgres connection | `postgres://wallet:wallet@postgres:5432/oddsfeed?sslmode=disable` |
| `REDIS_ADDR` | Redis address | `redis:6379` |
| `NATS_URL` | NATS URL | `nats://nats:4222` |

## 11. Error Handling & Reliability

- **Provider failures**: one provider failing does not block others. Errors are logged and metrics are emitted.
- **WebSocket disconnects**: automatic reconnect with exponential backoff (1s, 2s, 4s, ... up to max).
- **Polling fallback**: if WebSocket is disconnected longer than 2 sync intervals, the snapshot poller covers the gap.
- **Idempotent writes**: all upserts use `provider_*_id` as the stable key.
- **Timeouts**: every external call has a context timeout.
- **Circuit breaker**: if a provider fails repeatedly, the adapter is temporarily disabled and retried later.

## 12. Security

- The Odds/Feed gRPC service is internal only; it is not exposed publicly.
- The gateway will route read-only sports/odds queries to the Odds/Feed service when the Sportsbook frontend needs them.
- Provider API keys/tokens are configured via environment variables and never logged.
- No user authentication inside the Odds/Feed service; authorization is handled by the gateway.

## 13. Testing

- Unit tests for the normalizer and adapter interface.
- Contract tests for the gRPC API using a mock provider.
- WebSocket integration tests with a local mock server.
- Docker Compose integration test: start Odds/Feed + mock provider, verify data ingestion and NATS events.

## 14. Deployment

The Odds/Feed service is deployed alongside the existing stack:

- New binary: `cmd/oddsfeed/main.go`
- New Dockerfile: `Dockerfile.oddsfeed`
- New package: `internal/oddsfeed/...`
- New migrations: `internal/oddsfeed/migrations/`
- New protobuf: `internal/proto/oddsfeed.proto`
- New docker-compose service:
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
      DATABASE_URL: postgres://wallet:wallet@postgres:5432/oddsfeed?sslmode=disable
      REDIS_ADDR: redis:6379
      NATS_URL: nats://nats:4222
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
      nats:
        condition: service_started
  ```

The Postgres init script should create the `oddsfeed` database if it does not exist.

## 15. Migration Notes

- The wallet service database remains unchanged.
- The Odds/Feed service uses its own logical database (`oddsfeed`).
- Migrations use `golang-migrate` following the same pattern as the wallet service.

## 16. Future Work

- Streaming gRPC RPCs for live odds push.
- WebSocket-first ingestion with polling as pure fallback.
- Odds snapshots and historical odds analysis.
- Additional provider adapters: Sportradar, Genius, Polymarket, The Odds API.
- Odds compilation and internal line management.
- Risk-aware odds suspension when liability thresholds are reached.
