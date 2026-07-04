# Better Auth Go — Sportsbook/Casino Checklist

## v1 Wallet Microservice

### Infrastructure & Setup
- [ ] Add `cmd/gateway` and `cmd/wallet` service entrypoints.
- [ ] Add wallet Postgres schema and `golang-migrate` migrations.
- [ ] Add NATS and Redis to Docker Compose.
- [ ] Add gRPC generation setup for gateway → wallet.
- [ ] Add `.env.example` for gateway and wallet services.
- [ ] Create `scripts/init_env.sh` to generate all `.env` files and secrets.
- [ ] Create `scripts/dev-up.sh` to start the local Docker Compose stack.
- [ ] Create `scripts/migrate.sh` to run wallet DB migrations.
- [ ] Create `scripts/test.sh` to run unit and integration tests.
- [ ] Create `scripts/upgrade.sh` to pull, migrate, rebuild, and restart the stack.
- [ ] Create `docker-compose.yml` to launch the full stack with one command.
- [ ] Add migration run on container startup for the wallet service.
- [ ] Add health checks for all Docker services.

### Gateway Service
- [ ] Verify JWT via Better Auth JWKS endpoint.
- [ ] Forward user context to wallet service via gRPC metadata.
- [ ] Expose public HTTP endpoints: `/api/wallet/*`, `/api/admin/*`, `/webhooks/xcash/*`.
- [ ] Add admin authorization middleware.
- [ ] Add rate limiting and structured logging.

### Wallet Service
- [ ] Implement `GetBalance` gRPC handler.
- [ ] Implement `ListTransactions` gRPC handler.
- [ ] Implement `GetDepositAddress` gRPC handler with xcash integration.
- [ ] Implement `RequestWithdrawal` gRPC handler.
- [ ] Implement `ProcessDepositWebhook` gRPC handler with HMAC validation.
- [ ] Implement `ListPendingWithdrawals` and `ReviewWithdrawal` gRPC handlers.
- [ ] Implement atomic wallet credit/debit with optimistic locking.
- [ ] Implement idempotent deposit processing by xcash `sys_no`.

### xcash Integration
- [ ] Implement HMAC-SHA256 signing for `GET /v1/deposit/address`.
- [ ] Implement webhook signature validation.
- [ ] Add xcash client abstraction and mock for tests.
- [ ] Return `ok` body for successful webhook responses.

### Next.js UI
- [ ] Wallet page: show USDT/USDC balances.
- [ ] Deposit page: select currency/chain, show deposit address, copy button.
- [ ] Transaction history page.
- [ ] Withdrawal request page: amount, address, chain.
- [ ] Admin withdrawals dashboard: list, approve, reject.

### Testing
- [ ] Unit tests: concurrent wallet credit/debit, idempotency, rejection reversal.
- [ ] gRPC contract tests.
- [ ] Mocked xcash webhook integration tests.
- [ ] End-to-end Docker Compose test for deposit flow.
- [ ] End-to-end Docker Compose test for withdrawal flow.

### Security & Production
- [ ] Add request ID logging and structured JSON logs.
- [ ] Add health checks for Postgres, Redis, NATS, xcash.
- [ ] Add Prometheus metrics.
- [ ] Add deposit/withdrawal limits (configurable, even if not enforced in v1).
- [ ] Add KYC/AML hooks in schema.

### Documentation
- [ ] Update `README.md` with new architecture and local dev instructions.
- [ ] Keep `AGENTS.md` updated with production-ready notes.
- [ ] Add runbook for xcash webhook troubleshooting.

## Microservices Roadmap

| Service | Purpose | Slice |
|---|---|---|
| **Gateway** | Public API, JWT verification, rate limiting | v1 |
| **Wallet** | Balances, deposits, withdrawals, ledger | v1 |
| **Sportsbook** | Events, odds, bet types, settlement | v2 |
| **Casino** | Games, RNG, provably fair | v2 |
| **Settlement** | Payouts, bet settlement | v2 |
| **Risk** | Limits, KYC/AML hooks, geolocation, fraud | v2 |
| **Notifications** | Webhooks, emails, SMS | v2 |
| **Admin** | Operator dashboard, user management, reports | v2 |
| **Reporting** | Analytics, audit logs, compliance | v3 |
| **Odds/Feed** | External sports data ingestion | v3 |
| **Scheduler** | Cron jobs, event triggers | v3 |

**Wallet service roadmap:** `docs/superpowers/specs/2026-07-04-wallet-microservice-roadmap.md`  
**Full platform roadmap:** `docs/superpowers/specs/2026-07-04-betmonster-roadmap.md`

## Future Slices (not in v1)

- [ ] Sportsbook engine (events, odds, bet types, settlement).
- [ ] Casino games (crash, slots, provably fair).
- [ ] Automated withdrawals via hot wallet.
- [ ] Multi-currency support beyond USDT/USDC.
- [ ] Risk management, KYC/AML, geolocation.
- [ ] Multi-tenant operator support.
- [ ] Native mobile apps or PWA.
