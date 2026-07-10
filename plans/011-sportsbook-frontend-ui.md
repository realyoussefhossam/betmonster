# Plan 011: Add sportsbook betting UI and API client methods

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 0a91ec3..HEAD -- app/`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: Plan 010 (sportsbook backend endpoints stable and reviewed)
- **Category**: feature
- **Planned at**: commit `0a91ec3`, 2026-07-11
- **Issue**: (none)

## Why this matters

The backend now supports single moneyline sportsbook bets via the gateway (`POST /api/bets`, `GET /api/bets`, `GET /api/bets/{id}`, `POST /api/admin/bets/settle`), but the Next.js frontend only exposes wallet and admin withdrawals. End users cannot browse events or place bets, and admins cannot settle bets from the UI. This plan wires up the missing frontend surface.

## Current state

- `app/lib/go-api-client.ts` exposes only wallet methods. It uses `authClient.token()` for JWT retrieval and `fetch` with `Authorization: Bearer`.
- `app/app/page.tsx` links to `/wallet` and `/admin/withdrawals` only.
- `app/app/wallet/page.tsx` is the exemplar for data-loading client pages: `useEffect`, loading/error states, Tailwind + shadcn UI.
- `app/app/admin/withdrawals/page.tsx` is the exemplar for admin client pages: no client-side role check, relies on the API to return 403 for non-admins.
- Available UI components: `Button`, `Card`, `Input`, `Label`, `Skeleton`, `Sonner` toaster.
- `app/package.json` scripts: `dev`, `build`, `lint` (`eslint`).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Type check | `cd app && npx tsc --noEmit` | exit 0 |
| Lint | `cd app && npm run lint` | exit 0 |
| Build | `cd app && npm run build` | exit 0 |
| Full stack build | `make build` (from repo root) | exit 0 |

## Scope

**In scope** (files you may modify):
- `app/lib/go-api-client.ts` — add sportsbook types and methods.
- `app/app/page.tsx` — add a link to the sportsbook page.
- `app/app/sportsbook/page.tsx` — new page: list events, select market/outcome, place bet.
- `app/app/sportsbook/bets/page.tsx` — new page: list authenticated user's bets.
- `app/app/admin/settlements/page.tsx` — new page: list pending bets and settle them.

**Out of scope** (do NOT touch):
- Backend Go code or `internal/`.
- Wallet UI pages.
- Better Auth schema/configuration.
- Real-time odds polling beyond a simple interval.

## Git workflow

- Branch: `advisor/011-sportsbook-frontend-ui`
- Commit message style: `feat(frontend): ...` or `feat(app): ...`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add sportsbook types and methods to the API client

In `app/lib/go-api-client.ts`, add the following interfaces and methods, following the existing `ApiResponse<T>` pattern.

**Types**:

```ts
export interface Sport {
  id: string;
  name: string;
}

export interface League {
  id: string;
  name: string;
}

export interface Event {
  id: string;
  sportId: string;
  leagueId: string;
  homeTeam: string;
  awayTeam: string;
  startTime: string;
  status: string;
  liveMinute?: number;
}

export interface Outcome {
  id: string;
  name: string;
  odds: string;
  status: string;
}

export interface Market {
  id: string;
  eventId: string;
  name: string;
  status: string;
  outcomes: Outcome[];
}

export interface Bet {
  id: string;
  userId: string;
  eventId: string;
  marketId: string;
  outcomeId: string;
  odds: string;
  stake: string;
  potentialPayout: string;
  currency: string;
  status: string;
  referenceId: string;
  debitTransactionId?: string;
  creditTransactionId?: string;
  createdAt: string;
  settledAt?: string;
}

export interface EventsResponse {
  events: Event[];
}

export interface MarketsResponse {
  markets: Market[];
}

export interface BetsResponse {
  bets: Bet[];
}

export interface PlaceBetRequest {
  event_id: string;
  market_id: string;
  outcome_id: string;
  stake: string;
  currency: string;
}

export interface PlaceBetResponse {
  bet: Bet;
}

export interface SettleBetRequest {
  bet_id: string;
  outcome: "won" | "lost" | "cancelled";
}

export interface SettleBetResponse {
  bet: Bet;
}
```

**Methods** (add to `GoApiClient` class):

```ts
async listSports(): Promise<ApiResponse<{ sports: Sport[] }>> {
  return this.request("/api/sports", { method: "GET" });
}

async listEvents(): Promise<ApiResponse<EventsResponse>> {
  return this.request("/api/events", { method: "GET" });
}

async listMarkets(eventId: string): Promise<ApiResponse<MarketsResponse>> {
  return this.request(`/api/events/${encodeURIComponent(eventId)}/markets`, { method: "GET" });
}

async listOutcomes(marketId: string): Promise<ApiResponse<{ outcomes: Outcome[] }>> {
  return this.request(`/api/markets/${encodeURIComponent(marketId)}/outcomes`, { method: "GET" });
}

async placeBet(body: PlaceBetRequest): Promise<ApiResponse<PlaceBetResponse>> {
  return this.request("/api/bets", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

async listBets(): Promise<ApiResponse<BetsResponse>> {
  return this.request("/api/bets", { method: "GET" });
}

async getBet(betId: string): Promise<ApiResponse<{ bet: Bet }>> {
  return this.request(`/api/bets/${encodeURIComponent(betId)}`, { method: "GET" });
}

async settleBet(body: SettleBetRequest): Promise<ApiResponse<SettleBetResponse>> {
  return this.request("/api/admin/bets/settle", {
    method: "POST",
    body: JSON.stringify(body),
  });
}
```

**Verify**:

```bash
cd app && npx tsc --noEmit
```

Expected: exit 0.

### Step 2: Update the home page with a sportsbook link

In `app/app/page.tsx`, add a link to `/sportsbook` alongside the existing Wallet and Admin links.

```tsx
<Link href="/sportsbook">
  <Button variant="outline">Sportsbook</Button>
</Link>
```

**Verify**:

```bash
cd app && npm run lint
```

Expected: exit 0 (no new lint errors).

### Step 3: Create the sportsbook events and betting page

Create `app/app/sportsbook/page.tsx` with the following behavior:

- On mount, load events via `goApiClient.listEvents()`.
- For the selected event, load markets and outcomes.
- Let the user pick an outcome and enter a stake amount and currency.
- On submit, call `goApiClient.placeBet(...)` and show a success toast or error message.
- Use existing UI components (`Button`, `Card`, `Input`, `Label`) and Tailwind classes consistent with `app/app/wallet/page.tsx`.
- Show loading skeletons or text while fetching.

Use `encodeURIComponent` when building gateway URLs via the client.

Example layout (not prescriptive; match the existing style):

```tsx
<div className="container mx-auto max-w-2xl py-8 space-y-6">
  <h1 className="text-3xl font-bold">Sportsbook</h1>
  {/* event selector, market/outcome selector, stake input, place-bet button */}
</div>
```

**Verify**:

```bash
cd app && npx tsc --noEmit
```

### Step 4: Create the "My Bets" page

Create `app/app/sportsbook/bets/page.tsx`:

- On mount, load bets via `goApiClient.listBets()`.
- Display each bet with event, market, outcome, odds, stake, potential payout, status, and created time.
- Handle loading and error states.

**Verify**:

```bash
cd app && npx tsc --noEmit
```

### Step 5: Create the admin settlements page

Create `app/app/admin/settlements/page.tsx`:

- On mount, load pending bets via `goApiClient.listBets()` and filter to `status === "pending"` in the UI, or list all bets and allow settling any pending bet.
- For each pending bet, show a select/input for outcome (`won`, `lost`, `cancelled`) and a "Settle" button.
- On settle, call `goApiClient.settleBet({ bet_id: bet.id, outcome })`.
- Follow the same admin-page pattern as `app/app/admin/withdrawals/page.tsx`: no client-side role gate, rely on the API 403.

**Verify**:

```bash
cd app && npx tsc --noEmit
```

### Step 6: Final verification

**Verify**: `cd app && npm run lint` → exit 0.

**Verify**: `cd app && npm run build` → exit 0.

**Verify**: `cd .. && make build` → exit 0.

**Verify**: `git status --short` → only in-scope files modified.

## Test plan

- Manual: start the full stack (`docker compose up -d`), sign in via the Next.js frontend, navigate to Sportsbook, select an event/outcome, place a bet, view My Bets, then use the admin Settlements page to settle the bet.
- Type check: `npx tsc --noEmit` covers TypeScript correctness.
- Build: `npm run build` covers Next.js renderability.
- Lint: `npm run lint` covers style issues.

## Done criteria

- [ ] `app/lib/go-api-client.ts` exposes sportsbook types and methods.
- [ ] `app/app/page.tsx` links to `/sportsbook`.
- [ ] `app/app/sportsbook/page.tsx` lists events/markets/outcomes and places bets.
- [ ] `app/app/sportsbook/bets/page.tsx` lists authenticated user's bets.
- [ ] `app/app/admin/settlements/page.tsx` lists pending bets and allows settling.
- [ ] `cd app && npx tsc --noEmit` exits 0.
- [ ] `cd app && npm run lint` exits 0.
- [ ] `cd app && npm run build` exits 0.
- [ ] `make build` from repo root exits 0.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The gateway routes (`/api/events`, `/api/bets`, etc.) are not present in `internal/gateway/server.go`.
- `authClient.token()` is unavailable or the token shape changed.
- A verification fails twice after a reasonable fix attempt.

## Maintenance notes

- Future bet types (parlays, live betting) will require extending the Bet/Market types and UI selectors.
- Consider polling `listEvents()` if live odds updates are needed; for now the page loads once on mount.
- The admin settlements page trusts the backend for authorization; if a role-based UI gate is added later, keep the same API fallback behavior.
