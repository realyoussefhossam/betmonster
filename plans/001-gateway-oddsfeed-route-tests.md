# Plan 001: Add gateway sportsbook route tests

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat bd5ced8..HEAD -- internal/gateway/server_test.go internal/gateway/oddsfeed_client.go internal/gateway/server.go internal/oddsfeed/server.go internal/oddsfeed/memory_store.go internal/oddsfeed/service.go`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `bd5ced8`, 2026-07-10
- **Issue**: (none)

## Why this matters

The gateway exposes seven public sportsbook REST routes that forward to the Odds/Feed gRPC service. These routes are currently implemented but have no automated tests. Without tests, a change to the gRPC mapping, JSON serialization, or path parsing in `internal/gateway/server.go` can break the public sportsbook API silently. This plan adds isolated HTTP-level tests for each route using a local gRPC server, following the same pattern already used for the wallet routes in this file.

## Current state

- `internal/gateway/server.go` (lines 181–188) registers the sportsbook routes:
  ```go
  mux.Handle("/api/sports", server.WithRoutePattern("/api/sports", http.HandlerFunc(s.handleListSports)))
  mux.Handle("/api/sports/{sport_id}/leagues", server.WithRoutePattern("/api/sports/{sport_id}/leagues", http.HandlerFunc(s.handleListLeagues)))
  mux.Handle("/api/events", server.WithRoutePattern("/api/events", http.HandlerFunc(s.handleListEvents)))
  mux.Handle("/api/events/{event_id}", server.WithRoutePattern("/api/events/{event_id}", http.HandlerFunc(s.handleGetEvent)))
  mux.Handle("/api/events/{event_id}/markets", server.WithRoutePattern("/api/events/{event_id}/markets", http.HandlerFunc(s.handleListMarkets)))
  mux.Handle("/api/markets/{market_id}/outcomes", server.WithRoutePattern("/api/markets/{market_id}/outcomes", http.HandlerFunc(s.handleListOutcomes)))
  mux.Handle("/api/live/events", server.WithRoutePattern("/api/live/events", http.HandlerFunc(s.handleListLiveEvents)))
  ```

- `internal/gateway/server_test.go` already tests wallet routes with a local wallet gRPC server behind a `bufconn` listener (e.g., `TestHandleRatesPublicEndpoint` and `TestGatewayForwardsCallerIdentityToWallet`). These are the structural pattern to follow.

- `internal/oddsfeed/server.go` implements the gRPC handlers, and `internal/oddsfeed/memory_store.go` provides an in-memory store that can be seeded without Postgres.

- `internal/oddsfeed/server_test.go` already shows how to seed a service with the mock provider and serve it over `bufconn`.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Run gateway tests | `go test -race ./internal/gateway/...` | exit 0, all tests pass |
| Run all tests | `go test -race ./...` | exit 0, no new failures |
| Build gateway | `go build -o bin/gateway ./cmd/gateway` | exit 0 |
| Format check | `gofmt -d internal/gateway/server_test.go` | empty output (no formatting diffs) |

## Scope

**In scope** (the only files you should modify):
- `internal/gateway/server_test.go` — add new test functions.

**Out of scope** (do NOT touch, even if they look related):
- `internal/gateway/server.go` — routes are already implemented; only test them.
- `internal/oddsfeed/server.go` — gRPC handlers are already implemented; only consume them.
- `internal/oddsfeed/memory_store.go` — use as-is for seeding.
- Any frontend or Next.js code.
- The wallet tests already in the file (do not refactor them).

## Git workflow

- Branch: `advisor/001-gateway-oddsfeed-route-tests`
- Commit message style: `test(gateway): add sportsbook route tests` (repo uses conventional commits; recent examples include `feat(oddsfeed): ...` and `fix(oddsfeed): ...`).
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add a test helper that creates a seeded Odds/Feed gRPC server

Add a helper function inside `internal/gateway/server_test.go` (in package `gateway`) that:

1. Creates an `oddsfeed.NewInMemoryStore()`.
2. Wraps it in `oddsfeed.NewService(store, nil, nil, nil, nil)` (no providers, cache, bus, or logger needed for read-only tests).
3. Manually inserts one sport, one league, one event, one market, and one outcome into the store via the service's public methods or the store's `Upsert*` methods. Use the internal entity types from `internal/oddsfeed`.
4. Starts a `grpc.NewServer()` over a `bufconn.Listen(1024 * 1024)` listener, registers `oddsfeed.NewGRPCServer(svc)`, and returns the listener.

Example seed data:
- Sport: `ID = "sport-1"`, `Name = "Soccer"`, `Slug = "soccer"`, `Provider = "mock"`, `ProviderSportID = "sp-1"`.
- League: `ID = "league-1"`, `Name = "Mock League"`, `SportID = "sport-1"`, `Provider = "mock"`, `ProviderLeagueID = "lg-1"`, `Country = "Mockland"`.
- Event: `ID = "event-1"`, `LeagueID = "league-1"`, `SportID = "sport-1"`, `HomeParticipant = "Mock FC"`, `AwayParticipant = "Test United"`, `Status = "upcoming"`, `StartsAt = time.Now().Add(2 * time.Hour)`, `Provider = "mock"`, `ProviderEventID = "ev-1"`.
- Market: `ID = "market-1"`, `EventID = "event-1"`, `Type = "1x2"`, `Name = "Match Result"`, `Status = "active"`, `Provider = "mock"`, `ProviderMarketID = "mk-1"`.
- Outcome: `ID = "outcome-1"`, `MarketID = "market-1"`, `Name = "Home"`, `Odds = "2.10"`, `Status = "active"`, `Provider = "mock"`, `ProviderOutcomeID = "oc-1"`.

**Verify**: `go test -race ./internal/gateway/... -run TestNoSuchTest` → exits 0 (no tests yet, but the file compiles). If the helper fails to compile, fix it before proceeding.

### Step 2: Add tests for the seven sportsbook routes

For each route below, add a dedicated `Test*` function in `internal/gateway/server_test.go`. Each test should:

1. Create a logger with `slog.New(slog.NewJSONHandler(os.Stdout, nil))`.
2. Call the helper from Step 1 to get a `bufconn` listener.
3. Dial the listener with `grpc.DialContext` using `grpc.WithContextDialer` and `grpc.WithTransportCredentials(insecure.NewCredentials())`.
4. Create an `&OddsFeedClient{conn: pb.NewOddsFeedServiceClient(conn)}`.
5. Create a `NewServer` with that client (other dependencies can be nil or zero values as in existing tests; use `NewRateLimiter("memory", "", 100, 100)` and empty strings for supported currencies/chains/pairs).
6. Build an `httptest.NewRequest` with the correct method and path, and call `srv.Router().ServeHTTP(w, req)`.
7. Assert the response status is 200 and the JSON body contains the expected seeded data.

Routes to test:

| Test name | Path | Expected body content |
|-----------|------|----------------------|
| `TestGatewayListSports` | `GET /api/sports` | contains `"Soccer"` and `"sport-1"` |
| `TestGatewayListLeagues` | `GET /api/sports/sport-1/leagues` | contains `"Mock League"` and `"league-1"` |
| `TestGatewayListEvents` | `GET /api/events?sport_id=sport-1&league_id=league-1` | contains `"Mock FC"`, `"Test United"`, `"event-1"` |
| `TestGatewayGetEvent` | `GET /api/events/event-1` | contains `"Mock FC"`, `"event-1"` |
| `TestGatewayListMarkets` | `GET /api/events/event-1/markets` | contains `"Match Result"`, `"market-1"` |
| `TestGatewayListOutcomes` | `GET /api/markets/market-1/outcomes` | contains `"Home"`, `"2.10"`, `"outcome-1"` |
| `TestGatewayListLiveEvents` | `GET /api/live/events` | 200 OK with empty events list (the seeded event is `upcoming`, not `live`) |
| `TestGatewayListLiveEventsWithLiveEvent` | seed an additional event with `Status = "live"`, then `GET /api/live/events` | contains the live event |

Use the existing `TestHandleRatesPublicEndpoint` as the pattern for setting up `bufconn`, `WalletClient`, and `NewServer`.

**Verify**: `go test -race ./internal/gateway/... -run TestGatewayListSports` → PASS, and similarly for each new test.

### Step 3: Add edge-case tests

Add two more tests:

1. `TestGatewayGetEventNotFound`: request `GET /api/events/nonexistent-id`; assert status 404 and body contains `"not found"`.
2. `TestGatewayListEventsPagination`: seed two events, request `GET /api/events?page=1&page_size=1`, assert exactly one event is returned and a `page_size` of 1 is respected (check length of `events` array).

**Verify**: `go test -race ./internal/gateway/... -run TestGatewayGetEventNotFound` → PASS; `go test -race ./internal/gateway/... -run TestGatewayListEventsPagination` → PASS.

### Step 4: Run the full test suite and build

**Verify**: `go test -race ./internal/gateway/...` → all existing and new tests pass.

**Verify**: `go test -race ./...` → all packages pass (no regressions).

**Verify**: `go build -o bin/gateway ./cmd/gateway` → exits 0.

**Verify**: `gofmt -d internal/gateway/server_test.go` → empty output.

## Test plan

- New tests live in `internal/gateway/server_test.go`.
- They model the existing `TestHandleRatesPublicEndpoint` pattern for local gRPC server setup.
- Coverage:
  - Happy path for all 7 sportsbook routes.
  - 404 response for unknown event ID.
  - Pagination behavior.
  - Live score filtering (`status = live`).
- Verification: `go test -race ./internal/gateway/...` passes with the new tests.

## Done criteria

- [ ] `internal/gateway/server_test.go` contains tests for all 7 sportsbook routes.
- [ ] `go test -race ./internal/gateway/...` exits 0.
- [ ] `go test -race ./...` exits 0.
- [ ] `go build -o bin/gateway ./cmd/gateway` exits 0.
- [ ] `gofmt -d internal/gateway/server_test.go` returns empty output.
- [ ] No files outside `internal/gateway/server_test.go` are modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The drift check shows that `internal/gateway/server.go`, `internal/gateway/oddsfeed_client.go`, or `internal/oddsfeed/server.go` changed in a way that breaks the excerpts above.
- The `bufconn` gRPC dial pattern from the existing wallet tests no longer works in this repo.
- `internal/oddsfeed/memory_store.go` is missing or its interface changed so it cannot be seeded directly.
- A test fails twice after a reasonable fix attempt.
- You discover you need to modify files outside the in-scope list to make the tests pass.

## Maintenance notes

- These tests are HTTP-level contract tests for the gateway's public sportsbook API. If the REST response shape changes, these tests will fail and need updating — which is the intended behavior.
- If path parameters change (e.g., `sport_id` renamed), update the test paths.
- If the Odds/Feed service later requires authentication, these tests will need to add the same metadata/identity setup used by the wallet gateway tests.
