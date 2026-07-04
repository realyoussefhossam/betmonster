> BetMonster is an open-source, self-hosted sportsbook/casino platform. This repo contains the v1 wallet microservice (gateway + wallet) and the Next.js frontend.

# BetMonster

BetMonster is an open-source, self-hosted sportsbook/casino platform. The v1 focus is a **wallet microservice** that supports USDT/USDC deposits via [xcash](https://github.com/xca-sh/xcash) and manual admin withdrawals.

## Architecture

- **Next.js + Better Auth**: auth, sessions, UI, admin dashboard.
- **Go Gateway**: JWT verification via Better Auth JWKS, public HTTP API, routes to wallet.
- **Go Wallet**: owns wallet DB, balances, ledger, deposits, withdrawals.
- **xcash**: self-hosted crypto payment gateway used only for deposits.
- **Postgres**: Better Auth (Next.js/Prisma) and wallet service databases.
- **Redis**: cache and idempotency (v1 lays the groundwork).
- **NATS**: events between services (v1 lays the groundwork).

Internal gateway → wallet communication uses **gRPC**.

## Quick Start (Docker Compose)

Requires [Docker](https://docs.docker.com/get-docker/) and [Go 1.26+](https://go.dev/dl/).

```bash
git clone https://github.com/realyoussefhossam/betmonster.git
cd betmonster
./scripts/init_env.sh
# Edit .env and add your xcash credentials (XCASH_APPID, XCASH_HMAC_KEY, XCASH_WEBHOOK_SECRET)
docker compose up -d
```

This starts:

- Next.js app on http://localhost:3000
- Gateway on http://localhost:8080
- Wallet gRPC on localhost:50051 and health on http://localhost:8081
- Postgres on localhost:5433
- Redis on localhost:6379
- NATS on localhost:4222

## Local Development (non-Docker)

1. Start the infrastructure:
   ```bash
   ./scripts/dev-up.sh
   ```
2. Start the wallet service:
   ```bash
   go run ./cmd/wallet
   ```
3. Start the gateway:
   ```bash
   go run ./cmd/gateway
   ```
4. Start the Next.js app:
   ```bash
   cd app
   npm install
   npm run dev
   ```

## Useful Commands

```bash
# Run all Go tests
make test

# Build all binaries
make build

# Run wallet migrations
make migrate

# Regenerate gRPC code
make proto
```

## Wallet Endpoints

| Gateway Endpoint | Description |
|------------------|-------------|
| `GET /api/wallet/balance?currency=USDT` | User balance |
| `GET /api/wallet/transactions` | Transaction history |
| `GET /api/wallet/deposit-address?currency=USDT&chain=base` | Get or create xcash deposit address |
| `POST /api/wallet/withdraw` | Request a withdrawal |
| `GET /api/admin/withdrawals` | List pending withdrawals (admin only) |
| `POST /api/admin/withdrawals/review` | Approve/reject a withdrawal (admin only) |
| `POST /webhooks/xcash/deposit` | xcash deposit webhook |

## Docs

- [Wallet microservice design](docs/superpowers/specs/2026-07-04-wallet-microservice-design.md)
- [Wallet microservice roadmap](docs/superpowers/specs/2026-07-04-wallet-microservice-roadmap.md)
- [Platform roadmap](docs/superpowers/specs/2026-07-04-betmonster-roadmap.md)
- [CHECKLIST.md](CHECKLIST.md)

## License

MIT
