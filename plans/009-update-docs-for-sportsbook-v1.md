# Plan 009: Update project docs to reflect sportsbook v1

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat d4161b7..HEAD -- README.md AGENTS.md CHECKLIST.md docs/superpowers/specs/`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none (docs can be updated independently; should be done after Plan 007 and Plan 008 so the described APIs are stable)
- **Category**: docs
- **Planned at**: commit `d4161b7`, 2026-07-11
- **Issue**: (none)

## Why this matters

`README.md`, `AGENTS.md`, and `CHECKLIST.md` currently describe sportsbook and betting as future/v2 work. The codebase now has a working sportsbook gRPC service (`internal/sportsbook`), REST betting endpoints in the gateway (`POST /api/bets`, etc.), and a new `sportsbook` Docker Compose service. Stale docs mislead operators, frontend developers, and future agents about what the system can do today.

## Current state

- `README.md:182-193` lists sportsbook endpoints but only read-only ones (`/api/sports`, `/api/events`, etc.); it omits the betting endpoints.
- `README.md:53-65` architecture section lists gateway, wallet, and oddsfeed but does not mention the sportsbook service.
- `README.md:84-91` quick-start service list omits sportsbook.
- `AGENTS.md:12-21` and `AGENTS.md:40-47` still say "Sportsbook" is a v2/future service.
- `CHECKLIST.md:109-116` "Future Slices" still lists "Sportsbook engine" as not built.
- `CHECKLIST.md:75-89` microservices roadmap still marks sportsbook as v2.
- The new sportsbook service is exposed on `localhost:8083` (health) and `localhost:50053` (gRPC).
- Gateway betting endpoints (verified in `internal/gateway/server.go:191-193`):
  - `POST /api/bets` — place a bet (auth required)
  - `GET /api/bets` — list my bets (auth required)
  - `GET /api/bets/{bet_id}` — get a bet (auth required)
  - `POST /api/admin/bets/settle` — admin settle bet (admin required)

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Markdown links | `grep -E '\[.*\]\(.*\.md\)' README.md AGENTS.md CHECKLIST.md` | no broken relative links |
| Spelling sanity | `grep -i 'sportsbook' README.md AGENTS.md CHECKLIST.md` | all mentions consistent with v1 status |
| Build | `make build` | exit 0 (no code changes) |
| Tests | `go test ./...` | exit 0 (no code changes) |

## Scope

**In scope** (files you may modify):
- `README.md`
- `AGENTS.md`
- `CHECKLIST.md`

**Out of scope** (do NOT touch):
- Any source code.
- `docs/superpowers/specs/` existing specs (only update cross-references if needed).
- `plans/README.md` (the executor updates its own row when done, but do not rewrite the plan index content).

## Git workflow

- Branch: `advisor/009-update-docs-for-sportsbook-v1`
- Commit message style: `docs: ...` or `docs(sportsbook): ...`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Update `README.md`

1. Add the Sportsbook service to the architecture section (`README.md:53-65`), e.g.:
   > - **Go Sportsbook**: betting engine for single moneyline bets; locks stakes via the wallet service, records odds snapshots, and settles outcomes.

2. Add `Sportsbook` to the quick-start service list (`README.md:84-91`):
   > - Sportsbook gRPC on localhost:50053 and health on http://localhost:8083

3. Expand the Sportsbook Endpoints table (`README.md:182-193`) to include betting endpoints:
   | Gateway Endpoint | Description |
   |------------------|-------------|
   | `GET /api/sports` | List sports |
   | ... | ... |
   | `POST /api/bets` | Place a single moneyline bet (auth required) |
   | `GET /api/bets` | List authenticated user's bets (auth required) |
   | `GET /api/bets/{bet_id}` | Get a single bet (auth required) |
   | `POST /api/admin/bets/settle` | Manually settle a bet as won/lost/cancelled (admin required) |

4. Update the Makefile example (`README.md:127`) to mention `sportsbook` build output:
   > ```bash
   > # Build all binaries (gateway, wallet, oddsfeed, sportsbook)
   > make build
   > ```

**Verify**: `grep -A1 'Sportsbook' README.md | head -20` shows consistent service description and endpoints.

### Step 2: Update `AGENTS.md`

1. In `AGENTS.md:12-21` (Current Architecture), add:
   > - **Go Sportsbook**: v1 single moneyline betting engine. Calls wallet for stake debit/credit and oddsfeed for event/outcome validation.

2. In `AGENTS.md:34-49` (Microservices Roadmap), change the Sportsbook row to indicate v1 is in progress/delivered:
   > | **Sportsbook** | Events, odds, single moneyline bet placement, settlement | v1 |

3. Add a short "Sportsbook v1 notes" subsection under "Real Money Warning" or "Production-Ready Notes" summarizing:
   - Stake is debited before the bet is accepted.
   - Odds are locked at bet placement time.
   - Settlement credits winnings via the wallet ledger.
   - Bet records are immutable; status transitions are audited.

**Verify**: `grep -i 'sportsbook' AGENTS.md` shows v1 references, not only v2/future references.

### Step 3: Update `CHECKLIST.md`

1. Under "v2 Odds/Feed Microservice" (`CHECKLIST.md:91-101`), add a new top-level section:

   ```markdown
   ## v1 Sportsbook Microservice

   ### Sportsbook Service
   - [x] Add `cmd/sportsbook` entrypoint.
   - [x] Add Sportsbook Postgres schema and migrations.
   - [x] Add gRPC contract (`PlaceBet`, `GetBet`, `ListBets`, `SettleBet`) and generated code.
   - [x] Implement in-memory and Postgres store.
   - [x] Implement service logic for single moneyline bet placement and settlement.
   - [x] Add gRPC server and scheduler for auto-settlement.
   - [x] Integrate with Wallet `DebitWallet`/`CreditWallet` RPCs.
   - [x] Integrate with Odds/Feed for event/market/outcome validation.
   - [x] Add public sportsbook REST routes in the gateway.
   - [x] Add gateway → sportsbook gRPC client.

   ### Future Sportsbook Work
   - [ ] Parlays, systems, and live cash-out.
   - [ ] Bet limits, risk checks, and self-exclusion hooks.
   - [ ] Live betting beyond status polling.
   ```

2. In the "Future Slices" section (`CHECKLIST.md:107-116`), remove or reword "Sportsbook engine" since v1 single bets are implemented. Keep the remaining future items (casino, automated withdrawals, non-EVM assets, etc.).

3. Update the microservices roadmap table (`CHECKLIST.md:75-89`) so Sportsbook is v1.

**Verify**: `grep -A15 'v1 Sportsbook' CHECKLIST.md` exists and contains the checkboxes above.

### Step 4: Consistency check

**Verify**: All three files use the same terminology:
- "single moneyline" for the v1 bet type
- "settlement" for the payout flow
- "odds snapshot" for locked odds

Run:

```bash
grep -n 'Sportsbook' README.md AGENTS.md CHECKLIST.md
```

Expect at least three mentions, all consistent with v1 delivery.

### Step 5: Final verification

**Verify**: `make build` → exits 0.

**Verify**: `go test ./...` → exits 0.

**Verify**: `git diff --stat` → only `README.md`, `AGENTS.md`, `CHECKLIST.md` modified.

**Verify**: `git status --short` → only in-scope files modified.

## Test plan

- No code tests needed; this is a docs-only plan.
- Manual review: read each modified doc section to confirm it accurately describes the current system.
- Optional: open `README.md` rendered preview and confirm the sportsbook endpoint table renders correctly.

## Done criteria

- [ ] `README.md` mentions the sportsbook service in architecture and quick-start sections.
- [ ] `README.md` sportsbook endpoint table includes `POST /api/bets`, `GET /api/bets`, `GET /api/bets/{bet_id}`, and `POST /api/admin/bets/settle`.
- [ ] `AGENTS.md` describes sportsbook as v1 delivered, not future-only.
- [ ] `CHECKLIST.md` has a completed "v1 Sportsbook Microservice" section.
- [ ] `CHECKLIST.md` no longer lists sportsbook engine as a future-only item.
- [ ] All three docs use consistent sportsbook terminology.
- [ ] `make build` and `go test ./...` still pass.
- [ ] Only doc files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The gateway routes described in Step 1 do not exist in the code (verify against `internal/gateway/server.go:191-193`).
- A doc change would require touching source code to remain accurate.
- You find the docs already updated beyond the current state excerpts.

## Maintenance notes

- Keep the sportsbook endpoint table in `README.md` in sync with future route changes.
- As new bet types (parlays, live) are added, update `AGENTS.md` and `CHECKLIST.md` rather than letting them drift again.
- Consider generating an `API.md` reference from the gRPC `.proto` files and gateway route definitions in a future plan.
