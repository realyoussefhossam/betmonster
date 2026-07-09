# Plan 004: Production-readiness hardening — full endpoint verification and startup race

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 213469f..HEAD -- docker-compose.yml internal/gateway/server.go internal/gateway/server_test.go internal/gateway/oddsfeed_client.go internal/oddsfeed/server.go internal/oddsfeed/memory_store.go internal/oddsfeed/service.go cmd/gateway/main.go cmd/oddsfeed/main.go`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx | bug
- **Planned at**: commit `213469f`, 2026-07-10
- **Issue**: (none)

## Why this matters

The public sportsbook endpoints are now wired and the immediate data-quality bugs are fixed, but the stack is not yet production-ready. Two concrete issues were observed during the last restart:

1. The gateway container exits on startup if the oddsfeed gRPC service is not ready, because `cmd/gateway/main.go` creates the client with a short hard-coded timeout and `os.Exit(1)` on failure. The docker-compose `depends_on` for the gateway does not include `oddsfeed`, so the race is normal on every cold start.
2. We do not have automated verification that every public endpoint returns a correct, contract-compliant response. The existing gateway tests cover the sportsbook read paths but not the wallet mutation paths, admin paths, or the sportsbook markets/outcomes paths with real data.

This plan makes the stack resilient to startup ordering and adds a comprehensive endpoint contract test that exercises every public route.

## Current state

- `docker-compose.yml` (lines 121–127) has the gateway depending on `app`, `wallet`, and `redis`, but **not** on `oddsfeed`:
  ```yaml
    depends_on:
      app:
        condition: service_started
      wallet:
        condition: service_started
      redis:
        condition: service_started
  ```
- `cmd/gateway/main.go` (lines 31–35) creates the oddsfeed client and exits if it cannot connect:
  ```go
  oddsfeedClient, err := gateway.NewOddsFeedClient(cfg.OddsFeedServiceAddr)
  if err != nil {
      logger.Error("failed to connect oddsfeed service", slog.String("error", err.Error()))
      os.Exit(1)
  }
  ```
- `internal/gateway/oddsfeed_client.go` likely creates the gRPC connection with a short timeout (open and confirm).
- `internal/gateway/server_test.go` currently has sportsbook route tests but does not test wallet mutations, admin endpoints, or markets/outcomes with seeded data.
- `internal/gateway/server.go` exposes these public routes:
  - `GET /api/wallet/supported`
  - `GET /api/wallet/rates`
  - `GET /api/wallet/balance`
  - `GET /api/wallet/transactions`
  - `GET /api/wallet/deposit-address`
  - `POST /api/wallet/withdraw`
  - `GET /api/admin/withdrawals`
  - `POST /api/admin/withdrawals/review`
  - `POST /webhooks/xcash/deposit`
  - Sportsbook routes: `GET /api/sports`, `/api/sports/{sport_id}/leagues`, `/api/events`, `/api/events/{event_id}`, `/api/events/{event_id}/markets`, `/api/markets/{market_id}/outcomes`, `/api/live/events`

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Run gateway tests | `go test -race ./internal/gateway/...` | exit 0 |
| Run all tests | `go test -race ./...` | exit 0 |
| Build gateway | `make build` | exit 0 |
| Docker start (cold) | `docker compose down && docker compose up -d` | all services reach healthy/started |
| Docker compose config | `docker compose config` | exit 0, no errors |

## Scope

**In scope** (files you may modify):
- `docker-compose.yml` — add `oddsfeed` to gateway `depends_on`.
- `internal/gateway/oddsfeed_client.go` — make the initial connection retry or non-blocking.
- `cmd/gateway/main.go` — do not exit if oddsfeed is not immediately reachable; log and retry lazily.
- `internal/gateway/server_test.go` — add comprehensive endpoint contract tests.

**Out of scope** (do NOT touch):
- Wallet/business logic; only test the gateway contract.
- Frontend code.
- Oddsfeed provider logic.
- Plan files under `plans/` (except index update at the end).

## Git workflow

- Branch: `advisor/004-production-readiness-hardening`
- Commit message style: `fix(gateway): ...` / `test(gateway): ...` / `ops(docker): ...` (repo uses conventional commits).
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Make the gateway resilient to oddsfeed startup order

1. Open `internal/gateway/oddsfeed_client.go` and look at `NewOddsFeedClient`. If it dials with a short timeout, change it to use gRPC's default retry/backoff behavior (do not block on a hard timeout). The gRPC connection itself is lazy; the client should be created without an immediate health check.

2. If `NewOddsFeedClient` currently pings the server, remove the ping. The gateway's first request will naturally fail over gRPC's retry mechanism until oddsfeed is ready.

3. In `cmd/gateway/main.go`, replace the fatal error with a warning:
   ```go
   oddsfeedClient, err := gateway.NewOddsFeedClient(cfg.OddsFeedServiceAddr)
   if err != nil {
       logger.Error("failed to connect oddsfeed service; will retry lazily", slog.String("error", err.Error()))
   }
   ```
   If the gateway constructor requires a non-nil client, change the constructor to accept a nil client and return errors on the sportsbook handlers when it is unavailable. But prefer making the client creation itself non-fatal.

**Verify**: `go test -race ./internal/gateway/...` passes.

**Verify**: `make build` passes.

### Step 2: Add oddsfeed to the gateway's docker-compose depends_on

In `docker-compose.yml`, under the `gateway` service `depends_on`, add:
```yaml
      oddsfeed:
        condition: service_started
```

This ensures Docker Compose starts oddsfeed before gateway, even though the gateway now handles the race gracefully.

**Verify**: `docker compose config` exits 0 and shows `oddsfeed` under `gateway.depends_on`.

**STOP condition**: If `docker compose config` reports a cycle or invalid depends_on, stop and report.

### Step 3: Add comprehensive gateway endpoint contract tests

In `internal/gateway/server_test.go`, extend the existing test setup to cover:

1. **Wallet public endpoints**
   - `GET /api/wallet/supported` → 200, returns currencies/chains/pairs.
   - `GET /api/wallet/rates` → 200, returns rates.

2. **Wallet authenticated endpoints** (use the existing JWKS/bufconn pattern to produce a valid Bearer token, or mock the JWKS client)
   - `GET /api/wallet/balance?currency=USDT` → 200 for an authenticated user.
   - `GET /api/wallet/transactions` → 200 for an authenticated user.
   - `GET /api/wallet/deposit-address?currency=USDT&chain=polygon` → 200 or 400 depending on wallet mock setup; the test should verify the contract shape, not the real xcash integration.
   - `POST /api/wallet/withdraw` → verify the request body is forwarded to the wallet gRPC client.

3. **Admin endpoints**
   - `GET /api/admin/withdrawals` → 401 for non-admin user, 200 for admin user.
   - `POST /api/admin/withdrawals/review` → 401 for non-admin, 200 for admin.

4. **Webhook endpoint**
   - `POST /webhooks/xcash/deposit` → verify HMAC validation is attempted (the wallet mock can assert the call). Return 200 for a valid signature or 400 for invalid.

5. **Sportsbook markets/outcomes**
   - `GET /api/events/{event_id}/markets` → 200, returns markets.
   - `GET /api/markets/{market_id}/outcomes` → 200, returns outcomes.
   Use the existing in-memory oddsfeed test helper to seed an event with a market and outcomes.

For tests that require authentication, mock the `JWKSClient` or pass a test token. The simplest approach is to add a constructor overload or a test-only JWKS client that always returns a fixed user ID. If that is too invasive, use the existing `UserFromRequest` function with a test JWKS server on `bufconn` (not preferred; prefer mocking the client).

**STOP condition**: If adding tests requires changing the public production code in a way that affects behavior, stop and report the minimal change needed.

**Verify**: `go test -race ./internal/gateway/... -run 'TestGatewayEndpoints|TestGatewayAdmin|TestGatewayMarkets|TestGatewayOutcomes'` passes.

### Step 4: Run full verification suite

**Verify**: `go test -race ./internal/gateway/...` passes.

**Verify**: `go test -race ./...` passes.

**Verify**: `make build` passes.

**Verify**: `gofmt -d internal/gateway/...` returns empty output.

**Verify**: `git status --short` shows only the in-scope files modified.

## Test plan

- New tests in `internal/gateway/server_test.go` covering all public endpoints.
- Existing tests in `internal/gateway/server_test.go` already cover sportsbook sports/leagues/events/live routes; keep them passing.
- Verification: `go test -race ./internal/gateway/...` passes with the new tests.

## Done criteria

- [ ] Gateway no longer exits on startup if oddsfeed is not ready.
- [ ] `docker-compose.yml` includes `oddsfeed` in the gateway `depends_on`.
- [ ] `internal/gateway/server_test.go` has tests for every public endpoint.
- [ ] `go test -race ./internal/gateway/...` exits 0.
- [ ] `go test -race ./...` exits 0.
- [ ] `make build` exits 0.
- [ ] `gofmt -d internal/gateway/...` returns empty output.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- Removing the initial oddsfeed ping changes the gRPC client behavior in a way that breaks existing tests.
- `docker compose config` fails after the depends_on change.
- Mocking the JWKS client requires a large refactor of the gateway server.
- A test fails twice after a reasonable fix attempt.
- You need to modify files outside the in-scope list.

## Maintenance notes

- After this plan, the stack should survive a `docker compose down && docker compose up -d` without manual restarts.
- The endpoint contract tests will catch regressions when new routes are added or when the response shape changes.
- Future work: add end-to-end Docker Compose tests that actually spin up xcash and exercise a full deposit flow.
