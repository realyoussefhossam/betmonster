# Plan 003: Fix sportsbook endpoint data quality and live event status

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 8f0deb0..HEAD -- internal/oddsfeed/providers/azuro/azuro.go internal/oddsfeed/service.go internal/oddsfeed/websocket.go internal/oddsfeed/store.go internal/oddsfeed/memory_store.go internal/oddsfeed/pgstore.go internal/oddsfeed/server.go internal/gateway/server.go internal/oddsfeed/server_test.go internal/oddsfeed/providers/azuro/azuro_test.go internal/gateway/server_test.go`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: none
- **Category**: bug
- **Planned at**: commit `8f0deb0`, 2026-07-10
- **Issue**: (none)

## Why this matters

The sportsbook endpoints are technically wired, but two data-quality bugs make them unreliable:

1. The Azuro provider fetches only `numberOfGames=10` per league and sorts by ascending turnover, so high-turnover main events (e.g., Conor McGregor vs Max Holloway) are dropped from the sync. The user already observed a UFC 329 card missing its headliner.
2. The `/api/live/events` endpoint always returns `[]` because events never transition from `upcoming` to `live`. The WebSocket receives condition updates from Azuro, but `ApplyUpdate` only updates market/outcome status, not event status. The scheduler also only syncs `gameState=Prematch`, so live events are never refreshed.

Fixing these makes the public sportsbook API actually useful.

## Current state

- `internal/oddsfeed/providers/azuro/azuro.go:255` builds the hierarchy URL with:
  ```go
  reqURL := fmt.Sprintf("%s/market-manager/sports?environment=%s&gameState=%s&numberOfGames=10&orderBy=turnover&orderDirection=asc",
  ```
- `internal/oddsfeed/service.go:88-116` `ApplyUpdate` handles `odds` and `status` updates but only updates market/outcome records; it never updates event status.
- `internal/oddsfeed/service.go:31-72` `SyncProvider` calls `FetchHierarchy(ctx, "", nil)`, which defaults to `gameState="Prematch"`, so live events are never synced.
- `internal/oddsfeed/websocket.go:33-69` forwards provider updates to `service.ApplyUpdate` but does not track event status transitions.
- `internal/oddsfeed/store.go` and `internal/oddsfeed/memory_store.go` define `Store` and `UpdateMarketStatus`/`UpdateOutcomeStatus` but lack `UpdateEventStatus`.
- `internal/oddsfeed/pgstore.go` implements `UpdateMarketStatus` and `UpdateOutcomeStatus` but not event status updates.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Run oddsfeed tests | `go test -race ./internal/oddsfeed/...` | exit 0 |
| Run gateway tests | `go test -race ./internal/gateway/...` | exit 0 |
| Run all tests | `go test -race ./...` | exit 0 |
| Build all services | `make build` | exit 0, all binaries created |
| Format check | `gofmt -d internal/oddsfeed/...` | empty output |

## Scope

**In scope** (files you may modify):
- `internal/oddsfeed/providers/azuro/azuro.go` — fix hierarchy URL.
- `internal/oddsfeed/service.go` — add event status update logic and live sync.
- `internal/oddsfeed/store.go` — add `UpdateEventStatus` to the `Store` interface.
- `internal/oddsfeed/memory_store.go` — implement `UpdateEventStatus`.
- `internal/oddsfeed/pgstore.go` — implement `UpdateEventStatus`.
- `internal/oddsfeed/server_test.go` — add/update tests for live event status.
- `internal/oddsfeed/providers/azuro/azuro_test.go` — add/update tests for the URL change.

**Out of scope** (do NOT touch):
- `internal/gateway/server.go` — endpoints are already correct; only verify them.
- `internal/gateway/server_test.go` — do not change existing tests (they already pass).
- `README.md`, `CHECKLIST.md`, `AGENTS.md`, `Makefile` — already updated in Plan 002.
- Frontend code under `app/`.

## Git workflow

- Branch: `advisor/003-fix-sportsbook-endpoints-and-live-events`
- Commit message style: `fix(oddsfeed): ...` or `feat(oddsfeed): ...` (repo uses conventional commits).
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Fix the Azuro hierarchy URL to fetch more games and order by main events

In `internal/oddsfeed/providers/azuro/azuro.go`, change line 255 from:
```go
reqURL := fmt.Sprintf("%s/market-manager/sports?environment=%s&gameState=%s&numberOfGames=10&orderBy=turnover&orderDirection=asc",
```
to:
```go
reqURL := fmt.Sprintf("%s/market-manager/sports?environment=%s&gameState=%s&numberOfGames=100&orderBy=turnover&orderDirection=desc",
```

Rationale: `100` is large enough to capture a full UFC card per league; `desc` puts the highest-turnover (main) events first, so they are never truncated. `asc` currently puts low-turnover prelims first and drops headliners.

If Azuro supports omitting `numberOfGames`, you may instead remove the parameter entirely and rely on default server behavior. But `numberOfGames=100` is a safe, explicit, bounded change.

**Verify**: `grep "numberOfGames=100" internal/oddsfeed/providers/azuro/azuro.go` → matches. `grep "orderDirection=asc" internal/oddsfeed/providers/azuro/azuro.go` → no matches.

**Verify**: `go test -race ./internal/oddsfeed/providers/azuro/...` → passes.

### Step 2: Add `UpdateEventStatus` to the Store interface and implementations

1. In `internal/oddsfeed/store.go`, add to the `Store` interface:
   ```go
   UpdateEventStatus(ctx context.Context, provider, providerEventID, status string) error
   ```

2. In `internal/oddsfeed/memory_store.go`, implement it:
   ```go
   func (s *memoryStore) UpdateEventStatus(ctx context.Context, provider, providerEventID, status string) error {
       s.mu.Lock()
       defer s.mu.Unlock()
       key := eventKey(provider, providerEventID)
       e, ok := s.events[key]
       if !ok {
           return fmt.Errorf("update event status: not found")
       }
       e.Status = status
       e.UpdatedAt = now()
       return nil
   }
   ```

3. In `internal/oddsfeed/pgstore.go`, implement it with a SQL update:
   ```go
   func (s *PGStore) UpdateEventStatus(ctx context.Context, provider, providerEventID, status string) error {
       _, err := s.db.ExecContext(ctx, `
           UPDATE events
           SET status = $1, updated_at = now()
           WHERE provider = $2 AND provider_event_id = $3
       `, status, provider, providerEventID)
       return err
   }
   ```

**Verify**: `go test -race ./internal/oddsfeed/...` → compiles and passes (the new interface method is implemented).

### Step 3: Update `ApplyUpdate` to transition event status when live conditions arrive

In `internal/oddsfeed/service.go`, modify `ApplyUpdate` for the `status` case so that when a condition becomes `active` (i.e., the market is live), the parent event is also marked `live` if it was `upcoming`.

The cleanest way is to add a helper that resolves the event from the market. Since `UpdateMarketStatus` returns the market's `event_id` (update the existing return if necessary), you can use that to mark the event live.

Current `UpdateMarketStatus` in `store.go` returns `(string, error)` where the string is the market ID. You may need to change it to return `(eventID, marketID string, err error)` or add a separate `GetMarketEventID` method. Pick the minimal change that does not break existing callers.

Implementation approach (minimal change):
- Add a new method `GetMarketEventID(ctx context.Context, marketID string) (string, error)` to `Store` and its implementations.
- In `ApplyUpdate`, when a `status` update sets a market to `active`, call `GetMarketEventID`, then `UpdateEventStatus(ctx, provider, providerEventID, "live")`.
- You need to map the internal market ID back to the provider event ID. You can either:
  - Store `provider_event_id` in the market metadata or
  - Add a method `GetMarketProviderInfo(ctx, marketID) (provider, providerEventID string, err error)`.

If this mapping is too invasive, a simpler alternative is to add a periodic live sync (Step 4) and keep Step 3 for a future plan. For this plan, **make Step 3 optional**; the live sync in Step 4 is sufficient to make `/api/live/events` return events.

**STOP condition**: If resolving market → event requires changing the database schema, stop and report instead of improvising.

### Step 4: Add live event sync to the scheduler

In `internal/oddsfeed/scheduler.go`, add a second sync pass that calls `SyncLiveProvider` (or extend `SyncProvider` with a `gameState` parameter).

Simplest approach: add a new method on `Service`:
```go
func (s *Service) SyncLiveProvider(ctx context.Context, providerName string) error {
    p, ok := s.providers[providerName]
    if !ok {
        return fmt.Errorf("unknown provider: %s", providerName)
    }
    // Fetch live games
    snap, err := p.FetchSnapshot(ctx, "", map[string]string{"game_state": "Live"})
    if err != nil {
        return fmt.Errorf("fetch live snapshot: %w", err)
    }
    return s.applySnapshot(ctx, snap)
}
```

Then in `scheduler.go`, run both the existing `SyncProvider` and the new `SyncLiveProvider` each tick.

This will update the status of live events to `"live"` so `/api/live/events` returns them.

**Verify**: `go test -race ./internal/oddsfeed/...` → passes.

### Step 5: Add tests

1. In `internal/oddsfeed/providers/azuro/azuro_test.go` (or create one), add a test that verifies the hierarchy URL contains `numberOfGames=100` and `orderDirection=desc`.

2. In `internal/oddsfeed/server_test.go`, add a test that seeds an event with `status = "live"`, calls `ListLiveScores`, and asserts the live event is returned.

3. In `internal/oddsfeed/server_test.go`, add a test that seeds more than 10 events in a single league and verifies `ListEvents` returns all of them (to catch the Azuro truncation bug in a unit-test-friendly way using the mock provider, since the real Azuro provider is hard to test without the API).

**Verify**: `go test -race ./internal/oddsfeed/... -run TestAzuroHierarchyURL` → passes. `go test -race ./internal/oddsfeed/... -run TestGRPCServerListLiveScores` → passes.

### Step 6: Run full verification

**Verify**: `go test -race ./internal/oddsfeed/...` → passes.

**Verify**: `go test -race ./internal/gateway/...` → passes.

**Verify**: `go test -race ./...` → passes.

**Verify**: `make build` → exits 0, all binaries created.

**Verify**: `gofmt -d internal/oddsfeed/...` → empty output.

**Verify**: `git status --short` → only in-scope files modified.

## Test plan

- New tests:
  - `TestAzuroHierarchyURL` — asserts the Azuro sports URL uses `numberOfGames=100` and `orderDirection=desc`.
  - `TestGRPCServerListLiveScores` — seeds a live event and asserts `ListLiveScores` returns it.
  - `TestListEventsPaginationOverTen` — seeds 15 events in one league and asserts all are returned (page_size large enough).
- Existing tests in `internal/gateway/server_test.go` serve as the endpoint contract verification.
- Verification: `go test -race ./...` passes.

## Done criteria

- [ ] Azuro hierarchy URL uses `numberOfGames=100` and `orderDirection=desc`.
- [ ] `Store` interface and both implementations have `UpdateEventStatus`.
- [ ] Scheduler syncs live events so `/api/live/events` can return non-empty results.
- [ ] New tests for URL change, live event listing, and pagination over 10 events pass.
- [ ] `go test -race ./...` exits 0.
- [ ] `make build` exits 0.
- [ ] `gofmt -d internal/oddsfeed/...` returns empty output.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The Azuro API rejects `numberOfGames=100` or `orderDirection=desc` during manual testing.
- Adding `UpdateEventStatus` or `GetMarketEventID` requires changing the database schema.
- `SyncLiveProvider` cannot be implemented without modifying the `FeedProvider` interface.
- A test fails twice after a reasonable fix attempt.
- You need to modify files outside the in-scope list.

## Maintenance notes

- The `numberOfGames=100` value is a heuristic. If Azuro's actual league cards are larger, increase it or implement pagination.
- The live sync interval should probably be shorter than the prematch sync interval. If the scheduler gains a configurable interval per sync type, revisit this.
- When the WebSocket is fixed to also update event status directly, the scheduler live sync can be relaxed or removed.
