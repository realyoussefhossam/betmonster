> BetMonster is an open-source, self-hosted sportsbook/casino platform. This repo contains the v1 wallet microservice (gateway + wallet) and the Next.js frontend.

# BetMonster

BetMonster is an open-source, self-hosted sportsbook/casino platform. The v1 focus is a **wallet microservice** that supports USDT/USDC deposits via [xcash](https://github.com/xca-sh/xcash) and manual admin withdrawals.

## Supported Assets

The v1 implementation supports the EVM and Tron assets that xcash can process. Support is configured through currency-chain pairs (`SUPPORTED_PAIRS`), so the gateway and wallet only accept combinations that you explicitly enable. The wallet schema and gateway are asset-agnostic, so additional non-xcash assets can be added later.

| Asset | Networks | v1 status | Notes |
|-------|----------|-----------|-------|
| USDT | ERC20, TRC20, BEP20, Polygon | **Supported** | Default stablecoin for wagering. |
| USDC | ERC20, BEP20, Polygon, Base, Arbitrum One | **Supported** | Alternative stablecoin. |
| ETH | Ethereum, BNB Smart Chain, Base | **Supported** | Native EVM asset. |
| BNB | BNB Smart Chain | **Supported** | Native BSC asset. |
| TRX | Tron Network | **Supported** | Native Tron asset. |
| POL | Polygon, Ethereum | **Supported** | Polygon native token. |
| DAI | Ethereum | **Supported** | Stablecoin. |
| SHIB | Ethereum | **Supported** | Meme token. |
| BUSD | BNB Smart Chain | **Supported** | Binance stablecoin. |
| BETM | ERC20 | **Native token** (optional) | Project-native token used for gaming features and rewards. Rename to your project ticker. |
| BTC | Bitcoin Network | Future | Non-EVM / non-Tron chain. |
| SOL | Solana | Future | Non-EVM / non-Tron chain. |
| LTC | Litecoin | Future | Non-EVM / non-Tron chain. |
| DOGE | Dogecoin Network | Future | Non-EVM / non-Tron chain. |
| XRP | XRP Ledger | Future | Non-EVM / non-Tron chain; requires destination tags. |
| AVAX | Avalanche C-Chain | Future | xcash does not support Avalanche in v1. |
| TON / GRAM | TON | Future | Non-EVM / non-Tron chain; requires memo/tag. |

### Configuring pairs

Set `SUPPORTED_PAIRS` in `.env` to define which currency exists on which chain. The default in `.env.example` includes `anvil:*` pairs for local testing with `setup-xcash.sh`; remove them in production.

```bash
SUPPORTED_PAIRS=USDT:anvil,USDT:ethereum,USDT:bsc,USDT:polygon,USDT:tron,USDC:anvil,USDC:ethereum,USDC:bsc,USDC:polygon,USDC:base,USDC:arbitrum-one,ETH:anvil,ETH:ethereum,ETH:bsc,ETH:base,BNB:bsc,TRX:tron,POL:ethereum,POL:polygon,DAI:ethereum,SHIB:ethereum,BUSD:bsc
```

`SUPPORTED_CURRENCIES` and `SUPPORTED_CHAINS` are derived from `SUPPORTED_PAIRS` automatically. You can set them explicitly if you need to hide a currency or chain from the UI without removing the underlying pair.

### Adding or removing assets

1. Register the currency and chain in xcash (for EVM: activate the chain and map the token contract via `CryptoOnChain`; for Tron: only `USDT` and `TRX` are supported for deposit addresses).
2. Add or remove the pair from `SUPPORTED_PAIRS` in `.env`.
3. Restart the gateway and wallet containers.

BTC, SOL, LTC, DOGE, and XRP require non-EVM pipeline work and are out of scope for v1.

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
