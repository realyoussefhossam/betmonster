# BetMonster — v1 Wallet + v2 Odds/Feed Checklist

## v1 Wallet Microservice

### Infrastructure & Setup
- [x] Add `cmd/gateway` and `cmd/wallet` service entrypoints.
- [x] Add wallet Postgres schema and `golang-migrate` migrations.
- [x] Add NATS and Redis to Docker Compose.
- [x] Add gRPC generation setup for gateway → wallet.
- [x] Add `.env.example` for gateway and wallet services.
- [x] Create `scripts/init_env.sh` to generate all `.env` files and secrets.
- [x] Create `scripts/dev-up.sh` to start the local Docker Compose stack.
- [x] Create `scripts/migrate.sh` to run wallet DB migrations.
- [ ] Create `scripts/test.sh` to run unit and integration tests.
- [ ] Create `scripts/upgrade.sh` to pull, migrate, rebuild, and restart the stack.
- [x] Create `docker-compose.yml` to launch the full stack with one command.
- [x] Add migration run on container startup for the wallet service.
- [x] Add health checks for all Docker services.

### Gateway Service
- [x] Verify JWT via Better Auth JWKS endpoint.
- [x] Forward user context to wallet service via gRPC metadata.
- [x] Expose public HTTP endpoints: `/api/wallet/*`, `/api/admin/*`, `/webhooks/xcash/*`.
- [x] Add admin authorization middleware.
- [x] Add structured logging.
- [x] Add rate limiting.

### Wallet Service
- [x] Implement `GetBalance` gRPC handler.
- [x] Implement `ListTransactions` gRPC handler.
- [x] Implement `GetDepositAddress` gRPC handler with xcash integration.
- [x] Implement `RequestWithdrawal` gRPC handler.
- [x] Implement `ProcessDepositWebhook` gRPC handler with HMAC validation.
- [x] Implement `ListPendingWithdrawals` and `ReviewWithdrawal` gRPC handlers.
- [x] Implement atomic wallet credit/debit with optimistic locking.
- [x] Implement idempotent deposit processing by xcash `sys_no`.

### xcash Integration
- [x] Implement HMAC-SHA256 signing for `GET /v1/deposit/address`.
- [x] Implement webhook signature validation.
- [x] Add xcash client abstraction and mock for tests.
- [x] Return `ok` body for successful webhook responses.
- [x] Add `scripts/setup-xcash.sh` to auto-provision a local xcash + anvil test instance.

### Next.js UI
- [x] Wallet page: show USDT/USDC balances.
- [x] Deposit page: select currency/chain, show deposit address.
- [x] Transaction history page.
- [x] Withdrawal request page: amount, address, chain.
- [x] Admin withdrawals dashboard: list, approve, reject.

### Testing
- [x] Unit tests: wallet credit/debit, idempotency.
- [x] Unit tests: rejection reversal.
- [x] Unit tests: concurrent wallet credit/debit (via PGStore integration tests).
- [x] gRPC contract tests.
- [x] Mocked xcash webhook integration tests.
- [ ] End-to-end Docker Compose test for deposit flow.
- [x] End-to-end Docker Compose test for withdrawal flow (manual).

### Security & Production
- [x] Add structured JSON logs.
- [x] Add request ID logging.
- [x] Add health checks for Postgres, Redis, NATS services.
- [ ] Add Prometheus metrics.
- [x] Add deposit/withdrawal limits (configurable, even if not enforced in v1).
- [ ] Add KYC/AML hooks in schema.

### Documentation
- [x] Update `README.md` with new architecture and local dev instructions.
- [x] Document `SUPPORTED_PAIRS` configuration and how to add/remove assets.
- [x] Keep `AGENTS.md` updated with production-ready notes.
- [ ] Add runbook for xcash webhook troubleshooting.

## Microservices Roadmap

| Service | Purpose | Slice |
|---|---|---|
| **Gateway** | Public API, JWT verification, rate limiting | v1 |
| **Wallet** | Balances, deposits, withdrawals, ledger | v1 |
| **Sportsbook** | Events, odds, single moneyline bet placement, settlement | v1 |
| **Casino** | Games, RNG, provably fair | v2 |
| **Settlement** | Payouts, bet settlement | v2 |
| **Risk** | Limits, KYC/AML hooks, geolocation, fraud | v2 |
| **Notifications** | Webhooks, emails, SMS | v2 |
| **Admin** | Operator dashboard, user management, reports | v2 |
| **Reporting** | Analytics, audit logs, compliance | v3 |
| **Odds/Feed** | External sports data ingestion | v2 |
| **Scheduler** | Cron jobs, event triggers | v3 |

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

**Wallet service roadmap:** `docs/superpowers/specs/2026-07-04-wallet-microservice-roadmap.md`  
**Full platform roadmap:** `docs/superpowers/specs/2026-07-04-betmonster-roadmap.md`

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

## Future Slices (not in v1)

- [ ] Advanced sportsbook features (parlays, live betting, cash-out).
- [ ] Casino games (crash, slots, provably fair).
- [ ] Automated withdrawals via hot wallet.
- [ ] Non-EVM / non-Tron assets (BTC, SOL, LTC, DOGE, XRP).
- [ ] Native token support (e.g., BETM) for platform gaming and rewards.
- [ ] Risk management, KYC/AML, geolocation.
- [ ] Multi-tenant operator support.
- [ ] Native mobile apps or PWA.
