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

Set `SUPPORTED_PAIRS` in `.env` to define which currency exists on which chain. `SUPPORTED_CURRENCIES` and `SUPPORTED_CHAINS` are derived from it automatically. You can set them explicitly if you need to hide a currency or chain from the UI without removing the underlying pair.

The default `.env.example` includes `anvil:*` pairs for local testing with `setup-xcash.sh`. Remove `anvil:*` in production.

```bash
SUPPORTED_PAIRS=USDT:anvil,USDT:ethereum,USDT:bsc,USDT:polygon,USDT:tron,USDC:anvil,USDC:ethereum,USDC:bsc,USDC:polygon,USDC:base,USDC:arbitrum-one,ETH:anvil,ETH:ethereum,ETH:bsc,ETH:base,BNB:bsc,TRX:tron,POL:ethereum,POL:polygon,DAI:ethereum,SHIB:ethereum,BUSD:bsc
```

### Local vs production

`scripts/setup-xcash.sh` only provisions the local **anvil** chain with **USDT**, **USDC**, and **ETH** mapped to it. Only `USDT:anvil`, `USDC:anvil`, and `ETH:anvil` will actually work for deposits in the local Docker stack. The other pairs are kept in `.env` as the production reference; deposit calls for non-anvil pairs will fail against the local xcash because those chains are not active locally.

### Adding or removing assets

1. Register the currency and chain in your xcash deployment (for EVM: activate the chain and map the token contract via `CryptoOnChain`; for Tron: only `USDT` and `TRX` are supported for deposit addresses).
2. Add or remove the pair from `SUPPORTED_PAIRS` in `.env`.
3. Restart the gateway and wallet containers.

BTC, SOL, LTC, DOGE, XRP, AVAX, and TON require non-EVM/non-Tron pipeline work and are out of scope for v1.

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

## Local Testing Scripts

`scripts/send_anvil.py` is a helper that runs inside the xcash Django container to mint or transfer test funds on the local anvil chain. It reads the token contract address and decimals from the xcash database, so it works for any currency that xcash has provisioned on the `anvil` chain.

```bash
# Copy the script into the running xcash container
docker cp scripts/send_anvil.py xcash_django:/tmp/send_anvil.py

# Mint USDT (default when first arg is an address)
docker exec xcash_django python /tmp/send_anvil.py 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 20

# Explicit currency: USDT
docker exec xcash_django python /tmp/send_anvil.py USDT 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 20

# Explicit currency: USDC
docker exec xcash_django python /tmp/send_anvil.py USDC 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 5

# Native ETH transfer
docker exec xcash_django python /tmp/send_anvil.py ETH 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 0.5
```

The script auto-detects native tokens (e.g. ETH) by their empty contract address in `CryptoOnChain` and performs a regular value transfer instead of an ERC20 `mint`. ERC20 tokens must have a deployed mock contract; the script prints a clear error if the contract is missing (usually because the anvil chain was restarted and the mocks need to be redeployed).

### Redeploying anvil mock contracts

If the anvil chain is reset (block number drops back near zero), the xcash database will hold stale contract addresses. Re-run the bootstrap to deploy fresh mocks and update the mappings:

```bash
docker cp scripts/xcash_bootstrap.py xcash_django:/tmp/xcash_bootstrap.py
docker exec xcash_django python /tmp/xcash_bootstrap.py
```

The bootstrap is idempotent: it recreates the anvil chain record, deploys USDT/USDC mocks, funds the xcash system wallet, ensures the `BetMonster Local` project exists, and resets the EVM scan cursor if the anvil chain block height has dropped (so deposits on the fresh chain are not skipped).

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
