# Plan 005: Sportsbook v1 — single bet placement and settlement

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 2ccfa26..HEAD -- internal/oddsfeed/ internal/wallet/ internal/gateway/ internal/proto/ docker-compose.yml`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: L
- **Risk**: HIGH (handles real money)
- **Depends on**: none
- **Category**: direction / feature
- **Planned at**: commit `2ccfa26`, 2026-07-10
- **Issue**: (none)

## Why this matters

The Odds/Feed microservice now ingests sports events and odds, but users cannot place bets yet. The Sportsbook service is the missing betting engine. This v1 slice delivers the simplest useful betting flow: a single moneyline bet on an event, with wallet stake locking, settlement when the event finishes, and payout via the wallet service. Without this, the sportsbook UI is read-only.

## Current state

- `internal/oddsfeed/` exposes sports, leagues, events, markets, and outcomes via gRPC and the gateway exposes them as public REST routes.
- `internal/wallet/` handles balances, deposits, withdrawals, and ledger transactions via gRPC.
- `internal/gateway/` routes public HTTP requests to the wallet and oddsfeed gRPC services.
- There is no Sportsbook service, database, or API yet.
- The platform roadmap (`docs/superpowers/specs/2026-07-04-betmonster-roadmap.md`) lists Sportsbook as a v2 service: "Events, odds, markets, bet types (single, multi, system), live betting hooks".

## Design decisions

- **Service boundary**: Sportsbook owns bet records, odds snapshots at bet time, and settlement logic. It does NOT own wallet balances — it calls the Wallet gRPC service to debit stakes and credit winnings.
- **Money safety**: every bet is a wallet transaction. Stake is debited before the bet is accepted. Settlement credits winnings atomically via the wallet ledger.
- **Odds locking**: when a bet is placed, the Sportsbook records the exact odds at that moment (an odds snapshot per bet) so payout is deterministic even if live odds change later.
- **Bet types v1**: only **single moneyline** bets on an outcome. Parlays, systems, and live cash-out are out of scope.
- **Settlement trigger**: v1 uses a simple scheduler that polls events in `live`/`finished` status from Odds/Feed and settles bets when an outcome status becomes `won`/`lost`. Manual admin settlement is included as a fallback.
- **Idempotency**: bet placement is idempotent by a client-generated `reference_id` to prevent double-bets on retries.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Run all Go tests | `go test -race ./...` | exit 0 |
| Build all services | `make build` | exit 0, all binaries created |
| Run sportsbook tests | `go test -race ./internal/sportsbook/...` | exit 0 |
| Docker compose config | `docker compose config` | exit 0 |
| Regenerate protobuf | `make proto` | exit 0 |

## Scope

**In scope** (files you may create or modify):
- `internal/proto/sportsbook.proto` — gRPC contract.
- `internal/proto/sportsbook*.pb.go` — generated code (via `make proto`).
- `internal/sportsbook/` — new service package: models, store interface, pgstore, service, gRPC server, scheduler, migrations.
- `internal/sportsbook/migrations/*.sql` — database schema.
- `internal/shared/config/sportsbook.go` — config loader.
- `cmd/sportsbook/main.go` — entrypoint.
- `Dockerfile.sportsbook` — Docker build.
- `docker-compose.yml` — add sportsbook service.
- `internal/gateway/server.go` — add REST routes for sportsbook betting.
- `internal/gateway/sportsbook_client.go` — gRPC client for sportsbook.
- `internal/gateway/server_test.go` — add sportsbook route tests.
- `postgres/init/01-init.sql` — create sportsbook database.
- `.env.example` — add sportsbook env vars.
- `Makefile` — add sportsbook build target.

**Out of scope** (do NOT touch):
- Wallet business logic (only call it via gRPC).
- Oddsfeed provider logic (only query it via gRPC).
- Frontend code (UI comes later).
- Parlays, systems, live cash-out, live betting beyond status polling.
- Casino games.
- Risk/Admin services.

## Git workflow

- Branch: `advisor/005-sportsbook-v1-single-bets`
- Commit message style: `feat(sportsbook): ...` (repo uses conventional commits).
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Design and document the gRPC contract

Create `internal/proto/sportsbook.proto`:

```protobuf
syntax = "proto3";
package sportsbook;
option go_package = "github.com/realyoussefhossam/betmonster/internal/proto";

service SportsbookService {
  rpc PlaceBet(PlaceBetRequest) returns (PlaceBetResponse);
  rpc GetBet(GetBetRequest) returns (GetBetResponse);
  rpc ListBets(ListBetsRequest) returns (ListBetsResponse);
  rpc SettleBet(SettleBetRequest) returns (SettleBetResponse);
}

message OddsSnapshot {
  string outcome_id = 1;
  string odds = 2;  // decimal string, e.g. "2.10"
}

message Bet {
  string id = 1;
  string user_id = 2;
  string event_id = 3;
  string market_id = 4;
  string outcome_id = 5;
  string odds = 6;
  string stake = 7;  // decimal string
  string potential_payout = 8;
  string currency = 9;
  string status = 10;  // pending, won, lost, cancelled, settled
  string reference_id = 11;
  string created_at = 12;
  string settled_at = 13;
}

message PlaceBetRequest {
  string user_id = 1;
  string event_id = 2;
  string market_id = 3;
  string outcome_id = 4;
  string stake = 5;
  string currency = 6;
  string reference_id = 7;
}

message PlaceBetResponse { Bet bet = 1; }

message GetBetRequest { string id = 1; }
message GetBetResponse { Bet bet = 1; }

message ListBetsRequest {
  string user_id = 1;
  string status = 2;
  int32 page = 3;
  int32 page_size = 4;
}
message ListBetsResponse { repeated Bet bets = 1; }

message SettleBetRequest {
  string bet_id = 1;
  string outcome = 2;  // won, lost, cancelled
}
message SettleBetResponse { Bet bet = 1; }
```

Run `make proto` to generate Go code.

**Verify**: `ls internal/proto/sportsbook*.pb.go` → files exist.

**Verify**: `go build ./internal/proto/...` → compiles.

### Step 2: Add database migrations

Create `internal/sportsbook/migrations/20260710120000_create_sportsbook_schema.up.sql`:

```sql
CREATE TABLE bets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id text NOT NULL,
  event_id uuid NOT NULL,
  market_id uuid NOT NULL,
  outcome_id uuid NOT NULL,
  odds text NOT NULL,
  stake text NOT NULL,
  potential_payout text NOT NULL,
  currency text NOT NULL,
  status text NOT NULL CHECK (status IN ('pending', 'won', 'lost', 'cancelled', 'settled')),
  reference_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  settled_at timestamptz,
  UNIQUE(user_id, reference_id)
);

CREATE INDEX idx_bets_user_id ON bets(user_id);
CREATE INDEX idx_bets_status ON bets(status);
CREATE INDEX idx_bets_event_id ON bets(event_id);
CREATE INDEX idx_bets_outcome_id ON bets(outcome_id);
CREATE INDEX idx_bets_reference_id ON bets(reference_id);
```

Create the corresponding `down.sql`.

Add the sportsbook database in `postgres/init/01-init.sql`:

```sql
CREATE DATABASE sportsbook;
```

**Verify**: `go run github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1 -path internal/sportsbook/migrations -database 'postgres://wallet:wallet@localhost:5433/sportsbook?sslmode=disable' up` → success or no change.

### Step 3: Implement the Sportsbook service

Create `internal/sportsbook/`:

- `models.go` — `Bet` and `OddsSnapshot` types.
- `store.go` — `Store` interface:
  ```go
  type Store interface {
      CreateBet(ctx context.Context, b Bet) (string, error)
      GetBet(ctx context.Context, id string) (*Bet, error)
      ListBets(ctx context.Context, userID, status string, page, pageSize int) ([]Bet, error)
      UpdateBetStatus(ctx context.Context, id, status string, settledAt time.Time) error
      GetBetByReference(ctx context.Context, userID, referenceID string) (*Bet, error)
  }
  ```
- `pgstore.go` — Postgres implementation.
- `memory_store.go` — In-memory implementation for tests.
- `service.go` — `Service` with:
  - `PlaceBet(ctx, userID, eventID, marketID, outcomeID, stake, currency, referenceID) (Bet, error)`
  - `GetBet(ctx, id) (*Bet, error)`
  - `ListBets(ctx, userID, status, page, pageSize) ([]Bet, error)`
  - `SettleBet(ctx, betID, outcome) (Bet, error)`
  - `AutoSettleFromEvents(ctx) error` (settles pending bets when outcome status is known)
- `server.go` — gRPC server implementing `SportsbookService`.
- `scheduler.go` — periodic auto-settlement worker.
- `decimal.go` — helper using `shopspring/decimal` for odds/payout math (match wallet conventions).

**PlaceBet logic:**
1. Validate event/market/outcome exist via Odds/Feed gRPC.
2. Record current odds from the outcome.
3. Calculate potential payout: `stake * odds`.
4. Call Wallet gRPC `DebitWallet` to lock the stake. Use the bet `reference_id` as idempotency key.
5. Insert bet record with `status = pending`.

**SettleBet logic:**
1. Load bet. If not `pending`, return error.
2. If outcome is `won`: call Wallet `CreditWallet` with `potential_payout`. Update bet status to `settled` and record `won` outcome.
3. If outcome is `lost`: update bet status to `settled` and record `lost` outcome (no wallet credit).
4. If outcome is `cancelled`: call Wallet `CreditWallet` with stake (refund). Update bet status to `cancelled`.

**AutoSettleFromEvents logic:**
1. Query Odds/Feed for finished events with outcomes.
2. For each pending bet on those outcomes, call `SettleBet` with the outcome status.

**Verify**: `go test -race ./internal/sportsbook/...` → passes (tests in Step 7).

### Step 4: Add the Sportsbook entrypoint

Create `cmd/sportsbook/main.go` similar to `cmd/oddsfeed/main.go`:
- Load config.
- Connect to Postgres and run migrations.
- Create store, oddsfeed client, wallet client.
- Create service.
- Start gRPC server on `SPORTSBOOK_GRPC_PORT` (default 50053).
- Start health server on `SPORTSBOOK_PORT` (default 8083).
- Start scheduler.

**Verify**: `go build -o bin/sportsbook ./cmd/sportsbook` → exits 0.

### Step 5: Add config and Docker support

Create `internal/shared/config/sportsbook.go` with env vars:
- `PORT`
- `GRPC_PORT`
- `DATABASE_URL`
- `ODDSFEED_SERVICE_ADDR`
- `WALLET_SERVICE_ADDR`
- `SETTLE_INTERVAL_SECONDS`

Create `Dockerfile.sportsbook` similar to `Dockerfile.oddsfeed`.

Update `docker-compose.yml` to add the sportsbook service with depends_on on `postgres`, `wallet`, and `oddsfeed`.

Update `Makefile` `build` target to include `bin/sportsbook`.

Update `.env.example` with sportsbook env vars.

**Verify**: `make build` → creates `bin/sportsbook` and exits 0.

**Verify**: `docker compose config` → exits 0 and includes sportsbook service.

### Step 6: Wire sportsbook into the gateway

1. Create `internal/gateway/sportsbook_client.go` with `NewSportsbookClient(addr)` and `PlaceBet`, `GetBet`, `ListBets`, `SettleBet` methods.
2. In `cmd/gateway/main.go`, create the sportsbook client (lazy, non-blocking like oddsfeed).
3. In `internal/gateway/server.go`, pass the sportsbook client to `NewServer` and add routes:
   - `POST /api/bets` — place a bet (auth required).
   - `GET /api/bets` — list my bets (auth required).
   - `GET /api/bets/{bet_id}` — get a bet (auth required).
   - `POST /api/admin/bets/settle` — admin settle bet (admin required).

4. Implement handlers that forward to the sportsbook gRPC client.

**Verify**: `go test -race ./internal/gateway/...` → passes.

### Step 7: Write tests

Add tests in `internal/sportsbook/`:
- `service_test.go` — test place bet, idempotency, settle win, settle loss, settle cancel, auto-settle.
- `pgstore_test.go` — test store operations against a real Postgres test DB (follow wallet pgstore test pattern).
- `server_test.go` — gRPC server contract tests with in-memory store and mock wallet/oddsfeed clients.

Add tests in `internal/gateway/server_test.go`:
- `TestGatewayPlaceBet` — place bet returns 201 and bet data.
- `TestGatewayListBets` — list bets returns user's bets.

For the mock wallet/oddsfeed clients, create test helpers that implement the gRPC client interfaces and record calls.

**Verify**: `go test -race ./internal/sportsbook/...` → passes.

**Verify**: `go test -race ./internal/gateway/...` → passes.

### Step 8: Run full verification

**Verify**: `go test -race ./...` → passes.

**Verify**: `make build` → exits 0.

**Verify**: `gofmt -d internal/sportsbook/... internal/gateway/sportsbook_client.go cmd/sportsbook/main.go` → empty output.

**Verify**: `docker compose config` → exits 0.

## Test plan

- Unit tests in `internal/sportsbook/service_test.go` for bet math, idempotency, and settlement.
- Store tests in `internal/sportsbook/pgstore_test.go` against Postgres.
- gRPC server tests in `internal/sportsbook/server_test.go` with mock wallet/oddsfeed.
- Gateway route tests in `internal/gateway/server_test.go` for place/list bets.
- Verification: `go test -race ./...` passes.

## Done criteria

- [ ] `internal/proto/sportsbook.proto` and generated `.pb.go` files exist.
- [ ] `internal/sportsbook/migrations/` create the bets table.
- [ ] `internal/sportsbook/` implements `Service`, `Store`, gRPC server, and scheduler.
- [ ] `cmd/sportsbook/main.go` builds and starts the service.
- [ ] `Dockerfile.sportsbook` and `docker-compose.yml` sportsbook service added.
- [ ] Gateway exposes `POST /api/bets`, `GET /api/bets`, `GET /api/bets/{id}`, `POST /api/admin/bets/settle`.
- [ ] `make build` creates `bin/sportsbook`.
- [ ] `go test -race ./...` passes.
- [ ] `docker compose config` exits 0.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The drift check shows the Odds/Feed or Wallet gRPC interfaces changed in a way that breaks the integration points in this plan.
- Adding the sportsbook database requires changes to the existing wallet or oddsfeed databases.
- The Wallet service does not expose `DebitWallet`/`CreditWallet` gRPC methods (verify before implementing).
- Settlement logic requires knowing the event outcome, and the Odds/Feed outcome status update is not reliable enough.
- A test fails twice after a reasonable fix attempt.
- You need to modify files outside the in-scope list.

## Maintenance notes

- This is the minimal sportsbook slice. Future work: parlays, live cash-out, bet limits, risk checks, automated settlement oracles, and integration with the Casino service.
- The settlement scheduler is polling-based. A future optimization is to subscribe to the Odds/Feed NATS events and settle on event finish notifications.
- All bet payout math uses decimal strings to avoid floating-point errors. Future bet types (spread, totals) will need additional fields in the bet record.
