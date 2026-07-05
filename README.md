> BetMonster is an open-source, self-hosted sportsbook/casino platform. This repo contains the v1 wallet microservice (gateway + wallet) and the Next.js frontend.

# BetMonster

BetMonster is an open-source, self-hosted sportsbook/casino platform. The v1 focus is a **wallet microservice** that supports USDT/USDC deposits via [xcash](https://github.com/xca-sh/xcash) and manual admin withdrawals.

## Supported Assets

The platform is designed to support multiple crypto assets. The current v1 implementation supports every asset and chain that xcash can process. The wallet schema and gateway are asset-agnostic, so additional non-xcash assets can be added later.

| Asset | Networks | v1 status | Notes |
|-------|----------|-----------|-------|
| USDT | ERC20, TRC20, BEP20, Base, other EVM | **Supported** | Default stablecoin for wagering. |
| USDC | ERC20, TRC20, BEP20, Base, other EVM | **Supported** | Alternative stablecoin. |
| ETH | Ethereum, Base, other EVM | **Supported** | Native EVM asset. |
| BETM | ERC20, other EVM | **Native token** (optional) | Project-native token used for gaming features and rewards. Rename to your project ticker. |
| BNB | BNB Smart Chain | **Supported** | EVM-compatible chain. |
| TRX | Tron Network | **Supported** | Very low fees. |
| BTC | Bitcoin Network | Future | Non-EVM / non-Tron chain. |
| SOL | Solana | Future | Non-EVM / non-Tron chain. |
| LTC | Litecoin | Future | Non-EVM / non-Tron chain. |
| DOGE | Dogecoin Network | Future | Non-EVM / non-Tron chain. |
| XRP | XRP Ledger | Future | Non-EVM / non-Tron chain; requires destination tags. |

The v1 gateway defaults to `SUPPORTED_CURRENCIES=USDT,USDC` and `SUPPORTED_CHAINS=anvil`. Operators can enable any EVM or Tron asset/chain that xcash supports by updating `SUPPORTED_CURRENCIES` and `SUPPORTED_CHAINS`. BTC, SOL, LTC, DOGE, and XRP require non-EVM pipeline work and are out of scope for v1.

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
./scripts/setup-xcash.sh   # optional: starts a local xcash + anvil for testing
docker compose up -d
```

`setup-xcash.sh` clones the [xcash](https://github.com/xca-sh/xcash) repo into `deps/xcash`, starts the full xcash stack plus an anvil EVM test chain, and writes the generated `XCASH_APPID`, `XCASH_HMAC_KEY`, and `XCASH_WEBHOOK_SECRET` into your `.env`.

**Important:** `setup-xcash.sh` must run **before** `docker compose up -d` so the shared `xcash_public` Docker network exists. The BetMonster wallet container joins this network to reach xcash internally at `http://xcash-caddy:80`. The local anvil chain is configured with the chain code `anvil`, so use that when testing deposits from the UI or API.

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
| `GET /api/wallet/deposit-address?currency=USDT&chain=polygon` | Get or create xcash deposit address |
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
