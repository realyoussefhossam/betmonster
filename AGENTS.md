# BetMonster

Open-source, self-hosted sportsbook/casino platform.

## Project Goal

Build an **open-source, self-hosted sportsbook/casino platform** — similar to how xcash is an open-source self-hosted crypto payment gateway, but aiming to match the feature depth of leading crypto betting platforms (e.g., 1xBet, Stake, Roobet, Shuffle, Rainbet). Operators should be able to clone the repo, run a few scripts, and deploy their own fully functional sportsbook/casino.

- **First slice:** wallet microservice (deposits via xcash, manual admin withdrawals, USDT/USDC balances).
- **Later slices:** sportsbook engine, casino games, settlement, risk management, KYC/AML, automated withdrawals, operator dashboard, multi-tenant support, and enterprise custody.

## Current Architecture

- **Next.js + Better Auth**: auth, sessions, UI, admin dashboard.
- **Go gateway service**: JWT verification via Better Auth JWKS, routes to internal services.
- **Go wallet service**: owns wallet DB, balance, ledger, deposits, withdrawals.
- **xcash**: self-hosted crypto payment gateway used **only for deposits**.
- **NATS**: events between services.
- **Redis**: cache and idempotency.
- **Postgres**: two logical databases — one for Better Auth (Next.js/Prisma), one for the wallet service.

## Key Decisions

- **Microservices from v1**: gateway and wallet are separate Go binaries with their own database.
- **Internal communication**: gRPC (gateway → wallet), NATS for events.
- **Wallet model**: per-currency balances (USDT, USDC). No conversion between currencies.
- **Deposits**: xcash per-user deposit address (`GET /v1/deposit/address`) + webhook (`type: deposit`).
- **Withdrawals**: manual admin in v1. The wallet debits on request, admin approves and supplies the on-chain tx hash.
- **xcash does not support withdrawals**. Withdrawals must be handled outside xcash.

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

Auth stays in Better Auth / Next.js for v1.

## Real Money Warning

**BetMonster handles real money.** Every line of code that touches balances, deposits, withdrawals, transactions, or settlement is financial infrastructure. Mistakes here can directly cause loss of funds.

Rules for money-related code:

- **Never change balance/transaction logic without tests.** Write the failing test first, then the code, then verify it passes.
- **Always verify arithmetic.** Use a battle-tested decimal library (`shopspring/decimal`). Never use `float` for money.
- **Always verify concurrency.** Wallet updates must use optimistic locking or `SELECT FOR UPDATE`. Run concurrency tests before claiming correctness.
- **Always verify idempotency.** Deposits and settlements must be idempotent. Replaying a webhook or a retry must never double-credit.
- **Never skip audit fields.** Every transaction records `balance_before`, `balance_after`, and `reference_id`.
- **Never bypass validation or authorization.** Webhook signatures, admin checks, and balance checks are non-negotiable.
- **When unsure, stop and ask.** Do not guess, do not hack around, and do not silently defer a money-safety concern.

## Money Safety Guarantees

- **No negative balances.** Balance checks must reject any operation that would make a wallet negative.
- **Ledger-first design.** Every balance change is recorded as a transaction first. Never update a wallet balance without a matching transaction row.
- **Immutable transactions.** Transaction rows are never deleted. Only their status and reviewed fields may change.
- **Atomic money operations.** A debit and the corresponding withdrawal request must happen in the same database transaction.
- **Reconciliation.** The wallet service must be able to reconcile `wallets.balance` against the sum of completed transactions for every user.
- **Circuit breakers for withdrawals.** Withdrawals require admin approval in v1. In later versions, automated withdrawals must have daily limits, address allowlists, and anomaly detection.
- **Webhook confirmations.** Do not credit a deposit until xcash confirms the required block confirmations and the webhook signature is valid.
- **Reversibility.** Failed or rejected operations must reverse any debits or holds in the same transaction.
- **Disaster recovery.** Every financial event is logged immutably. Backups of the wallet database are required before any production migration.
- **Operator funds are separate.** Player wallet balances and operator/hot-wallet funds are never mixed.

## Production-Ready Notes

- **Financial safety first**:
  - Every balance change must be audited (`balance_before`, `balance_after`, `reference_id`).
  - Wallet updates use optimistic locking (`version` column) or `SELECT FOR UPDATE`.
  - Deposits are idempotent by `sys_no` (xcash) to prevent double crediting.
  - Withdrawals debit first, then require admin approval before completion.

- **Security**:
  - JWT verification only in the gateway.
  - Internal services must not be publicly reachable.
  - Use mTLS or internal VPC for gateway ↔ wallet in production.
  - Validate xcash webhook HMAC signature.
  - Never log private keys or webhook secrets.

- **Compliance hooks**:
  - v1 does not enforce KYC/AML, but schema includes `kyc_status` and `withdrawal.kyc_required`.
  - Add deposit/withdrawal limits, self-exclusion, and geolocation in later slices.
  - Licensing, jurisdiction, and custody policies are operator responsibilities.

- **Observability**:
  - Structured JSON logs with request IDs and user IDs.
  - Prometheus metrics for wallet operations, xcash webhooks, and gRPC latency.
  - Health checks for Postgres, Redis, NATS, and xcash reachability.

- **Testing**:
  - Unit tests for ledger math and concurrency.
  - gRPC contract tests.
  - Webhook integration tests with a mocked xcash server.
  - End-to-end Docker Compose tests for deposit and withdrawal flows.

## Design Spec

- **Wallet v1 spec**: `docs/superpowers/specs/2026-07-04-wallet-microservice-design.md`

## Deployment Goal

The production stack must be launchable in three commands, like xcash:

```bash
git clone https://github.com/realyoussefhossam/betmonster.git
cd betmonster
./scripts/init_env.sh
docker compose up -d
```

- `init_env.sh` generates all `.env` files and secrets.
- `docker compose up -d` starts Next.js, gateway, wallet, Postgres, Redis, and NATS.
- xcash is expected to be self-hosted separately, but its credentials are generated into our `.env` files.
- Helper scripts: `dev-up.sh`, `migrate.sh`, `test.sh`, `upgrade.sh`.

## How to Work in This Repo

- Read the spec and `CHECKLIST.md` before touching code.
- Write tests before implementation for wallet/balance logic.
- Update this `AGENTS.md` with new production-ready notes as we learn them.
- Update `CHECKLIST.md` as tasks are completed or new slices are added.
- Keep Better Auth tables untouched by the wallet service; use `user_id` as the cross-service link.
