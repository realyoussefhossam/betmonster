# Plan 002: Update all missed docs and config for the current project state

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat ea050d1..HEAD -- README.md CHECKLIST.md Makefile AGENTS.md`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live files before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: docs
- **Planned at**: commit `ea050d1`, 2026-07-10
- **Issue**: (none)

## Why this matters

The repo has evolved past v1: the Odds/Feed microservice is implemented, gateway caller identity is forwarded to the wallet, and PGStore integration tests exist. The public-facing documentation and build config have not kept up. An outdated README, CHECKLIST, and Makefile mislead contributors and operators about what is implemented and how to run it. This plan brings all of them into sync with the current code.

## Current state

- `README.md` still opens with "v1 focus is a wallet microservice" and does not mention the Odds/Feed service or sportsbook endpoints.
- `CHECKLIST.md` is titled "v1 Wallet Microservice Checklist" and marks several already-completed items as unchecked (e.g., "Forward user context to wallet service via gRPC metadata" is now done; "Unit tests: concurrent wallet credit/debit" is now done by PGStore integration tests).
- `Makefile` `build` target only builds `gateway` and `wallet`; it does not build `oddsfeed`.
- `AGENTS.md` may need a note that the Odds/Feed service is now implemented and that gateway → wallet metadata forwarding is in place.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build all services | `make build` | exits 0, `bin/gateway`, `bin/wallet`, `bin/oddsfeed` created |
| Run tests | `make test` | exits 0 |
| Run integration tests | `make integration-test` | exits 0 (requires Postgres) |
| Format Go files | `gofmt -d Makefile` | N/A — skip |

## Scope

**In scope** (the only files you should modify):
- `README.md`
- `CHECKLIST.md`
- `Makefile`
- `AGENTS.md`

**Out of scope** (do NOT touch):
- Source code or tests.
- `.env.example` and `docker-compose.yml` — already contain oddsfeed config.
- Frontend files under `app/`.
- Plan files under `plans/` (except the index update at the end).

## Git workflow

- Branch: `advisor/002-update-missed-docs-and-config`
- Commit message style: `docs: update README, CHECKLIST, Makefile, and AGENTS for oddsfeed and recent changes`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Update README.md for the Odds/Feed service and sportsbook endpoints

In `README.md`:

1. Change the opening paragraph to mention the Odds/Feed microservice:
   ```markdown
   BetMonster is an open-source, self-hosted sportsbook/casino platform. The v1 focus is a **wallet microservice** that supports USDT/USDC deposits via [xcash](https://github.com/xca-sh/xcash) and manual admin withdrawals. v2 adds the **Odds/Feed microservice** to ingest, normalize, and serve sports fixtures, odds, markets, and live scores via a public sportsbook REST API.
   ```

2. In the Architecture section, add:
   ```markdown
   - **Go Odds/Feed**: ingests external sports data (Azuro as the first provider), normalizes it into an internal model, and exposes sports/odds via gRPC to the gateway and future sportsbook service.
   ```
   Update the Postgres line to:
   ```markdown
   - **Postgres**: Better Auth (Next.js/Prisma), wallet service, and oddsfeed service databases.
   ```
   Update the internal communication line to:
   ```markdown
   Internal gateway → wallet and gateway → oddsfeed communication uses **gRPC**.
   ```

3. In the Quick Start service list, add:
   ```markdown
   - Odds/Feed gRPC on localhost:50052 and health on http://localhost:8082
   ```

4. In the Local Development section, after the gateway step, add:
   ```markdown
   5. Start the oddsfeed service:
      ```bash
      go run ./cmd/oddsfeed
      ```
   ```
   Adjust the Next.js step to `6.`.

5. In the Useful Commands section, add:
   ```markdown
   # Build all binaries (including oddsfeed)
   make build && go build -o bin/oddsfeed ./cmd/oddsfeed
   ```
   (Or, if you update the Makefile in Step 4, change it to just `make build`.)

6. After the Wallet Endpoints table, add a Sportsbook Endpoints table:
   ```markdown
   ## Sportsbook Endpoints

   | Gateway Endpoint | Description |
   |------------------|-------------|
   | `GET /api/sports` | List sports |
   | `GET /api/sports/{sport_id}/leagues` | List leagues for a sport |
   | `GET /api/events` | List events (filter by `sport_id`, `league_id`, `status`) |
   | `GET /api/events/{event_id}` | Get a single event |
   | `GET /api/events/{event_id}/markets` | List markets for an event |
   | `GET /api/markets/{market_id}/outcomes` | List outcomes for a market |
   | `GET /api/live/events` | List currently live events with scores |
   ```

7. In the Docs section, add:
   ```markdown
   - [Odds/Feed microservice design](docs/superpowers/specs/2026-07-06-oddsfeed-microservice-design.md)
   ```

**Verify**: `grep -q "Odds/Feed" README.md && grep -q "/api/sports" README.md && grep -q "oddsfeed" README.md` → all match.

### Step 2: Update CHECKLIST.md to reflect the current state

In `CHECKLIST.md`:

1. Change the title from:
   ```markdown
   # BetMonster — v1 Wallet Microservice Checklist
   ```
   to:
   ```markdown
   # BetMonster — v1 Wallet + v2 Odds/Feed Checklist
   ```

2. Under Gateway Service, mark as done:
   ```markdown
   - [x] Forward user context to wallet service via gRPC metadata.
   ```

3. Under Testing, mark as done:
   ```markdown
   - [x] Unit tests: concurrent wallet credit/debit (via PGStore integration tests).
   ```

4. Under the Microservices Roadmap, update the Odds/Feed row from v3 to v2 if it is still v3, or leave it if already v2. (Check the live file first.)

5. Add a new top-level section for v2 Odds/Feed with completed items:
   ```markdown
   ## v2 Odds/Feed Microservice

   ### Odds/Feed Service
   - [x] Add `cmd/oddsfeed` entrypoint.
   - [x] Add Odds/Feed Postgres schema and migrations.
   - [x] Add gRPC contract and generated code.
   - [x] Implement Azuro provider adapter and mock provider.
   - [x] Implement normalized sports/leagues/events/markets/outcomes store.
   - [x] Add polling scheduler and WebSocket live worker.
   - [x] Add Redis live cache and NATS event bus.
   - [x] Add public sportsbook REST routes in the gateway.
   - [x] Add gateway → oddsfeed gRPC client.
   ```

**Verify**: `grep -q "v2 Odds/Feed" CHECKLIST.md && grep -q "Forward user context" CHECKLIST.md` → both match.

### Step 3: Update Makefile to build the oddsfeed binary

In `Makefile`:

1. Add `oddsfeed` to the `build` target:
   ```makefile
   build:
       mkdir -p bin
       go build -o bin/gateway ./cmd/gateway
       go build -o bin/wallet ./cmd/wallet
       go build -o bin/oddsfeed ./cmd/oddsfeed
   ```

2. Add `oddsfeed.proto` to the `proto` target if it is not already there (verify live file first).

**Verify**: `make build` → creates `bin/oddsfeed` and exits 0.

### Step 4: Update AGENTS.md with current context

In `AGENTS.md`:

1. In the Current Architecture section, add the Odds/Feed service to the bullet list and update the Postgres line to include oddsfeed.

2. Add a short note under the Security or Production-Ready Notes section:
   ```markdown
   - Gateway forwards authenticated caller identity to the wallet gRPC service via metadata. The wallet service validates the caller user ID and requires admin metadata for admin-only RPCs.
   ```

**Verify**: `grep -q "Odds/Feed" AGENTS.md && grep -q "caller identity" AGENTS.md` → both match.

### Step 5: Run the full verification suite

**Verify**: `make build` → exits 0, `bin/gateway`, `bin/wallet`, and `bin/oddsfeed` all exist.

**Verify**: `make test` → exits 0.

**Verify**: `git status --short` → only the four in-scope files are modified.

**Verify**: `git diff --stat` → shows changes only in `README.md`, `CHECKLIST.md`, `Makefile`, `AGENTS.md`.

## Test plan

- No automated tests for markdown; verification is by grepping for expected sections and by running the build/test commands.

## Done criteria

- [ ] `README.md` mentions the Odds/Feed microservice and sportsbook endpoints.
- [ ] `CHECKLIST.md` reflects the v2 Odds/Feed work and marks completed v1 items.
- [ ] `Makefile` `build` target produces `bin/oddsfeed`.
- [ ] `AGENTS.md` includes the Odds/Feed service and caller-identity forwarding note.
- [ ] `make build` exits 0.
- [ ] `make test` exits 0.
- [ ] `git status --short` shows only the four in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The drift check shows any in-scope file changed significantly and the excerpts no longer match.
- `make build` or `make test` fails after the changes.
- You discover you need to modify source code or other files to make the documentation accurate.

## Maintenance notes

- This is a documentation/config catch-up. Future feature work should update these files incrementally rather than letting them drift again.
- When the Sportsbook service (bet placement) lands, README and CHECKLIST will need another pass.
