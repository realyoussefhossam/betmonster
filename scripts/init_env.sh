#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT"

generate_secret() {
  openssl rand -base64 48 | tr -d '=+/\n' | cut -c1-64
}

if [ -f .env ]; then
  echo ".env already exists, skipping generation."
else
  cat > .env <<EOF
# BetMonster local development environment
BETTER_AUTH_SECRET=$(generate_secret)
BETTER_AUTH_URL=http://localhost:3000

NEXT_PUBLIC_GO_API_URL=http://localhost:8080
GO_API_URL=http://localhost:8080

GATEWAY_PORT=8080
# JWKS_URL defaults to http://localhost:3000/api/auth/jwks outside Docker.
# In Docker Compose the gateway service overrides it to http://app:3000/api/auth/jwks.
# JWKS_URL=http://localhost:3000/api/auth/jwks
WALLET_SERVICE_ADDR=localhost:50051
ADMIN_USER_IDS=
# Local dev pairs: production-focused list + anvil for testing.
# The local xcash stack only activates the anvil chain and maps USDT, USDC, and
# ETH on it. Non-anvil pairs are listed here as the production reference;
# deposit calls for those pairs will fail against the local xcash until you
# configure those chains/tokens in your real xcash deployment.
# For production, copy the pair list from .env.example and remove anvil:*.
SUPPORTED_PAIRS=USDT:anvil,USDT:ethereum,USDT:bsc,USDT:polygon,USDT:tron,USDC:anvil,USDC:ethereum,USDC:bsc,USDC:polygon,USDC:base,USDC:arbitrum-one,ETH:anvil,ETH:ethereum,ETH:bsc,ETH:base,BNB:bsc,TRX:tron,POL:ethereum,POL:polygon,DAI:ethereum,SHIB:ethereum,BUSD:bsc

WALLET_PORT=8081
DATABASE_URL=postgres://wallet:wallet@localhost:5433/wallet?sslmode=disable
REDIS_ADDR=localhost:6379
NATS_URL=nats://localhost:4222

XCASH_BASE_URL=http://localhost:6688
XCASH_APPID=
XCASH_HMAC_KEY=
XCASH_WEBHOOK_SECRET=

POSTGRES_USER=wallet
POSTGRES_PASSWORD=wallet
POSTGRES_DB=wallet
EOF
  echo "Generated .env"
fi

if [ -f app/.env.local ]; then
  echo "app/.env.local already exists, skipping generation."
else
  BETTER_AUTH_SECRET=""
  if [ -f .env ]; then
    BETTER_AUTH_SECRET="$(grep '^BETTER_AUTH_SECRET=' .env | cut -d '=' -f2- || true)"
  fi
  if [ -z "$BETTER_AUTH_SECRET" ]; then
    BETTER_AUTH_SECRET="$(generate_secret)"
  fi

  cat > app/.env.local <<EOF
BETTER_AUTH_SECRET=$BETTER_AUTH_SECRET
BETTER_AUTH_URL=http://localhost:3000
NEXT_PUBLIC_GO_API_URL=http://localhost:8080
GO_API_URL=http://localhost:8080
EOF
  echo "Generated app/.env.local"
fi

echo "Done. Fill in XCASH_APPID, XCASH_HMAC_KEY, and XCASH_WEBHOOK_SECRET before running with xcash."
