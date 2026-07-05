# BetMonster v1 Wallet Microservice Design

**Scope:** first slice of the BetMonster open-source, self-hosted sportsbook/casino platform.

**Date:** 2026-07-04  
**Status:** draft — pending implementation plan

## 1. Context

BetMonster is an open-source, self-hosted sportsbook/casino platform. The existing repo started as a Next.js + Better Auth frontend paired with a Go API that only verifies JWTs. The goal is to evolve it into a full platform that operators can deploy with a few commands, similar to how xcash is an open-source self-hosted crypto payment gateway. The first slice is the **wallet microservice**, because every other subsystem (sportsbook, casino, settlement) depends on it.

## 2. Goals

- Allow users to deposit **USDT and USDC** via a self-hosted **xcash** crypto gateway.
- Maintain a per-currency wallet balance with an auditable transaction ledger.
- Support **manual admin withdrawals** in v1 (automated payouts are a later slice).
- Provide the foundation for future betting/casino services to debit/credit wallets.

## 3. Non-Goals

- No fiat on/off ramps.
- No real betting or casino games in this slice.
- No KYC/AML enforcement in v1 (schema will include hooks for later).
- No automated on-chain payouts in v1.

## 4. Architecture

```
┌─────────────┐      ┌──────────────┐      ┌──────────────┐
│   Next.js   │──────▶│   Gateway    │──────▶│   Wallet     │
│  (Better    │ HTTP  │   Service    │ gRPC  │   Service    │
│   Auth UI)  │◀─────│  (JWT/JWKS)  │◀─────│  (Postgres)  │
└─────────────┘      └──────────────┘      └──────────────┘
                                                    │
                                                    ▼
                                            ┌──────────────┐
                                            │    xcash     │
                                            │  (deposits)  │
                                            └──────────────┘
```

### Components

- **Next.js frontend**: auth, UI, admin dashboard. Proxies wallet calls to the Go gateway.
- **Gateway service** (`cmd/gateway`): verifies JWTs via Better Auth JWKS, routes to internal services, exposes public HTTP.
- **Wallet service** (`cmd/wallet`): owns the wallet database. Handles balance, deposits, withdrawals, ledger.
- **xcash**: self-hosted Django crypto gateway used only for deposit addresses and deposit webhooks.
- **NATS**: event bus for wallet events (`deposit.confirmed`, `withdrawal.requested`).
- **Redis**: cache and idempotency key store.
- **Postgres (wallet DB)**: separate from the Better Auth database.

## 5. Currency Model

- Each user has one wallet per currency.
- v1 supports every asset and chain that xcash can process. xcash handles EVM chains (e.g., `anvil`, `base`, `ethereum`, `bsc`) and Tron.
- The schema is asset-agnostic; future non-xcash assets and chains can be added without changing the wallet table.
- No currency conversion is performed; deposits credit the matching wallet.

### Asset Roadmap

| Asset | Networks | v1 status | Notes |
|-------|----------|-----------|-------|
| USDT | ERC20, TRC20, BEP20, Base, other EVM | **Supported** | Default stablecoin. |
| USDC | ERC20, TRC20, BEP20, Base, other EVM | **Supported** | Alternative stablecoin. |
| ETH | Ethereum, Base, other EVM | **Supported** | Native EVM asset. |
| BETM | ERC20, other EVM | **Native token** (optional) | Project-native token used for gaming features and rewards. Operators can rename the ticker. |
| BNB | BNB Smart Chain | **Supported** | EVM-compatible chain. |
| TRX | Tron Network | **Supported** | Very low fees. |
| BTC | Bitcoin Network | Future | Non-EVM / non-Tron chain. |
| SOL | Solana | Future | Non-EVM / non-Tron chain. |
| LTC | Litecoin | Future | Non-EVM / non-Tron chain. |
| DOGE | Dogecoin Network | Future | Non-EVM / non-Tron chain. |
| XRP | XRP Ledger | Future | Non-EVM / non-Tron chain; requires destination tags. |

v1 operators can enable any EVM or Tron asset/chain that xcash supports by updating `SUPPORTED_CURRENCIES` and `SUPPORTED_CHAINS`. BTC, SOL, LTC, DOGE, and XRP require non-EVM pipeline work and are out of scope for v1.

## 6. Deposit Flow

1. User selects currency and chain on the deposit page.
2. Next.js calls `GET /api/wallet/deposit-address?currency=USDT&chain=base` via the gateway.
3. Gateway verifies the JWT, extracts `user_id`, and forwards the request to the wallet service.
4. Wallet service checks for an existing active deposit address for `(user_id, currency, chain)`. If none, it calls xcash:
   ```text
   GET /v1/deposit/address?uid={user_id}&chain=base&crypto=USDT
   ```
   with HMAC-SHA256 signing using the project's `appid` and `hmac_key`. It stores the returned `deposit_address` and returns it.
5. User sends crypto to the address.
6. xcash detects the deposit and sends a webhook to `POST /webhooks/xcash/deposit` on the gateway.
7. Gateway forwards the webhook to the wallet service.
8. Wallet service validates the webhook HMAC signature, deduplicates by `sys_no`, credits the matching wallet, and records a `deposit` transaction with `balance_before` and `balance_after`.
9. Wallet service responds with `200` and body `ok` (required by xcash for webhook success).

### xcash Deposit Webhook Payload

```json
{
  "type": "deposit",
  "data": {
    "sys_no": "DXC2606026K9P2QWX",
    "uid": "user-10001",
    "chain": "base",
    "block": 12345678,
    "hash": "0xabc123...",
    "crypto": "USDC",
    "amount": "500",
    "confirmed": true,
    "risk_level": null,
    "risk_score": null
  }
}
```

## 7. Withdrawal Flow

1. User requests withdrawal: currency, amount, destination address, chain.
2. Gateway verifies JWT and forwards to the wallet service.
3. Wallet service validates the balance, atomically debits the wallet, and creates a `withdrawal_requests` record in `pending` state plus a pending `withdrawal` transaction.
4. Admin reviews pending withdrawals in the dashboard.
5. Admin manually sends the crypto from the operations wallet, then submits the transaction hash.
6. Wallet service marks the request as `completed` and updates the transaction status.
7. If rejected, the wallet debit is reversed in the same transaction.

## 8. Data Model

```sql
wallets
  id uuid primary key
  user_id text not null
  currency text not null
  balance numeric(28,8) not null default 0
  version int not null default 0
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
  unique(user_id, currency)

transactions
  id uuid primary key
  user_id text not null
  wallet_id uuid not null references wallets(id)
  type text not null check (type in ('deposit','withdrawal','bet','win','fee','adjustment'))
  amount numeric(28,8) not null
  balance_before numeric(28,8) not null
  balance_after numeric(28,8) not null
  status text not null check (status in ('pending','completed','failed'))
  reference_id text unique
  metadata jsonb
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()

deposit_addresses
  id uuid primary key
  user_id text not null
  currency text not null
  chain text not null
  address text not null
  xcash_deposit_id text
  status text not null check (status in ('active','archived'))
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
  -- at most one active deposit address per user/currency/chain
  create unique index idx_active_deposit_address
    on deposit_addresses(user_id, currency, chain)
    where status = 'active'

withdrawal_requests
  id uuid primary key
  user_id text not null
  wallet_id uuid not null references wallets(id)
  amount numeric(28,8) not null
  currency text not null
  destination_address text not null
  chain text not null
  status text not null check (status in ('pending','approved','rejected','completed'))
  tx_hash text
  reviewed_by text
  created_at timestamptz not null default now()
  updated_at timestamptz not null default now()
```

## 9. API Surface

| Endpoint | Method | Description |
|---|---|---|
| `/api/wallet/balance` | GET | Get balance for a currency |
| `/api/wallet/transactions` | GET | Paginated transaction history |
| `/api/wallet/deposit-address` | GET | Get or create a deposit address |
| `/api/wallet/withdraw` | POST | Request a withdrawal |
| `/api/admin/withdrawals` | GET | List pending withdrawals |
| `/api/admin/withdrawals/:id/approve` | POST | Approve withdrawal + record tx hash |
| `/api/admin/withdrawals/:id/reject` | POST | Reject withdrawal + reverse debit |
| `/webhooks/xcash/deposit` | POST | xcash deposit webhook |

## 10. gRPC Contract (Gateway → Wallet)

```protobuf
service WalletService {
  rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);
  rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse);
  rpc GetDepositAddress(GetDepositAddressRequest) returns (GetDepositAddressResponse);
  rpc RequestWithdrawal(RequestWithdrawalRequest) returns (RequestWithdrawalResponse);
  rpc ProcessDepositWebhook(ProcessDepositWebhookRequest) returns (ProcessDepositWebhookResponse);
  rpc ListPendingWithdrawals(ListPendingWithdrawalsRequest) returns (ListPendingWithdrawalsResponse);
  rpc ReviewWithdrawal(ReviewWithdrawalRequest) returns (ReviewWithdrawalResponse);
}

message GetBalanceRequest { string user_id = 1; string currency = 2; }
message GetBalanceResponse { string currency = 1; string balance = 2; }

message ListTransactionsRequest { string user_id = 1; int32 page = 2; int32 page_size = 3; }
message ListTransactionsResponse { repeated Transaction transactions = 1; }

message GetDepositAddressRequest { string user_id = 1; string currency = 2; string chain = 3; }
message GetDepositAddressResponse { string address = 1; string chain = 2; string currency = 3; }

message RequestWithdrawalRequest {
  string user_id = 1;
  string currency = 2;
  string amount = 3;
  string destination_address = 4;
  string chain = 5;
}
message RequestWithdrawalResponse { string withdrawal_id = 1; string status = 2; }

message ProcessDepositWebhookRequest { string body = 1; map<string, string> headers = 2; }
message ProcessDepositWebhookResponse { string response_body = 1; }

message ListPendingWithdrawalsRequest { int32 page = 1; int32 page_size = 2; }
message ListPendingWithdrawalsResponse { repeated WithdrawalRequest withdrawals = 1; }

message ReviewWithdrawalRequest {
  string withdrawal_id = 1;
  string action = 2; // approve | reject
  string tx_hash = 3; // required when action = approve
  string reviewed_by = 4;
}
message ReviewWithdrawalResponse { string status = 1; }
```

## 11. Security

- **JWT verification** happens only in the gateway using the existing Better Auth JWKS endpoint.
- **Internal service trust**: gateway passes `X-User-ID` or gRPC metadata to the wallet service. Wallet service rejects any public request without it.
- **Webhook signature validation**: xcash webhooks are validated using HMAC-SHA256 and the project's `hmac_key`.
- **Idempotency**: deposits are deduplicated by `sys_no` and `reference_id`.
- **Admin authorization**: admin endpoints require a configured admin role or Better Auth role metadata.
- **Audit trail**: every balance change records `balance_before` and `balance_after`.
- **mTLS/internal network**: gateway-to-wallet traffic should be restricted to internal network or mTLS in production.

## 12. Error Handling

- Optimistic locking failures on wallet rows are retried with exponential backoff.
- xcash webhook failures return a non-2xx status so xcash retries.
- Failed deposits are recorded as `failed` transactions and never credited.
- Withdrawal rejection reverses the debit in the same database transaction.

## 13. Testing

- Unit tests for wallet balance operations (credit, debit, idempotency, concurrency).
- gRPC contract tests for gateway ↔ wallet.
- Webhook integration tests with a mocked xcash server.
- Docker Compose integration test for the full deposit/withdrawal flow.

## 14. Deployment

The goal is a **one-command production stack** similar to xcash:

```bash
git clone https://github.com/realyoussefhossam/betmonster.git
cd betmonster
./scripts/init_env.sh   # generate all secrets and .env files
docker compose up -d     # launch the full stack
```

### What `docker compose up -d` starts

- Next.js frontend
- Go gateway service
- Go wallet service
- Postgres (Better Auth database)
- Postgres (wallet database)
- Redis
- NATS

### What `scripts/init_env.sh` generates

- `app/.env` — Next.js / Better Auth secrets, database URL, xcash project credentials.
- `api/.env` (or `gateway/.env`) — gateway service secrets, JWKS URL, internal service credentials.
- `wallet/.env` — wallet DB connection, Redis/NATS URLs, xcash API credentials.
- `xcash` is expected to be self-hosted separately and configured as a project; its `appid` and `hmac_key` are written into our `.env` files.

### Helper Scripts

- `scripts/dev-up.sh` — starts the local Docker Compose stack.
- `scripts/migrate.sh` — runs `golang-migrate` against the wallet database.
- `scripts/test.sh` — runs unit, gRPC contract, and integration tests.
- `scripts/upgrade.sh` — pulls the latest code, runs migrations, rebuilds, and restarts the stack.

### Migrations

- Wallet migrations live in `wallet/migrations/` and use `golang-migrate`.
- The Docker Compose setup runs migrations automatically before starting the wallet service.
- Prisma migrations for Better Auth tables remain in `app/prisma/` and are handled by the Next.js container on startup.

## 15. Compliance & Risk

- v1 does not include KYC/AML enforcement, but the schema includes `kyc_status` and `withdrawal.kyc_required` flags.
- Deposit limits and withdrawal limits are configurable per user or globally.
- Audit trail supports future compliance reporting.
- **Operator responsibility:** licensing, jurisdictional compliance, KYC/AML policies, and custody legal requirements are outside the scope of this design.

## 16. Wallet Microservice Roadmap

This v1 design is the **foundation** of the Wallet microservice. The wallet service will expand from simple USDT/USDC support to a production-ready wallet with multi-currency, automated withdrawals, compliance, reconciliation, and enterprise custody. A full roadmap is in `docs/superpowers/specs/2026-07-04-wallet-microservice-roadmap.md`.

## 17. Platform Microservices Roadmap

The full BetMonster platform is composed of the following microservices. v1 builds only the services marked as **v1**.

| Service | Purpose | Slice |
|---|---|---|
| **Gateway** | Public API, JWT verification, rate limiting, routing to internal services | v1 |
| **Wallet** | Balances, deposits, withdrawals, transaction ledger | v1 |
| **Sportsbook** | Events, odds, bet types, bet placement, settlement | v2 |
| **Casino** | Games, RNG, provably fair, game state, house edge | v2 |
| **Settlement** | Payouts, bet settlement, win/loss calculations | v2 |
| **Risk** | Deposit/withdrawal limits, KYC/AML hooks, geolocation, fraud scoring | v2 |
| **Notifications** | Webhooks, emails, SMS, in-app notifications | v2 |
| **Admin** | Operator dashboard, user management, withdrawals, reports | v2 |
| **Reporting** | Analytics, audit logs, compliance reports | v3 |
| **Odds/Feed** | Ingest external sports data and odds feeds | v3 |
| **Scheduler** | Cron jobs, event start/end triggers, settlement scheduling | v3 |

**Auth** is handled by Better Auth (Next.js / Prisma) rather than a dedicated microservice in v1. A separate auth service may be introduced later if needed.

## 18. Future Features

- Automated withdrawals via hot wallet.
- Multi-currency support beyond USDT/USDC.
- Multi-tenant operator support.
- Native mobile apps or PWA.
